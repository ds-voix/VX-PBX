action::vm;
# macro VM(MailBox,MailContext,flags,fn)

use strict;
use warnings;
use Exporter;

our @ISA= qw( Exporter );

# these CAN be exported.
our @EXPORT_OK = qw( vm );

# these are exported by default.
our @EXPORT = qw( vm );

sub vm {
 my $_ = shift; # Params array reference
 my $descr = shift // '';

 my @p = @{$_};

 print "[Func ". ::FN() ."] $descr\n";
 print " Macro = VM ; (MailBox,MailContext,flags,fn)\n";

 my $MailBox = $p[0] // $::Fields{'ROOT.Admin'};
 if (defined $MailBox) {
  print " P1 = \'$MailBox\'\n";
 } else {
  print "; Mailbox not defined!\n";
 }

 my $MailContext = $p[1];
 my $flags = $p[2];

 print " P2 = $MailContext\n" if defined $MailContext;
 print " P3 = $flags\n" if defined $flags;
 print " P4 = Func(". ::FN(1) .")\n" if $::oi >= 0; # Break chain for special objects

 print "\n";
 return 'hangup';
}

#[voicemail ip#gdberg.ru] ; VoiceMail for Gardenberg
# email = ip@gdberg.ru
# serveremail = Gardenberg.PBX
# delete = yes ; Delete record after message sent
# password = Amm6PqZBmDrd
# fullname = "Mailbox $$"
#
#[Func VM] ; VoiceMail for Gardenberg
# Macro = VM ; macro VM(MailBox,MailContext,flags,fn)
# P1 = 'ip#gdberg.ru'
# P4 = Hang(16)

sub VM {
 return;
}
1;
