package action::fnc;
# Switch to Func($fnc)

use strict;
use warnings;
use Exporter;

our @ISA= qw( Exporter );

# these CAN be exported.
our @EXPORT_OK = qw( fnc );

# these are exported by default.
our @EXPORT = qw( fnc );

sub fnc {
 my $_ = shift; # Params array reference
 my $descr = shift // '';

 my @p = @{$_};
 my $fnc = $p[0];

 print "[Func ". ::FN() ."] $descr\n";
 if (defined $fnc) {
  print " Macro = Switch ; (fn,Var,Limit,FallBack)\n";
  print " P1 = Func($fnc)\n";
  if (defined $::Fields{"ROOT.CallLimit"} && $::Fields{"ROOT.CallLimit"} =~ /^\d+$/) {
   print " P3 = " . $::Fields{"ROOT.CallLimit"} . "\n";
   delete $::Fields{"ROOT.CallLimit"};
  }
 } else {
  print "; Func not defined!";
  print " Macro = Hang ; Terminate call\n";
 }

 print "\n";
 return undef;
}
