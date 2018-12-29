package action::hooks;

use strict;
use warnings;
use Exporter;

our @ISA= qw( Exporter );

# these CAN be exported.
our @EXPORT_OK = qw( HOOKS );

# these are exported by default.
our @EXPORT = qw( HOOKS );

sub HOOKS {
 $_ = shift; # Params array reference
 my $descr = shift // '';

 my @p = @{$_};

 $descr =~ s/^[\s]*;[\s]*//;
 my ($key, $val);
 my $hooks = "DISK=(\n";

 for (sort keys %::Fields) {
  if (/^HOOKS\./p) {
   next if (/^HOOKS\..+\.file$/p);
   $val = ::trim($::Fields{$_}) // '';
   $key = ${^POSTMATCH};
   $hooks .= "[$key]=\"$val\"\n";
  }
 }

 $hooks .= ")\n";

 my $fh;
 my $File = $::Fields{"HOOKS.$p[0].file"} // '';

 open($fh, '>', $File) or die "Can't open file \"$File\"";
 {
  local $/;
  print $fh "$hooks";
 }
 close($fh);

 print "Hooks saved to file \"$File\"\n";

 return;
}

1;
