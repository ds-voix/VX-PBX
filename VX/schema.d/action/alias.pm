package action::alias;
# Switch to Func($alias)

use strict;
use warnings;
use Exporter;

our @ISA= qw( Exporter );

# these CAN be exported.
our @EXPORT_OK = qw( alias );

# these are exported by default.
our @EXPORT = qw( alias );

sub alias {
 my $_ = shift; # Params array reference
 my $descr = shift // '';

 my @p = @{$_};
 my $alias = $p[0];

 print "[Func ". ::FN() ."] $descr\n";
 if (defined $alias) {
  print " Macro = Switch ; (fn,Var,Limit,FallBack)\n";
  print " P1 = Func($alias)\n";
 } else {
  print "; Func not defined!";
  print " Macro = Hang ; Terminate call\n";
 }

 print "\n";
 return undef;
}
