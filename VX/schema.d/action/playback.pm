package action::playback;
# macro PlayFiles(File,Count,NoAnswer,fn)

use strict;
use warnings;
use Exporter;

our @ISA= qw( Exporter );

# these CAN be exported.
our @EXPORT_OK = qw( playback );

# these are exported by default.
our @EXPORT = qw( playback );

sub playback {
 my $_ = shift; # Params array reference
 my $descr = shift // '';

 my @p = @{$_};

 my $bind = '';
 $bind = "$::BIND/" if ($::BIND ne '' && uc($::BIND) ne 'NULL');

 print "[Func ". ::FN() ."] $descr\n";
 print " Macro = PlayFiles ; (File,Count,NoAnswer,fn)\n";

 my $File = $p[0] // '\'silence/1\'';
 $File = "\'$bind" . ::unquote($p[0])."\'" if defined $p[0];

 my $Count = $p[1];
 my $NoAnswer = $p[2];

 print " P1 = $File\n";
 print " P2 = $Count\n" if defined $Count;
 print " P3 = $NoAnswer\n" if defined $NoAnswer;
 print " P4 = Func(". ::FN(1) .")\n" if $::oi >= 0; # Break chain for special objects

 print "\n";
 return "hangup";
}

1;
