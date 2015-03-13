package action::hangup;
# macro Hang()

use strict;
use warnings;
use Exporter;

our @ISA= qw( Exporter );

# these CAN be exported.
our @EXPORT_OK = qw( hangup );

# these are exported by default.
our @EXPORT = qw( hangup );

sub hangup {
 my $_ = shift; # Params array reference
 my $descr = shift // '';

 my @p = @{$_};

 print "[Func ". ::FN() ."] $descr\n";
 print " Macro = Hang ; Terminate call\n";

 print "\n";
 return undef;
}

1;
