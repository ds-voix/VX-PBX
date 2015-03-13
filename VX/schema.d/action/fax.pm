action::fax;
# macro fax(mailbox, id, header,tiff)

use strict;
use warnings;
use Exporter;

our @ISA= qw( Exporter );

# these CAN be exported.
our @EXPORT_OK = qw( fax );

# these are exported by default.
our @EXPORT = qw( fax );

sub fax {
 my $_ = shift; # Params array reference
 my $descr = shift // '';

 my @p = @{$_};

 print "[Func ". ::FN() ."] $descr\n";
 print " Macro = fax ; (mailbox,id,header,tiff)\n";

 my $mailbox = $p[0] // $::Fields{'ROOT.Admin'};
 if (defined $mailbox) {
  print " P1 = \'$mailbox\'\n";
 } else {
  print "; Mailbox not defined!\n";
 }

 my $id = $p[1];
 my $header = $p[2];
 my $tiff = $p[3];

 print " P2 = $id\n" if defined $id;
 print " P3 = $header\n" if defined $header;
 print " P4 = $tiff\n" if defined $tiff;

 print "\n";
 return undef;
}

1;
