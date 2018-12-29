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

my $conn;
my @r;

$conn = DBI->connect("dbi:Pg:dbname=pbx","asterisk","",{PrintError => 0});

if ($DBI::err != 0) {
 die "ERR: Couldn't open connection: ".$DBI::errstr."\n";
}

# Retrieve fax lists with active calls present
my $sql = " SELECT *
 FROM \"MailFax\"
 WHERE EXISTS
  (
   SELECT \"NRec\" FROM \"Fax\"
    WHERE (\"MailFax\" = \"MailFax\".\"NRec\")
    AND (\"Archieved\" = 'epoch')
    AND (NOT \"Done\")
  )
  AND (\"Simultaneous\" > 0)
 ORDER BY \"NRec\"
";

my $prep = $conn->prepare($sql);
my $res = $prep->execute();
if (!defined $res) {
 die "Query failed: ".$conn->errstr."\n";
}

while (@r = $prep->fetchrow_array())
{
 fax(@r);
}
$conn->disconnect;

exit;

sub fax
{ # Look SQL TABLE "Fax"
 my $NRec = shift;
 my $EMail = shift;
 my $CallerID = shift;
 my $HeaderInfo = shift;
 my $LocalStationId = shift;
 my $WaitTime = shift;
 my $Context = shift;
 my $Mailer = shift;
 my $Simultaneous = shift;
 my $Shedule = shift;
 my $Timeout = shift;
 my $Description = shift;

 my @row;

 print " Processing fax list NRec = $NRec-$Mailer-$EMail-$CallerID-$Simultaneous-$Shedule-$Timeout-$Description\n";
 # Count of fax'es that are still processing
 my $sql = "SELECT COUNT(\"NRec\")
  FROM \"Fax\"
  WHERE (\"MailFax\" = $NRec)
   AND (\"LastStatus\" < 0)
   AND (\"Archieved\" = 'epoch')
   AND (NOT \"Done\")
   AND (\"ValidTill\" >= 'now')
   AND ((\"LastCall\" IS NULL) OR (\"LastCall\" < timestamp 'now' - interval '$Timeout minutes'))
 ";
 my $prep = $conn->prepare($sql);
 my $res = $prep->execute();
 if (!defined $res) {
  die "Query failed: ".$conn->errstr."\n";
 }
 @row = $prep->fetchrow_array();
 # Now, count maximum fax'es to be processed
 $Simultaneous = $Simultaneous - $row[0];
 if ($Simultaneous <= 0)
 {
  print "QUEUE IS FULL\n";
  return;
 }

 # Now fetch fax'es to be processed as FIFO turn
 my $sql = "SELECT
 \"NRec\", \"Exten\"
  FROM \"Fax\" AS MSG
  WHERE (\"MailFax\" = $NRec)
   AND (\"LastStatus\" >= 0)
   AND (\"Archieved\" = 'epoch')
   AND (NOT \"Done\")
   AND (\"ValidTill\" >= 'now')
   AND ((\"LastCall\" IS NULL) OR (\"LastCall\" < timestamp 'now' - interval '$Timeout minutes'))
   AND NOT EXISTS
  ( -- are some messages in other queues?
   SELECT \"NRec\" FROM \"Fax\"
    WHERE (\"MailFax\" != $NRec)
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

 my $prep = $conn->prepare($sql);
 my $res = $prep->execute();
 if (!defined $res) {
  die "Query failed: ".$conn->errstr."\n";
 }

 if ($prep->rows == 0)
 {
  print "QUEUE HAS NO CALLS TO BE PROCESSED NOW\n";
  return;
 }

 while (@row = $prep->fetchrow_array())
 {
  # Mark this call "in processing" (and move it in the turn)
  my $sql = "UPDATE \"Fax\" SET \"LastStatus\" = -1, \"LastCall\" = 'now' WHERE \"NRec\" = $row[0]";
  my $ins = $conn->prepare($sql);
  my $res = $ins->execute();
  if (!defined $res) {
   die "Query failed: ".$conn->errstr."\n";
  }
  # Now, make outgoing call from Asterisk PBX
  place_call(@row, $CallerID, $WaitTime, $Context);
 }
} # sub informer

# Place CallBacks into Asterisk dir
sub place_call
{
 my $NRec = shift;
 my $Exten = shift;
 my $CallerID = shift;
 my $WaitTime = shift;
 my $Context = shift;

 my $dir = "/var/tmp/fax";
 my $ast_dir = "/var/spool/asterisk/outgoing_done";

 print "NRec = $NRec Exten = $Exten\n";

 if (!(-e $dir))
 {
  mkdir($dir, 0700) or die "Couldn't make directory: $!n";
 }
 open(CALL, '+>', "$dir/fax-$Exten.call") or die "Couldn't open file for writing: $!n";
 chmod(0600,"$dir/fax-$Exten.call") or die "Couldn't do chmod: $!n";

 # Now, make a proper CallBack-file (look for Asterisk CallBacks)
# print CALL "NRec = $NRec Exten = $Exten\n";
 print CALL "CallerID: $CallerID\n";
 print CALL "Channel: LOCAL/$NRec\@send_fax/n\n";
 print CALL "WaitTime: $WaitTime\n";
 print CALL "Context: $Context\n";
 print CALL "Extension: $NRec\n";
 print CALL "Priority: 1\n";
 print CALL "AlwaysDelete: yes\n";
 print CALL "Archive: no\n";

 close(CALL) or die "Couldn't close file: $!n";
 rename("$dir/fax-$Exten.call","$ast_dir/fax-$Exten.call") or die "Couldn't move file: $!n";
 rmdir($dir) or die "Couldn't remove directory: $!n";
} # sub place_call

exit;
#############################################################################

sub QUERY($) # Execute pgsql query, returning $res
{
 my $query = shift;

 my $prep = $conn->prepare($query);
 my $res = $prep->execute();
 if (!defined $res) {
  die "<$query>\n failed: ".$conn->errstr."\n";
 }

 return $prep;
}

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
