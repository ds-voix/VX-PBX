package action::line;
# HW template [LINE xx] dongle

use strict;
use warnings;
use Exporter;

our @ISA= qw( Exporter );

# these CAN be exported.
our @EXPORT_OK = qw( LINE );

# these are exported by default.
our @EXPORT = qw( LINE );

sub LINE {
 my $_ = shift; # Params array reference
 my $descr = shift // '';

 my @p = @{$_};

 $descr =~ s/^[\s]*;[\s]*//;
 $::Fields{"LINE.$p[0].name"} = $descr;
 return;
}

1;
