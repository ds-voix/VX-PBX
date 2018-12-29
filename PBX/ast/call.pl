#!/usr/bin/perl
# -w
# Place DialOut calls into Asterisk dir

use strict;
use IO::File;     # File IO, part of core

place_call ($ARGV[0],$ARGV[1],$ARGV[2],$ARGV[3]);
exit;

sub place_call
{
 my $Src = shift;
 my $Dst = shift;
 my $BIND = shift;
 my $UUID = shift;

 my $dir = "/var/tmp/dialout";
# my $ast_dir = "/var/tmp/dialout";
 my $ast_dir = "/var/spool/asterisk/outgoing";

 if (!(-e $dir))
 {
  mkdir($dir, 0700) or die "Couldn't make directory: $!n";
 }
 open(CALL, '+>', "$dir/dialout-$Src.call") or die "Couldn't open file for writing: $!n";
 chmod(0600,"$dir/dialout-$Src.call") or die "Couldn't do chmod: $!n";

 # Now, make a proper CallBack-file (look for Asterisk CallBacks)
# print CALL "NRec = $NRec Exten = $Src\n";
 print CALL "CallerID: $Src\n";
 print CALL "Channel: LOCAL/$Dst\@dialout/n\n";
 print CALL "WaitTime: 30\n";
 print CALL "Context: dialout-dst\n";
 print CALL "Extension: $Dst\n";
 print CALL "Priority: 1\n";
 print CALL "AlwaysDelete: yes\n";
 print CALL "Archive: no\n";
 print CALL "Set: __BIND=$BIND\n" if ($BIND);
 print CALL "Set: __SRC=$Src\n";
 print CALL "Set: __DST=$Dst\n";
 print CALL "Set: __UUID=$UUID\n";

 close(CALL) or die "Couldn't close file: $!n";
 rename("$dir/dialout-$Src.call","$ast_dir/dialout-$Src.call") or die "Couldn't move file: $!n";
 rmdir($dir) or die "Couldn't remove directory: $!n";
} # sub place_call
