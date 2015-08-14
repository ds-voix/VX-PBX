package action::hw;
# HW template processor for various ip phones/gates

use strict;
use warnings;
use Exporter;

use Cwd qw(abs_path);

our @ISA= qw( Exporter );

# these CAN be exported.
our @EXPORT_OK = qw( HW );

# these are exported by default.
our @EXPORT = qw( HW );

sub HW {
# return if $::OBJECTS{"HW.$::LABEL"};
 my $_ = shift; # Params array reference
 my $descr = shift // '';

 use Switch 'Perl6';

 my @p = @{$_};
 my $hw = lc($p[0]);

# Config values
 my $Enable1 = $::Fields{"LINE.1.enable"} // $::Fields{'ROOT.enable'} // '0';
 $Enable1 = ($Enable1 ? 'Yes':'No');
 my $Enable2 = $::Fields{"LINE.2.enable"} // $::Fields{'ROOT.enable'} // '0';
 $Enable2 = ($Enable2 ? 'Yes':'No');

 my $Host = $::Fields{'ROOT.host'} // '';
 my $Host1 = $::Fields{"LINE.1.host"} // $Host;
 my $Host2 = $::Fields{"LINE.2.host"} // $Host;

 my $DialPlan1 = $::Fields{"LINE.1.dialplan"} // $::Fields{'ROOT.dialplan'} // 'x.';
 $DialPlan1 = ::unquote(::trim($DialPlan1));
 $DialPlan1 =~ s/</&lt;/;
 $DialPlan1 =~ s/>/&gt;/;
 my $DialPlan2 = $::Fields{"LINE.2.dialplan"} // $::Fields{'ROOT.dialplan'} // 'x.';
 $DialPlan2 = ::unquote(::trim($DialPlan2));
 $DialPlan2 =~ s/</&lt;/;
 $DialPlan2 =~ s/>/&gt;/;

 my $Name1 = $::Fields{"LINE.1.name"} // '';
 my $Name2 = $::Fields{"LINE.2.name"} // '';

 my $User1 = $::Fields{"LINE.1.user"} // '';
 $User1 =~ s/(?<![\$\\])\$\$(?!\$)/$::BIND/g; # Replace $$
 my $User2 = $::Fields{"LINE.2.user"} // '';
 $User2 =~ s/(?<![\$\\])\$\$(?!\$)/$::BIND/g; # Replace $$
# print "*$User1*$User2*";

 my $Secret1 = $::Fields{"LINE.1.secret"} // $::Fields{'ROOT.secret'};
 my $Secret2 = $::Fields{"LINE.2.secret"} // $::Fields{'ROOT.secret'};

# Template file
 my $template = '';
 given ($hw) {
  when "spa2102" {
   $template = '/usr/local/sbin/schema.d/hw/spa2102.xml';
  }
  default {
   print STDERR "HW unknown: \"$hw\"\n";
   exit 1;
  }
 }


# Substitutions
 my $fh;
 my $content;
 open($fh, '<', $template) or die "Can't open file \"$template\"";
 {
  local $/;
  $content = <$fh>;
 }
 close($fh);

 $content =~ s/#NTP#/$Host/g;
 $content =~ s/#LOG#/$Host/g;
 $content =~ s/#DEBUG#/$Host/g;

 $content =~ s/#ENABLE1#/$Enable1/g;
 $content =~ s/#ENABLE2#/$Enable2/g;

 $content =~ s/#MOH1#/$Host1/g;
 $content =~ s/#MOH2#/$Host2/g;

 $content =~ s/#PROXY1#/$Host1/g;
 $content =~ s/#PROXY2#/$Host2/g;

 $content =~ s/#NAME1#/$Name1/g;
 $content =~ s/#NAME2#/$Name2/g;

 $content =~ s/#USER1#/$User1/g;
 $content =~ s/#USER2#/$User2/g;

 $content =~ s/#SECRET1#/$Secret1/g;
 $content =~ s/#SECRET2#/$Secret2/g;

 $content =~ s/#DIALPLAN1#/$DialPlan1/g;
 $content =~ s/#DIALPLAN2#/$DialPlan2/g;

# print $content;
 # Output
 my $n = $::DID;
 if ($n > 0) {
  $n = ".$n";
 } else {
  $n = '';
 }
 $n = "$::BIND$n";

 my $File = $::Fields{"HW.$p[0].file"} // '';
 $File =~ s/(?<![\$\\])\$\$(?!\$)/$n/g; # Replace $$
 $File = abs_path($File);

 if ( $File !~ /^\/var\/tftp\//) { # Prevent exploitation
  print STDERR "File path prohibited: \"$File\"\n";
  exit 1;
 }

 my $conf = $File;
 $conf =~ s/^\/var\/tftp\///;
 $content =~ s/#CONF#/$Host\/$conf/g;

 open($fh, '>', $File) or die "Can't open file \"$File\"";
 {
  local $/;
  print $fh "$content";
 }
 close($fh);

 print "Config saved to file \"$File\"\n";
# $::OBJECTS{"HW.$::LABEL"} = 1;

 # Jerk device, if any
#http://192.168.5.46/admin/resync?http://a3.ptf.spb.ru/spa_4882.1
 my $ip = $::Fields{"HW.$p[0].ip"} // '';
 return unless $ip;
 if ($ip !~ /\b((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.|$)){4}\b/) { # IPv4
  print STDERR "Invalid IPv4: \"$ip\"\n";
  exit 1;
 }

 my $url;
 given ($hw) {
  when "spa2102" {
   $url = "http://$ip/admin/resync?http://$Host/$conf";
  }
  default {
   print STDERR "HW unknown: \"$hw\"\n";
   exit 1;
  }
 }

 print "CURL $url\n\033\[1;34m";
 print `/usr/bin/curl --silent --show-error --connect-timeout 3 --max-time 3 '$url'`;
 print "\033\[0m";

 return;
}

1;
