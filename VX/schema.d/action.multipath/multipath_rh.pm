package action::multipath_rh;
# HW template [WWID xx] dongle

use strict;
use warnings;
use Exporter;

our @ISA= qw( Exporter );

# these CAN be exported.
our @EXPORT_OK = qw( MULTIPATH_RH );

# these are exported by default.
our @EXPORT = qw( MULTIPATH_RH );

sub MULTIPATH_RH {
 $_ = shift; # Params array reference
 my $descr = shift // '';

 my @p = @{$_};

 $descr =~ s/^[\s]*;[\s]*//;
# $::Fields{"WWID.$p[0].name"} = $descr;
# print "WWID.$p[0].name = $descr\n";

 my @cmd;
 push @cmd, `/sbin/multipath -ll "1" | /bin/egrep -o ' sd[a-z0-9]+ ' | /bin/sed 's/ //g' | /usr/bin/awk '{print "echo 1 > /sys/block/"\$1"/device/delete"}'`;
 `/sbin/dmsetup message "1" 0 "fail_if_no_path"`;

 if (defined $::Fields{"MULTIPATH.destroy"}) { # List of wwid's to be destroyed
  while( my( $wwid, $alias ) = each %{$::Fields{"MULTIPATH.destroy"}} ) {
   print "destroy $wwid = $alias \n";
   push @cmd, `/sbin/multipath -ll $wwid | /bin/egrep -o ' sd[a-z0-9]+ ' | /bin/sed 's/ //g' | /usr/bin/awk '{print "echo 1 > /sys/block/"\$1"/device/delete"}'`;

   if ("$alias" ne '') {
    `/sbin/dmsetup info "$alias"`;
    if (($? >> 8) == 0) {
     print "Sending destroying message for $alias\n";
     `/sbin/dmsetup message "$alias" 0 "fail_if_no_path"`;
    }
   }
  }
 }
 print "Deleting devices...\n";
 for my $c (@cmd) {
  print "executing: $c";
  `$c`;
 }


 `#/sbin/multipath -ll | /bin/grep -- "failed faulty" | /bin/grep -v -- "- #" | /usr/bin/awk '{print "echo 1 > /sys/block/" \$3 "/device/delete"}' | /bin/sh
/sbin/multipath -ll "1" | /bin/egrep -o ' sd[a-z0-9]+ ' | /bin/sed 's/ //g' | /usr/bin/awk '{print "echo 1 > /sys/block/"\$1"/device/delete"}' | /bin/sh
for i in \$(/usr/bin/systool -c fc_host | /bin/grep 'Class Device' | /bin/egrep -o 'host[0-9]+') ; do echo '- - -' > \$(/usr/bin/find /sys/devices/ -name 'scan' | /bin/grep "/\$i/") ; done

for i in \`/bin/ls -1 /sys/block/sd*/device/rescan\` ; do (echo 1 > \$i); done

/bin/cat <<EOF > /etc/multipath.conf
defaults {
  find_multipaths yes
  user_friendly_names yes

# If set to yes, then multipath will disable queuing when the last path to a device has been deleted.
  flush_on_last_del yes
# If set to no, the multipathd daemon will disable queuing for all devices when it is shut down. The default value is no.
  queue_without_daemon yes
# The number of seconds the SCSI layer will wait after a problem has been detected on an FC remote port before removing it from the system
  dev_loss_tmo 2147483647
# The number of seconds the SCSI layer will wait after a problem has been detected on an FC remote port before failing I/O to devices on that remote port.
  fast_io_fail_tmo 5

# A numeric value for this attribute specifies the number of times the system should attempt to use a failed path before disabling queuing.
  no_path_retry fail # A value of fail indicates immediate failure, without queuing.
}
blacklist {
        devnode               "^nbd"
}
multipaths {
### BEGIN HOOKS
### END HOOKS
}
devices {
    device {
        vendor                "DELL"
        product               "MD36xxf"
        path_grouping_policy  group_by_prio
        prio                  rdac
        path_checker          rdac
        path_selector         "round-robin 0"
        hardware_handler      "1 rdac"
        failback              immediate
        features              "2 pg_init_retries 50"
#        no_path_retry         30
        no_path_retry         fail
        rr_min_io             100
    }
    device {
        vendor                "DELL"
        product               "MD38xxf"
        path_grouping_policy  group_by_prio
        prio                  rdac
        path_checker          rdac
        path_selector         "round-robin 0"
        hardware_handler      "1 rdac"
        failback              immediate
        features              "2 pg_init_retries 50"
#        no_path_retry         30
        no_path_retry         fail
        rr_min_io             100
    }
}
EOF`;


 my $fh;
 my $conf = $::Fields{"MULTIPATH_RH.$p[0].file"} // '';
 my $content = '';

 unless (open(CONF, '<', "$conf")) {
  print STDERR "Unable to open file \"$conf\".  Error: $!\n";
  exit 1;
 }

 my $DROP = undef;
 foreach (<CONF>) { # Process config file
  $DROP = 0 if (/^[\s]*#[\s#]+END[\s]+HOOKS/);
  $content .= "$_" unless ($DROP);
  if (/^[\s]*#[\s#]+BEGIN[\s]+HOOKS/) {
   $DROP = 1;
   $content .= $::Fields{"MULTIPATH.multipaths"};
  }
 }
 close(CONF);
 return if ($content eq '');

 open($fh, '>', $conf) or die "Can't open file \"$conf\"";
 {
  local $/;
  print $fh "$content";
 }
 close($fh);

 print STDERR "Multipaths saved to file \"$conf\", now executing \"multipathd restart\": result=";
 print STDERR `/usr/sbin/service multipath-tools stop ; /bin/sleep 1 ; /usr/sbin/service multipath-tools start`;
 print STDERR `/sbin/multipath -ll | /bin/egrep 'DELL|IBM' | /usr/bin/cut -d ' ' -f 1 | /usr/bin/xargs -I XXX /sbin/dmsetup mknodes XXX`;

 print "Phase 2: cleaning...\n";
 `/sbin/dmsetup message "1" 0 "fail_if_no_path"`;

 if (defined $::Fields{"MULTIPATH.destroy"}) { # List of wwid's to be destroyed
  while( my( $wwid, $alias ) = each %{$::Fields{"MULTIPATH.destroy"}} ) {
   print "destroy $wwid = $alias \n";

   if ("$alias" ne '') {
    `/sbin/dmsetup info "$alias"`;
    if (($? >> 8) == 0) {
     print "Sending destroying message for $alias\n";
     `/sbin/dmsetup message "$alias" 0 "fail_if_no_path"`;
    }
   }
  }
 }
 print "Deleting devices...\n";
 `/sbin/multipath -ll "1" | /bin/egrep -o ' sd[a-z0-9]+ ' | /bin/sed 's/ //g' | /usr/bin/awk '{print "echo 1 > /sys/block/"\$1"/device/delete"}' | /bin/sh`;
 print STDERR `/sbin/multipathd reconfigure ; /usr/sbin/service multipathd start`;

 `/sbin/dmsetup remove -f 1 2>/dev/null`; # "1" aggregates failed unknown pathes (result of misconfigured /dev/hands), so try to auto-destroy it.
 return;
}

1;
