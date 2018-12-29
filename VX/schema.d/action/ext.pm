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
  if (defined $::Fields{"ROOT.calllimit"} && $::Fields{"ROOT.calllimit"} =~ /^\d+$/) {
   print " P3 = " . $::Fields{"ROOT.calllimit"} . "\n";
   delete $::Fields{"ROOT.calllimit"};
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
  if (/^EXT\.$::LABEL\./p) {
   my $val = $::Fields{$_};
   $val //= '';
   my $key = ${^POSTMATCH};

   next if ($key =~/^transfer/);
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

###   print " $key = $val\n";
   print " Timeout = $val\n" if ("$key" eq 'timeout');
  }
 }

 # OnBusy & OnTimeout actions
 my @act = ();
 my @lb = ();

 my $TransferOnBusy = $::Fields{"EXT.$::LABEL.transferonbusy"};
 my $TransferOnTimeout = $::Fields{"EXT.$::LABEL.transferontimeout"};
 my $TransferCall = $::Fields{"EXT.$::LABEL.transfercall"};
 my $SpawnCalls = $::Fields{"EXT.$::LABEL.spawncalls"};
 my @k = ();
 my $val;

 if (defined $TransferOnBusy) {
  ::Keys($TransferOnBusy,\@k);
  if ($k[0] eq 'ext') {
   $val = $k[1] // '';
  } else { # Take current extension label, add mnemonic suffix
   $::LABELS{"EXT.transferonbusy"} = ::FN() . '.B'; # Rewrite!
   push(@act, $TransferOnBusy);
   push(@lb, "transferonbusy");
   $val = $::LABELS{"EXT.transferonbusy"};
  }
  print " TransferOnBusy = $val\n";
 }
 if (defined $TransferOnTimeout) {
  ::Keys($TransferOnTimeout,\@k);
  if ($k[0] eq 'ext') {
   $val = $k[1] // '';
  } else {
   $::LABELS{"EXT.transferontimeout"} = ::FN() . '.T'; # Rewrite!
   push(@act, $TransferOnTimeout);
   push(@lb, "transferontimeout");
   $val = $::LABELS{"EXT.transferontimeout"};
  }
  print " TransferOnTimeout = $val\n";
 }
 if (defined $TransferCall) {
  ::Keys($TransferCall,\@k);
  if ($k[0] eq 'ext') {
   $val = $k[1] // '';
  } else {
   $::LABELS{"EXT.transfercall"} = ::FN() . '.X'; # Rewrite!
   push(@act, $TransferCall);
   push(@lb, "transfercall");
   $val = $::LABELS{"EXT.transfercall"};
  }
  print " TransferCall = $val\n";
 }
 if (defined $SpawnCalls) { # No "SpawnCalls" object really defined at this time!!!
  ::Keys($SpawnCalls,\@k);
  if ($k[0] eq 'ext') {
   $val = $k[1] // '';
  } else {
   $::LABELS{"EXT.spawncalls"} = ::FN() . '.X'; # Rewrite!
   push(@act, $SpawnCalls);
   push(@lb, "spawncalls");
   $val = $::LABELS{"EXT.spawncalls"};
  }
  print " SpawnCalls = $val\n";
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
