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
# print "*** $::DID\n";
 $bind = "$::BIND/" if ($::BIND ne '' && uc($::BIND) ne 'NULL');

 if (defined $::Fields{"$label.hello"}) {
  $play = "\'$bind" . ::unquote($::Fields{"$label.hello"})."\'";
 } else {
  if (defined $::Fields{"ROOT.hello"}) {
   $play = "\'$bind" . ::unquote($::Fields{"ROOT.hello"})."\'";
   delete $::Fields{"ROOT.hello"};
  }
 }

 my $wait = $::Fields{"ROOT.wait"} // '';

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
   $::Fields{"$label.MOH"} //= $::Fields{"ROOT.moh"} // $d[1];
#   $play = "$bind" . ::unquote($p[2]) if defined $p[2];
  } else {
   if ($p[0] eq '=' && $p[1] =~ /^(dial|queue)$/i) {
    my @d = ();
    ::CSV($p[3], \@d); # Spec
    $extra = undef if defined $d[1] && $d[1] !~ /^r(ing)?$/i;
    $::Fields{"$label.MOH"} = $d[1];
    $::Fields{"$label.MOH"} //= $::Fields{"ROOT.moh"} // $d[1];
    $play = "$bind" . ::unquote($p[4]) if defined $p[4];
   }
  }

  if ($p[0] =~ /^(ext|action)$/i) {
   if (defined $p[2]) {
    $extra = undef if defined $p[2] && $p[2] !~ /^r(ing)?$/i;
    $::Fields{"$label.MOH"} //= $::Fields{"ROOT.moh"} // $p[2];
   }
   $play = "$bind" . ::unquote($p[3]) if defined $p[3];
  }
 } else {
  $act = join(' ', @p);
  $wait = $p[3] if defined $p[3];
  $extra = undef if defined $p[1] && $p[1] !~ /^r(ing)?$/i;
  $play = "$bind" . ::unquote($p[2]) if defined $p[2] && $p[2] =~ /[\d\w]/;
 }

 $wait =~ s/\D//g;
 if ($wait) {
  $extra //= '';
  $extra .= ",,,$wait";
  $extra .= ":Func(". ::FN(1) .")" if $::oi >= 0; # Break chain for special objects
 }

 print "[Func ". ::FN() ."] $descr\n";

 if (defined $::Fields{"ROOT.indication"}) {
  print " Macro = Indication ; (fn,Timeout,NoTransfer,Limit)\n";
  $play //= '';
  $noanswer //= '';
  $extra //= '';
  print " P1 = queue(" . ::FL('QUEUE',$act) . ",$play,$noanswer,$extra) ; (name,play,noanswer,extra)\n";
  print " P2 = " . $::Fields{"ROOT.indication"} . "\n" if $::Fields{"ROOT.indication"} =~ /^\d+$/ && $::Fields{"ROOT.indication"};
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

# my $Hunt = $::Fields{'QUEUE.hunt'} // '';
 my $Timeout = $::Fields{"QUEUE.$::LABEL.timeout"} // $::Fields{'ROOT.timeout'};
 my $MOH = $::Fields{"QUEUE.$::LABEL.moh"} // $::Fields{'ROOT.moh'};
 my $CID = $::Fields{"QUEUE.$::LABEL.cid"} // '';
# my $Dial = $::Fields{"QUEUE.$::LABEL.dial"};
 my @Exten = ();
 ::Keys($::Fields{"QUEUE.$::LABEL.exten"},\@Exten);

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
