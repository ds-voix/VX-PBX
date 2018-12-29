package action::wwid;
# HW template [WWID xx] dongle

use strict;
use warnings;
use Exporter;

our @ISA= qw( Exporter );

# these CAN be exported.
our @EXPORT_OK = qw( WWID );

# these are exported by default.
our @EXPORT = qw( WWID );

sub WWID {
 $_ = shift; # Params array reference
 my $descr = shift // '';

 my @p = @{$_};

 my $host = $::Fields{"WWID.$p[0].host"} // '';
# print "HOST=$host\n";
 return if (($host ne '') && ("$host\n" ne `/bin/hostname`));

 $descr =~ s/^[\s]*;[\s]*//;

# print "WWID.$p[0].name = $descr\n";

 my $alias = $::Fields{"WWID.$p[0].alias"} // '';
 $alias =~ s/[^0-9A-Za-z_.-]//g unless ($alias eq '!');

 if ($alias ne '') {
  my $vm = $::Fields{"WWID.$p[0].vm"} // '';
  my $wwid = '3' . $p[0];
  $wwid =~ s/://g;
  $wwid =~ s/[^0-9A-Fa-f]//g;
  $wwid = lc($wwid);
  $wwid =~ s/^3// if ((length($wwid) == 34) && ($wwid =~ /^33/));

  if ($alias eq '!' && length($wwid) == 33) {
   $alias = `/sbin/multipath -ll $wwid | /bin/grep $wwid | /usr/bin/awk '{print \$1}'`;
   $alias =~ s/[ ]*\n$//;
   $::Fields{"MULTIPATH.destroy"}{"$wwid"} = "$alias";
   return;
   #######

   print "** Removing wwid=$wwid\n";
   print `/sbin/multipath -ll $wwid`;
   `/sbin/multipath -ll $wwid | /bin/grep -q $wwid`;
   if (($? >> 8) == 0) {
    print "Deleting devices for $wwid alias \"$alias\"\n";
    `/sbin/multipath -ll $wwid | /bin/egrep -o ' sd[a-z0-9]+ ' | /bin/sed 's/ //g' | /usr/bin/awk '{print "echo 1 > /sys/block/"\$1"/device/delete"}' | /bin/sh`;

    print `/sbin/dmsetup info "$alias"`;
    if (($? >> 8) == 0) {
     print "Sending destroying message for $alias\n";
     `/sbin/dmsetup message "$alias" 0 "fail_if_no_path"`;
     `/sbin/dmsetup info "$alias"`;
### Calling "dmsetup message" looks safer ###
#    print "Calling dmsetup remove $alias\n";
#    `/sbin/dmsetup remove -f "$alias"`;
###  Moved to hooks ###
#    `for i in \$(/usr/bin/systool -c fc_host | /bin/grep 'Class Device' | /bin/egrep -o 'host[0-9]+') ; do echo '- - -' > \$(/usr/bin/find /sys/devices/ -name 'scan' | /bin/grep "/\$i/") ; done`;
#    `/sbin/multipathd reconfigure ; /usr/sbin/service multipath-tools start`;
    }
   }
  } else {
   $wwid .= ' ### INVALID LENGTH ###' if (length($wwid) != 33);
   my $multipath = "        multipath {\n                wwid                    $wwid\n                alias                   $alias\n        }\n";
   $::Fields{"MULTIPATH.multipaths"} .= $multipath;
   $::Fields{"HOOKS.$vm"} .= " $alias" if ($vm ne '');
  }
 }
 return;
}

1;
