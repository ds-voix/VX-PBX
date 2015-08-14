action::menu;
# macro Menu(NRec,NoHang)

use strict;
use warnings;
use Exporter;

our @ISA= qw( Exporter );

# these CAN be exported.
our @EXPORT_OK = qw( menu MENU );

# these are exported by default.
our @EXPORT = qw( menu MENU );

sub menu {
 my $_ = shift; # Params array reference
 my $descr = shift // '';

 my @p = @{$_};

 print "[Func ". ::FN() ."] $descr\n";
 print " Macro = Menu ; (NRec,NoHang,Limit)\n";

 print " P1 = \'" . ::FL('MENU',join(' ',@p),1) . "\'\n";
 if (defined $::Fields{"ROOT.calllimit"} && $::Fields{"ROOT.calllimit"} =~ /^\d+$/) {
  print " P3 = " . $::Fields{"ROOT.calllimit"} . "\n";
  delete $::Fields{"ROOT.calllimit"};
 }

 print "\n";
 return undef;
}

sub MENU { # Declare menu
 my $_ = shift; # Params array reference
 my $descr = shift // '';

 my @p = @{$_};
 my %ACT = (); # Menu on-key actions
 my @act = ();
 my @lb = ();
 my ($key, $val);

 print "[Menu " . ::FL('MENU',join(' ',@p),1) ."]$descr\n";

 for (sort keys %::Fields) {
  if (/^MENU\.$::LABEL\./p) {
   $val = ::unquote($::Fields{$_}) // '';
   $key = ${^POSTMATCH};

   if ($key =~/^[(]?[\s'"]*([0-9*#]|timeoutaction)[\s'",]*/ && $val !~ '^BACK:?') {
    my $l = $1;
    my @k = ();
    &Keys($::Fields{$_},\@k);

    if ($k[0] eq 'ext') {
     $val = "Exten($k[1])";
    } else {
     $::LABELS{"MENU.$key"} = ::FN('MENU',$::LABEL) . '.' . substr($1,0,1); # Rewrite!
     push(@act, $val);
     push(@lb, $key);
     $val = "Func(" . $::LABELS{"MENU.$key"} . ")";
    }
    $key = '"' . $key . '"' unless ($key eq 'timeoutaction') || $key =~ /^\(/;
    ${key} = 'TimeoutAction' if ($key eq 'timeoutaction');
    $ACT{$key} = $val;
   } else {
    if ($key =~ /hello|prompt/) {
     $val = "$::BIND/$val" if ($val ne '');
    }
    if ($key eq 'parent') {
     $val = ::FL('MENU',$val,1);
    }
    $key = uc(substr($key,0,1)) . substr($key,1);
    print "\"$key\" = '$val'\n";
   }
  }
 }

 for (sort keys %ACT) {
  print " $_ = $ACT{$_}\n";
 }

 print "\n";

 my $i = 0;
 for (@act) {
  $::LABEL = $lb[$i++]; # Still need the label hack :/
  $::oi = -1;

  Action($_);
 }
 return;
}

1;
