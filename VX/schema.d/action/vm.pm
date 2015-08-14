action::vm;
## macro VM(MailBox,MailContext,flags,fn)
# macro VMail(Mail,MaxLength,Prompt,fn)

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

 my $bind = '';
 $bind = "$::BIND/" if ($::BIND ne '' && uc($::BIND) ne 'NULL');

 print "[Func ". ::FN() ."] $descr\n";
 print " Macro = VMail ; (Mail,MaxLength,Prompt,fn)\n";

 my $MailBox = $p[0] // $::Fields{'ROOT.admin'};
 if (defined $MailBox) {
  $MailBox =~ s/\,/\%/g;
  print " P1 = \'" . ::unquote($MailBox) . "\'\n";
 } else {
  print "; Mailbox not defined!\n";
 }

 my $MaxLength = $p[1];
 my $Prompt = $p[2];

 print " P2 = " . ::unquote($MaxLength) . "\n" if defined $MaxLength;
 if (defined $Prompt) {
  if (::unquote($Prompt) ne ':') {
   print " P3 = $bind" . ::unquote($Prompt) . "\n";
  } else {
   print " P3 = " . ::unquote($Prompt) . "\n";
  }
 }
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
