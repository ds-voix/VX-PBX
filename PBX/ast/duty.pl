#!/usr/bin/perl
# -w
use strict;

use IO::File;     # File IO, part of core
use Getopt::Std;  # command line option processing
use Fcntl qw(:DEFAULT :flock);  # for file locking
use POSIX qw(strftime);         # pretty date formatting
use Time::Local;                # to do reverse of gmtime()
use DBI;
use File::Copy;
use Sys::Syslog qw(:standard :macros);

my @CONNSTR = ("dbi:Pg:dbname=pbx","asterisk","",{PrintError => 0});
our $conn;
my $prep;
my $res;
my $sql;
my @r;

$SIG{'TERM'} = 'TERM_handler';
sub TERM_handler {
 openlog('astDuty:', "perror,nofatal,pid", "local0");
 syslog(LOG_CRIT, "Interrupted by SIG_TERM!");
 closelog();
 exit;
}

openlog('astDuty:', "perror,nofatal,pid", "local0");
syslog(LOG_CRIT, "Asterisk duty handler started");

while (1)
{
 openlog('astDuty:', "perror,nofatal,pid", "local0");
 setlogmask( LOG_MASK(LOG_CRIT) | LOG_MASK(LOG_ERR) | LOG_MASK(LOG_WARNING) | LOG_MASK(LOG_NOTICE) ); # | LOG_MASK(LOG_INFO)

# $conn = undef;
 $conn = DBI->connect(@CONNSTR);

 if ($DBI::err) {
  syslog(LOG_ERR, "ERR: Couldn't open connection: %s", $DBI::errstr);
  sleep(60);
  next;
 }

 # Retrieve messaging lists with active calls present
 $sql = " select \"NRec\", \"Exten\",\"BIND\",\"Amount\",\"Shift\",\"Timeout\",\"Penalty\",\"Valid\",\"Announce\",\"Description\"
  from \"Duty\"
  where (select count(*) from \"DutyAgent\" where (\"DutyAgent\".\"Duty\" = \"Duty\".\"NRec\") and (\"Active\" > (now() - \"Elect\"))) < \"Amount\"
   and ((\"Schedule\" is NULL) or \"InSchedule\"(\"Schedule\"));
 ";

 $prep = $conn->prepare($sql);
 $res = $prep->execute();

 unless (defined $res) {
  syslog(LOG_ERR, "ERR: Query failed: %s", $conn->errstr);
  sleep(60);
  next;
 }

 while (@r = $prep->fetchrow_array())
 {
#  print "@r\n";
  duty(@r);
 }

 undef $prep;
 $conn->disconnect;
# closelog(); CRASH!!!!
 sleep(60);
} # while true

openlog('astDuty:', "perror,nofatal,pid", "local0");
syslog(LOG_ERR, "Unexpected  exit!!!");
closelog();
exit;
#############################################################################

sub duty
{ # Look SQL TABLE "DutyAgent"
 my $NRec = shift;
 my $Exten = shift;
 my $BIND = shift;
 my $Amount = shift;
 my $Shift = shift;
 $Shift --;
 my $Timeout = shift;
 my $Penalty = shift;
 my $Valid = shift;
 my $Announce = shift;
 my $Description = shift;

 my @row;

 # Validate agents. Get last active first
 $sql = "select \"NRec\",\"Agent\",\"Description\",\"Active\"
  from  \"DutyAgent\"
  where (\"Duty\" = $NRec)
   and (\"Fail\" < (now() - interval '$Penalty'))
  order by \"Active\" ASC, \"Called\" DESC
  LIMIT 1 OFFSET $Shift;
 ";

 my $prep = $conn->prepare($sql);
 my $res = $prep->execute();
 if (!defined $res) {
  syslog(LOG_ERR, "ERR: Query failed: %s", $conn->errstr);
  sleep(60);
  return;
 }

 if ($prep->rows == 0)
 {
  syslog(LOG_INFO, "LIST: NO AGENTS TO BE VALIDATED NOW");
  return;
 } else
 {
  @row = $prep->fetchrow_array();
  syslog(LOG_NOTICE, "AGENT #%s, Active=%s", ($row[1], $row[3]));
  my $sql = "UPDATE \"DutyAgent\" SET \"Called\" = 'now' WHERE \"NRec\" = $row[0]";
  my $ins = $conn->prepare($sql);
  my $res = $ins->execute();
  if (!defined $res) {
   syslog(LOG_ERR, "ERR: Query failed: %s", $conn->errstr);
   sleep(60);
   return;
  }
  # Now, make outgoing call from Asterisk PBX
  place_call(@row[0], $Exten, $BIND, $Timeout);
 }

# undef $prep;
# $conn->disconnect;
} # sub duty

# Place CallBacks into Asterisk dir
sub place_call
{
 my $NRec = shift;
 my $Exten = shift;
 my $BIND = shift;
 my $WaitTime = shift;

 my $dir = "/var/tmp/duty";
 my $ast_dir = "/var/spool/asterisk/outgoing";

 syslog(LOG_NOTICE, "CALL: NRec=%s, Exten=%s", ($NRec, $Exten));

 if (!(-e $dir))
 {
  unless (mkdir($dir, 0700))
  {
   syslog(LOG_ERR, "ERR: Couldn't make directory: %s", $!);
   sleep(60);
   return;
  }
 }
 unless (open(CALL, '+>', "$dir/duty-$Exten.call"))
 {
  syslog(LOG_ERR, "ERR: Couldn't open file for writing: %s", $!);
  sleep(60);
  return;
 }
 unless (chmod(0600,"$dir/duty-$Exten.call"))
 {
  syslog(LOG_ERR, "ERR: Couldn't do chmod: %s", $!);
  sleep(60);
  return;
 }

 # Now, make a proper CallBack-file (look for Asterisk CallBacks)
# print CALL "NRec = $NRec Exten = $Exten\n";
 print CALL "CallerID: DUTY <$Exten>\n";
 print CALL "Channel: LOCAL/$NRec\@duty/n\n";
 print CALL "WaitTime: $WaitTime\n";
 print CALL "Context: duty_agent\n";
 print CALL "Extension: $NRec\n";
 print CALL "Priority: 1\n";
 print CALL "AlwaysDelete: yes\n";
 print CALL "Archive: no\n";
 print CALL "Set: __BIND=$BIND\n";

 close(CALL) or die "Couldn't close file: $!";
 unless (rename("$dir/duty-$Exten.call","$ast_dir/duty-$Exten.call"))
 {
  syslog(LOG_ERR, "ERR: Couldn't move file: %s", $!);
  sleep(60);
  return;
 }
 unless (rmdir($dir))
 {
  syslog(LOG_ERR, "ERR: Couldn't remove directory: %s", $!);
  sleep(60);
  return;
 }
} # sub place_call

#############################################################################

#sub QUERY($) # Execute pgsql query, returning $res
#{
# my $query = shift;
#
# my $prep = $conn->prepare($query);
# my $res = $prep->execute();
# if (!defined $res) {
#  syslog(LOG_ERR, "ERR: Query <%s> failed: %s", ($query, $conn->errstr));
#  sleep(60);
#  return;
# }
#
# return $prep;
#}

sub trim($)
{
	my $string = shift;
	$string =~ s/^\s+//;
	$string =~ s/\s+$//;
	return $string;
}
# Left trim function to remove leading whitespace
sub ltrim($)
{
	my $string = shift;
	$string =~ s/^\s+//;
	return $string;
}
# Right trim function to remove trailing whitespace
sub rtrim($)
{
	my $string = shift;
	$string =~ s/\s+$//;
	return $string;
}

sub ulc($) # UTF-8 to lower_case
{
 my $string = shift;
 return encode_utf8(lc(decode_utf8($string)));
}

sub uuc($) # UTF-8 to UPPER_case
{
 my $string = shift;
 return encode_utf8(uc(decode_utf8($string)));
}
