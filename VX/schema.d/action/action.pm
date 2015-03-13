action::action;
use strict;
use warnings;
use Exporter;

our @ISA= qw( Exporter );

# these CAN be exported.
our @EXPORT_OK = qw( action ACTION  );

# these are exported by default.
our @EXPORT = qw( action ACTION );

sub action {
 my $_ = shift; # Params array reference
 my $descr = shift // '';

 my @p = @{$_};
 my $fn = $p[0] // '';
 $fn = $::LABELS{"ACTION.$fn"};

 print "[Func ". ::FN() ."] $descr\n";
 if (defined $fn) {
  print " Macro = Switch ; (fn,Var,Limit,FallBack)\n";
  print " P1 = Func($fn)\n";
  if (defined $::Fields{"ROOT.CallLimit"} && $::Fields{"ROOT.CallLimit"} =~ /^\d+$/) {
   print " P3 = " . $::Fields{"ROOT.CallLimit"} . "\n";
   delete $::Fields{"ROOT.CallLimit"};
  }
 } else {
  print "; Action not defined!";
  print " Macro = Hang ; Terminate call\n";
 }

 print "\n";
 return undef;
}

sub ACTION {
 return;
}

1;
