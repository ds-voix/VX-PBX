package action::schedule;
# macro Schedule(Spec,f1,f2,ZT)

use strict;
use warnings;
use Exporter;

our @ISA= qw( Exporter );

# these CAN be exported.
our @EXPORT_OK = qw( schedule SCHEDULE );

# these are exported by default.
our @EXPORT = qw( schedule SCHEDULE );

sub schedule {
 my $_ = shift; # Params array reference
 my $descr = shift // '';
 my $inline = shift // 0;

 my @p = @{$_};

 print "[Func ". ::FN() ."] $descr\n";
 print " Macro = Schedule ; (Spec,f1,f2,ZT)\n";

 my $fn = $p[0] // '';
 $fn = $::LABELS{"SCHEDULE.$fn"};
 if (defined $fn) {
  $fn = "Func($fn)";
 } else {
  $fn = 'Hang() ; No action on this schedule!';
 }

 if ($inline) {
  print " P1 = \'$p[0]\'\n";
 } else {
  print " P1 = \'" . ::FL('SCHEDULE',join(' ',@p),1) . "\'\n";
 }
 print " P2 = $fn\n";
 print " P3 = Func(". ::FN(1) .")\n" if $::oi >= 0; # Break chain for special objects

 print "\n";
 return 'hangup';
}

sub SCHEDULE {
 my $_ = shift; # Params array reference
 my $descr = shift // '';

 my @p = @{$_};

 return if (defined $p[1] && $p[1] eq '=');

 print "[Schedule " . ::FL('SCHEDULE',join(' ',@p),1) ."]$descr\n";
 print " NOT = 1 ; Inverse match\n" if $p[1];

 my $s = 'Schedules';
 for (sort keys %::Fields) {
  if (/^SCHEDULE\.$::LABEL\./) {
   print "[$s]\n";
   $s = '';

   my @sched = split(/(?<=[\w\d`'"*])[\s]+(?=[\w\d`'"*])|[\s]*[^\w\d\s`'"*:.-][\s]*/, $::Fields{$_});
#   print "*0*@sched[0]*1*@sched[1]*2*@sched[2]*3*@sched[3]*4*@sched[4]***\n";
   my $not = 0;

   if ($sched[0] eq '!') {
    shift @sched;
    $not = 1;
   }
   if ($sched[0] =~ /^!/) {
    $sched[0] =~ s/^.//;
    $not = 1;
   }
   print "NOT = 1 ; Exclusion\n" if (/\!$/ || $not);
   # A bit of heuristic about what's written
#   if (defined $sched[1] && $sched[1] =~ /^(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)/i) { # Day+Month instead of time
#    unshift(@sched,undef); unshift(@sched,undef);
#   }

   if (defined $sched[0]) {
    if ($sched[0] =~ /^(sun|mon|tue|wed|thu|fri|sat)/i) { # DoW instead of time
     unshift(@sched,undef);
    } elsif ($sched[0] =~/^(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)|^\d{4}/i) { # Month|Year instead of time
     unshift(@sched,undef); unshift(@sched,undef); unshift(@sched,undef);
     unshift(@sched,undef) if $sched[3] =~/^\d{4}[^\d:]/;
    } elsif ($sched[0] =~ /^\d{1,2}[^\d:]/) { # DoM instead of time
     unshift(@sched,undef); unshift(@sched,undef);
    }
   }

   if (defined $sched[0]) {
    $sched[0] =~ s/\.\./-/g;
    $sched[0] =~ s/[^0-9:-]//g;
    print " TimeRange = $sched[0]\n";
   }
   if (defined $sched[1]) {
    $sched[1] = lc($sched[1]);
    $sched[1] =~ s/\.\./-/g;
    $sched[1] =~ s/[^\w-]//g;
    print " DaysOfWeek = $sched[1]\n";
   }
   if (defined $sched[2]) {
    $sched[2] =~ s/\.\./-/g;
    $sched[2] =~ s/[^0-9-]//g;
    print " DaysOfMonth = $sched[2]\n";
   }
   if (defined $sched[3]) {
    $sched[3] = lc($sched[3]);
    $sched[3] =~ s/\.\./-/g;
    $sched[3] =~ s/[^\w-]//g;
    print " Month = $sched[3]\n";
   }
   if (defined $sched[4]) {
    $sched[4] =~ s/\.\./-/g;
    $sched[4] =~ s/[^0-9-]//g;
    print " Year = $sched[4]\n";
   }
  }
 }
 print "; This schedule has no conditions!\n" if $s;

 print "\n";
 return;
}

1;
