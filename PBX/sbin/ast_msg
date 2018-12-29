#!/usr/bin/perl
# -w

# Message queues processing for Asterisk
# Copyright (C) 2014 Dmitry Svyatogorov ds@vo-ix.ru

#    This program is free software: you can redistribute it and/or modify
#    it under the terms of the GNU Affero General Public License as
#    published by the Free Software Foundation, either version 3 of the
#    License, or (at your option) any later version.
#
#    This program is distributed in the hope that it will be useful,
#    but WITHOUT ANY WARRANTY; without even the implied warranty of
#    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#    GNU Affero General Public License for more details.
#
#    You should have received a copy of the GNU Affero General Public License
#    along with this program.  If not, see <http://www.gnu.org/licenses/>.

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
 openlog('astMsg:', "perror,nofatal,pid", "local0");
 syslog(LOG_CRIT, "Interrupted by SIG_TERM!");
 closelog();
}

openlog('astMsg:', "perror,nofatal,pid", "local0");
syslog(LOG_CRIT, "Asterisk voice|fax message handler started");

while (1)
{
 openlog('astMsg:', "perror,nofatal,pid", "local0");
 setlogmask( LOG_MASK(LOG_CRIT) | LOG_MASK(LOG_ERR) | LOG_MASK(LOG_WARNING) | LOG_MASK(LOG_NOTICE) ); # | LOG_MASK(LOG_INFO)

# $conn = undef;
 $conn = DBI->connect(@CONNSTR);

 if ($DBI::err) {
  syslog(LOG_ERR, "ERR: Couldn't open connection: %s", $DBI::errstr);
  sleep(60);
  next;
 }

 # Retrieve messaging lists with active calls present
 $sql = " SELECT *
  FROM \"InformerMessages\"

  LEFT OUTER JOIN \"Schedules\" ON (\"InformerMessages\".\"Schedule\" = \"Schedules\".\"Schedule\")
             AND (\"IFTIME\"(\"Schedules\".\"TimeRange\"||'|'||\"Schedules\".\"DaysOfWeek\"||'|'||\"Schedules\".\"DaysOfMonth\"||'|'||\"Schedules\".\"Month\"||'|'||\"Schedules\".\"Year\"))

  WHERE EXISTS
   (
    SELECT \"NRec\" FROM \"Informer\"
     WHERE (\"InformerMessages\" = \"InformerMessages\".\"NRec\")
     AND (\"Archieved\" = 'epoch')
     AND (NOT \"Done\")
   )
   AND (\"Simultaneous\" > 0)
   AND NOT ((\"Voice\" IS NULL) AND (\"FAX\" IS NULL))
   AND ((\"InformerMessages\".\"Schedule\" is NULL) or NOT (\"Schedules\".\"NRec\" is NULL))
  ORDER BY \"InformerMessages\".\"NRec\"
 ";

 print "$sql\n";

 $prep = $conn->prepare($sql);
 $res = $prep->execute();

 unless (defined $res) {
  syslog(LOG_ERR, "ERR: Query failed: %s", $conn->errstr);
  sleep(60);
  next;
 }

 while (@r = $prep->fetchrow_array())
 {
  informer(@r);
 }

 undef $prep;
 $conn->disconnect;
# closelog(); CRASH!!!!
 sleep(60);
} # while true

openlog('astMsg:', "perror,nofatal,pid", "local0");
syslog(LOG_ERR, "Unexpected  exit!!!");
closelog();
exit;
#############################################################################

sub informer
{ # Look SQL TABLE "InformerMessages"
 my $NRec = shift;
 my $CallerID = shift;
 my $WaitTime = shift;
 my $Context = shift;
 my $Mailer = shift;
 my $Voice = shift;
 my $Annoy = shift;
 my $FAX = shift;
 my $Simultaneous = shift;
 my $Shedule = shift;
 my $Timeout = shift;
 my $Description = shift;

 my @row;

# print " Processing message list NRec = $NRec-$Mailer-$Voice-$Annoy-$FAX-$Simultaneous-$Shedule-$Timeout-$Description\n";
 # Count of messages that are still processing
 my $sql = "SELECT COUNT(\"NRec\")
  FROM \"Informer\"
  WHERE (\"InformerMessages\" = $NRec)
   AND (\"LastStatus\" < 0)
   AND (\"Archieved\" = 'epoch')
   AND (NOT \"Done\")
   AND (\"ValidTill\" >= 'now')
   AND ((\"LastCall\" IS NULL) OR (\"LastCall\" < timestamp 'now' - interval '$Timeout minutes'))
 ";
 my $prep = $conn->prepare($sql);
 my $res = $prep->execute();
 if (!defined $res) {
  syslog(LOG_ERR, "ERR: Query failed: %s", $conn->errstr);
  sleep(60);
  return;
 }
 @row = $prep->fetchrow_array();
 # Now, count maximum messages to be processed
 $Simultaneous = $Simultaneous - $row[0];
 if ($Simultaneous <= 0)
 {
  syslog(LOG_NOTICE, "NOTE: QUEUE IS FULL");
  return;
 }

 # Now fetch messages to be processed as FIFO turn
 $sql = "SELECT
 \"NRec\", \"Exten\"
  FROM \"Informer\" AS MSG
  WHERE (\"InformerMessages\" = $NRec)
   AND (\"LastStatus\" >= 0)
   AND (\"Archieved\" = 'epoch')
   AND (NOT \"Done\")
   AND (\"ValidTill\" >= 'now')
   AND ((\"LastCall\" IS NULL) OR (\"LastCall\" < timestamp 'now' - interval '$Timeout minutes') OR (\"LastStatus\" = 2)) -- busy as well
   AND NOT EXISTS
  ( -- are some messages in other queues?
   SELECT \"NRec\" FROM \"Informer\"
    WHERE (\"InformerMessages\" != $NRec)
    AND (\"Exten\" = MSG.\"Exten\")
    AND (\"LastStatus\" < 0)
    AND (\"Archieved\" = 'epoch')
    AND (NOT \"Done\")
    AND (\"ValidTill\" >= 'now')
--    AND ((\"LastCall\" = 'epoch') OR (\"LastCall\" >= timestamp 'now' - interval '$Timeout minutes'))
  )
  ORDER BY \"LastCall\", \"PlacedAt\"
  LIMIT $Simultaneous
 ";
# print "$sql\n";

 $prep = $conn->prepare($sql);
 $res = $prep->execute();
 if (!defined $res) {
  syslog(LOG_ERR, "ERR: Query failed: %s", $conn->errstr);
  sleep(60);
  return;
 }

 if ($prep->rows == 0)
 {
  syslog(LOG_INFO, "LIST: NO CALLS TO BE PROCESSED NOW");
  return;
 } else
 {
  syslog(LOG_NOTICE, "LIST: NRec=%s, Descr=%s, Calls=%s", ($NRec, $Description, $prep->rows));
 }

 while (@row = $prep->fetchrow_array())
 {
  # Mark this call "in processing" (and move it in the turn)
  my $sql = "UPDATE \"Informer\" SET \"LastStatus\" = -1, \"LastCall\" = 'now' WHERE \"NRec\" = $row[0]";
  my $ins = $conn->prepare($sql);
  my $res = $ins->execute();
  if (!defined $res) {
   syslog(LOG_ERR, "ERR: Query failed: %s", $conn->errstr);
   sleep(60);
   return;
  }
  # Now, make outgoing call from Asterisk PBX
  place_call(@row, $CallerID, $WaitTime, $Context);
 }
 undef $prep;
# $conn->disconnect;
} # sub informer

# Place CallBacks into Asterisk dir
sub place_call
{
 my $NRec = shift;
 my $Exten = shift;
 my $CallerID = shift;
 my $WaitTime = shift;
 my $Context = shift;

 my $dir = "/var/tmp/informer";
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
 unless (open(CALL, '+>', "$dir/informer-$Exten.call"))
 {
  syslog(LOG_ERR, "ERR: Couldn't open file for writing: %s", $!);
  sleep(60);
  return;
 }
 unless (chmod(0600,"$dir/informer-$Exten.call"))
 {
  syslog(LOG_ERR, "ERR: Couldn't do chmod: %s", $!);
  sleep(60);
  return;
 }

 # Now, make a proper CallBack-file (look for Asterisk CallBacks)
# print CALL "NRec = $NRec Exten = $Exten\n";
 print CALL "CallerID: $CallerID\n";
 print CALL "Channel: LOCAL/$NRec\@informer/n\n";
 print CALL "WaitTime: $WaitTime\n";
 print CALL "Context: $Context\n";
 print CALL "Extension: $NRec\n";
 print CALL "Priority: 1\n";
 print CALL "AlwaysDelete: yes\n";
 print CALL "Archive: no\n";

 close(CALL) or die "Couldn't close file: $!";
 unless (rename("$dir/informer-$Exten.call","$ast_dir/informer-$Exten.call"))
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
