package action::ext;
# TABLE "Exten"

use strict;
use warnings;
use Exporter;

our @ISA= qw( Exporter );

# these CAN be exported.
our @EXPORT_OK = qw( ext EXT );

# these are exported by default.
our @EXPORT = qw( ext EXT );

sub ext {
 my $_ = shift; # Params array reference
 my $descr = shift // '';

 my @p = @{$_};
 my $ext = $p[0];

 print "[Func ". ::FN() ."] $descr\n";
 if (defined $ext) {
  print " Macro = Switch ; (fn,Var,Limit,FallBack)\n";
  print " P1 = Exten($ext)\n";
  if (defined $::Fields{"ROOT.CallLimit"} && $::Fields{"ROOT.CallLimit"} =~ /^\d+$/) {
   print " P3 = " . $::Fields{"ROOT.CallLimit"} . "\n";
   delete $::Fields{"ROOT.CallLimit"};
  }
 } else {
  print "; Exten not defined!";
  print " Macro = Hang ; Terminate call\n";
 }

 print "\n";
 return undef;
}

sub EXT { # Declare extension
 my $_ = shift; # Params array reference
 my $descr = shift // '';

 my @p = @{$_};
 my $ext = $p[0];

# my @act = ();
# my $i = 0;

 print "[Exten $ext]$descr\n";

 for (sort keys %::Fields) {
  if (/^EXT\./p) {
   my $val = $::Fields{$_};
   $val //= '';
   my $key = ${^POSTMATCH};

   next if ($key =~/^Transfer/);
#   if ($key =~/^Transfer/) {
#    my @k = ();
#    ::Keys($::Fields{$_},\@k);
#
#    if ($k[0] eq 'ext') {
#     $val = $k[1];
#    } else {
#     push(@act, $val);
#
#     $val = ::FN($i);
#     $i++;
#    }
#   }
   print " $key = $val\n";
  }
 }

 # OnBusy & OnTimeout actions
 my @act = ();
 my @lb = ();

 my $TransferOnBusy = $::Fields{'EXT.TransferOnBusy'};
 my $TransferOnTimeout = $::Fields{'EXT.TransferOnTimeout'};
 my $TransferCall = $::Fields{'EXT.TransferCall'};
 my @k = ();
 my $val;

 if (defined $TransferOnBusy) {
  ::Keys($TransferOnBusy,\@k);
  if ($k[0] eq 'ext') {
   $val = $k[1];
  } else { # Take current extension label, add mnemonic suffix
   $::LABELS{"EXT.TransferOnBusy"} = ::FN() . '.B'; # Rewrite!
   push(@act, $TransferOnBusy);
   push(@lb, "TransferOnBusy");
   $val = $::LABELS{"EXT.TransferOnBusy"};
  }
  print " TransferOnBusy = $val\n";
 }
 if (defined $TransferOnTimeout) {
  ::Keys($TransferOnTimeout,\@k);
  if ($k[0] eq 'ext') {
   $val = $k[1];
  } else {
   $::LABELS{"EXT.TransferOnTimeout"} = ::FN() . '.T'; # Rewrite!
   push(@act, $TransferOnTimeout);
   push(@lb, "TransferOnTimeout");
   $val = $::LABELS{"EXT.TransferOnTimeout"};
  }
  print " TransferOnTimeout = $val\n";
 }
 if (defined $TransferCall) {
  ::Keys($TransferCall,\@k);
  if ($k[0] eq 'ext') {
   $val = $k[1];
  } else {
   $::LABELS{"EXT.TransferCall"} = ::FN() . '.X'; # Rewrite!
   push(@act, $TransferCall);
   push(@lb, "TransferCall");
   $val = $::LABELS{"EXT.TransferCall"};
  }
  print " TransferCall = $val\n";
 }

 print "\n";

 my $i = 0;
 for (@act) {
  $::LABEL = $lb[$i++]; # Still need the label hack :/
  $::oi = -1;
  ::Action($_);
 }

 return;
}

1;
