#!/usr/bin/perl
# -w

# Auto-redial behaviour for asterisk
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

#my $conn = Pg::connectdb("dbname=pbx");
# user=postgres
my $conn;
my @r;

while (1==1)
{
 $conn = DBI->connect("dbi:Pg:dbname=pbx","asterisk","",{PrintError => 0});

 if ($DBI::err != 0) {
  print "ERR: Couldn't open connection: ".$DBI::errstr."\n";
  sleep(15);
  next;
 }

# Retrieve calls to try now
 my $sql = " SELECT *
  FROM \"AutoDial\"
  WHERE \"PlacedAt\" between (timestamp 'now' - interval '15 minutes') and (timestamp 'now' - interval '15 seconds')
 ";

 my $prep = $conn->prepare($sql);
 my $res = $prep->execute();
 if (!defined $res) {
  print "Query failed: ".$conn->errstr."\n";
  sleep(15);
  next;
 }

 while (@r = $prep->fetchrow_array())
 {
 # my @row = $res->fetchrow;
 # print "$i\t\"$row[0]\"\t\"$row[1]\"\t\"$row[2]\"\n";
  place_call(@r);
 }
 $conn->disconnect;
 sleep(15);
}
exit;
##########################################################################

# Place Calls into Asterisk dir
sub place_call
{
 my $Src = shift;
 my $Dst = shift;
 my $PlacedAt = shift;
 my $LastCall = shift;
 my $NoAnswer = shift;
 my $Busy = shift;

 my $dir = "/var/tmp/autodial";
 my $ast_dir = "/var/spool/asterisk/outgoing";

 if (!(-e $dir))
 {
  mkdir($dir, 0700) or die "Couldn't make directory: $!n";
 }
 open(CALL, '+>', "$dir/autodial-$Src.call") or die "Couldn't open file for writing: $!n";
 chmod(0600,"$dir/autodial-$Src.call") or die "Couldn't do chmod: $!n";

 # Now, make a proper CallBack-file (look for Asterisk CallBacks)
# print CALL "NRec = $NRec Exten = $Src\n";
 print CALL "CallerID: $Src\n";
 print CALL "Channel: LOCAL/$Dst\@autodial/n\n";
 print CALL "WaitTime: 30\n";
 print CALL "Context: autodial-dst\n";
 print CALL "Extension: $Dst\n";
 print CALL "Priority: 1\n";
 print CALL "AlwaysDelete: yes\n";
 print CALL "Archive: no\n";

 close(CALL) or die "Couldn't close file: $!n";
 rename("$dir/autodial-$Src.call","$ast_dir/autodial-$Src.call") or die "Couldn't move file: $!n";
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
