package action::queue;
# macro queue(name,play,noanswer,extra)

use strict;
use warnings;
use Exporter;

our @ISA= qw( Exporter );

# these CAN be exported.
our @EXPORT_OK = qw( queue QUEUE );

# these are exported by default.
our @EXPORT = qw( queue QUEUE );

sub queue {
 my $_ = shift; # Params array reference
 my $descr = shift // '';
 my $inline = shift;

 my $play;
 my $noanswer;
 my $extra = 'r';

 my @p = @{$_};
 my $act = join(' ', @p);
 my $label = "QUEUE.$p[0]";

 my $bind = '';
 $bind = "$::BIND/" if ($::BIND ne '' && uc($::BIND) ne 'NULL');

 if (defined $::Fields{"$label.Hello"}) {
  $play = "\'$bind" . ::unquote($::Fields{"$label.Hello"})."\'";
 } else {
  if (defined $::Fields{"ROOT.Hello"}) {
   $play = "\'$bind" . ::unquote($::Fields{"ROOT.Hello"})."\'";
   delete $::Fields{"ROOT.Hello"};
  }
 }

 my $wait = $::Fields{"ROOT.Wait"} // '';

 if ($inline) { # Make object
#  for (keys %::Fields) {
#    delete $::Fields{$_} if (/^QUEUE\./);
#  }
  $act = join(' ', @p);
#  ::ON('QUEUE',$act);
  my $fn = $::OI{'QUEUE'};
  $fn = '' unless $fn;
  $::LABELS{"QUEUE.$act"} //= "$::did.Q$fn";

  if ($p[0] !~ /^([A-Za-z_.-]+|=)$/) {
   my @d = ();
   ::CSV($p[1], \@d); # Spec
   $extra = undef if (defined $d[1] && $d[1] !~ /^r(ing)?$/i) || (defined $d[0] &&  !defined $d[1] && $d[0] =~ /^m/i);
   $::Fields{"$label.MOH"} //= $::Fields{"ROOT.MOH"} // $d[1];
  } else {
   if ($p[0] eq '=' && $p[1] =~ /^(dial|queue)$/i) {
    my @d = ();
    ::CSV($p[3], \@d); # Spec
    $extra = undef if defined $d[1] && $d[1] !~ /^r(ing)?$/i;
    $::Fields{"$label.MOH"} = $d[1];
    $::Fields{"$label.MOH"} //= $::Fields{"ROOT.MOH"} // $d[1];
   }
  }

  if ($p[0] =~ /^(ext|action)$/i) {
   if (defined $p[2]) {
    $extra = undef if defined $p[2] && $p[2] !~ /^r(ing)?$/i;
    $::Fields{"$label.MOH"} //= $::Fields{"ROOT.MOH"} // $p[2];
   }
   $play = "$bind$p[3]" if defined $p[3];
  }
 } else {
  $act = join(' ', @p);
  $wait = $p[3] if defined $p[3];
  $extra = undef if defined $p[1] && $p[1] !~ /^r(ing)?$/i;
  $play = "$bind$p[2]" if defined $p[2] && $p[2] =~ /[\d\w]/;
 }

 $wait =~ s/\D//g;
 if ($wait) {
  $extra //= '';
  $extra .= ",,,$wait";
  $extra .= ":Func(". ::FN(1) .")" if $::oi >= 0; # Break chain for special objects
 }

 print "[Func ". ::FN() ."] $descr\n";

 if (defined $::Fields{"ROOT.Indication"}) {
  print " Macro = Indication ; (fn,Timeout,NoTransfer,Limit)\n";
  $play //= '';
  $noanswer //= '';
  $extra //= '';
  print " P1 = queue(" . ::FL('QUEUE',$act) . ",$play,$noanswer,$extra) ; (name,play,noanswer,extra)\n";
  print " P2 = " . $::Fields{"ROOT.Indication"} . "\n" if $::Fields{"ROOT.Indication"} =~ /^\d+$/ && $::Fields{"ROOT.Indication"};
 } else {
  print " Macro = queue ; (name,play,noanswer,extra)\n";

  print " P1 = \'" . ::FL('QUEUE',$act) . "\'\n";
  print " P2 = $play\n" if defined $play;
  print " P3 = $noanswer\n" if defined $noanswer;
  print " P4 = $extra\n" if defined $extra;
 }

 print "\n";

 if ($inline) { # Make object
  my @d;
  my @params = ($act);

  my $obj = $::OBJECT; # Must be redefined when inline!
  my $lab = $::LABEL;  #

  $::OBJECT = 'QUEUE';
  $::LABEL = $act;
#  my $fn = $::OI{'QUEUE'};
#  $fn = '' unless $fn;
#  $::LABELS{"QUEUE.$act"} = "$::did.Q$fn"; # Rewrite!

  if ($p[0] eq 'ext') {
   ::CSV($p[1], \@d);
   $::Fields{"$label.Exten"} = join(' ', @d);
  }
  QUEUE(\@params,' ; inline');

  $act = "= dial $act" unless $p[0] =~ /^([A-Za-z_.-]+|=)$/;
  unless ($::OBJECTS{"queue.$act"}) {
   my $oi = $::oi;
   $::oi = -1;
   ::Action($act, ' ; inline') unless $p[0] eq 'ext';
   $::OBJECTS{"queue.$act"} = 1;
   $::oi = $oi;
  }

  $::OBJECT = $obj;
  $::LABEL = $lab;
 }

 return "hangup" if $wait;
 return undef;
}

sub QUEUE {
 return if $::OBJECTS{"QUEUE.$::LABEL"};
 my $_ = shift; # Params array reference
 my $descr = shift // '';

 my @p = @{$_};

# my $Hunt = $::Fields{'QUEUE.Hunt'} // '';
 my $Timeout = $::Fields{"QUEUE.$::LABEL.Timeout"} // $::Fields{'ROOT.Timeout'};
 my $MOH = $::Fields{"QUEUE.$::LABEL.MOH"} // $::Fields{'ROOT.MOH'};
 my $CID = $::Fields{"QUEUE.$::LABEL.CID"} // '';
# my $Dial = $::Fields{"QUEUE.$::LABEL.Dial"};
 my @Exten = ();
 ::Keys($::Fields{"QUEUE.$::LABEL.Exten"},\@Exten);

 my $bind = '';
 $bind = "!$::BIND+" if ($::BIND ne '' && uc($::BIND) ne 'NULL');

# $Hunt = 'P' unless ($Hunt =~ /[PRS]/);

 $_ = $CID;
 $CID = '';
 $CID = '>' if /A/;
 $CID = '!' if /B/;


 print "[queues " . ::FL('QUEUE',join(' ',@p)) . "]$descr\n";
 print " musicclass = $MOH\n" if (defined $MOH && $MOH !~ /^$|^r(ing)?$/i);

 print "[queue_members]\n";
 if (@Exten) {
  for (@Exten) {
   print " interface = \"LOCAL/$bind$_\@iax/n\"\n" if ($_ ne '');
  }
 } else {
  print " interface = \"LOCAL/${bind}" . ::FN(-9999) . "\@iax/n\"\n"; # Must point always to the entry
 }

# if (defined $Dial && $Dial ne '') {
#  print "[Exten q$queue]$Description\n";
#  print " Timeout = $Timeout\n" if (defined $Timeout && $Timeout =~ /^\d+$/);
#  print " SpawnCalls = :$Hunt$CID:,$Dial\n";
# }

 print "\n";
 $::OBJECTS{"QUEUE.$::LABEL"} = 1;
 return;
}

1;
