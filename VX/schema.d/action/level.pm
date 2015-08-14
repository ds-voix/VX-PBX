package action::level;
# macro CallLevelI(F_gt,F_le,level)

use strict;
use warnings;
use Exporter;

our @ISA= qw( Exporter );

# these CAN be exported.
our @EXPORT_OK = qw( level LEVEL );

# these are exported by default.
our @EXPORT = qw( level LEVEL );

sub level {
 my $_ = shift; # Params array reference
 my $descr = shift // '';
 my $inline = shift // 0;

 my @p = @{$_};

 print "[Func ". ::FN() ."] $descr\n";
 print " Macro = CallLevelI ; (F_gt,F_le,level)\n";

 my $fn = $p[0] // '';
 ::ON("LEVEL","$fn");
 $fn = $::LABELS{"LEVEL.$fn"};
 if (defined $fn) {
  $fn = "Func($fn)";
 } else {
  $fn = 'Hang() ; No action for CallLevelI!';
 }

 print " P1 = $fn\n";
 print " P2 = Func(". ::FN(1) .")\n" if $::oi >= 0; # Break chain for special objects
 print " P3 = \'$p[1]\'\n" if $p[1];

 print "\n";
 return 'hangup';
}

sub LEVEL {
 return;
}

1;
