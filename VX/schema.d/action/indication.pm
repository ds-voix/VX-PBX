package action::indication;
# macro Indication(fn,Timeout,NoTransfer,Limit)

use strict;
use warnings;
use Exporter;

our @ISA= qw( Exporter );

# these CAN be exported.
our @EXPORT_OK = qw( indication );

# these are exported by default.
our @EXPORT = qw( indication );

sub indication {
 my $_ = shift; # Params array reference
 my $descr = shift // '';

 my @p = @{$_};

 print "[Func ". ::FN() ."] $descr\n";
 print " Macro = Indication ; (fn,Timeout,NoTransfer,Limit)\n";

 my $Timeout = $p[0];
 my $Limit = $p[1];
 my $NoTransfer = $p[2];

 if ($::oi >= 0) {
  print " P1 = Func(". ::FN(1) .")\n";
 } else {
  print " P1 = Hang()\n"; # Break chain for special objects
 }

 print " P2 = $Timeout\n" if defined $Timeout;
 print " P3 = $Limit\n" if defined $Limit;
 print " P4 = $NoTransfer\n" if defined $NoTransfer;

 print "\n";
 return "hangup";
}

1;
