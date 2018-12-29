# This is a sample Perl module for the OpenLDAP server slapd.
# $OpenLDAP$
## This work is part of OpenLDAP Software <http://www.openldap.org/>.
##
## Copyright 1998-2012 The OpenLDAP Foundation.
## Portions Copyright 1999 John C. Quillan.
## All rights reserved.
##
## Redistribution and use in source and binary forms, with or without
## modification, are permitted only as authorized by the OpenLDAP
## Public License.
##
## A copy of this license is available in the file LICENSE in the
## top-level directory of the distribution or, alternatively, at
## <http://www.OpenLDAP.org/license.html>.

# Usage: Add something like this to slapd.conf:
#
# database perl
# suffix "o=VO-IX,c=RU"
# perlModulePath /directory/containing/this/module
# perlModule VersatileLDAP
#
# See the slapd-perl(5) manual page for details.
#
# This demo module keeps an in-memory hash {"DN" => "LDIF entry", ...}
# built in sub add{} & co. The data is lost when slapd shuts down.

package VersatileLDAP;
use strict;
use warnings;
use POSIX;
use IO::File;     # File IO, part of core
use DBI;
use Sys::Syslog qw(:standard :macros);

use Encode; # UTF-8 strings support
use utf8;
#use encoding 'UTF-8', STDOUT => 'UTF-8';

$VersatileLDAPDAP::VERSION = '1.00';

sub new {
    my $class = shift;

    my $this = {};
    bless $this, $class;
    print {*STDERR} "Here in new\n";
    print {*STDERR} 'Posix Var ' . BUFSIZ . ' and ' . FILENAME_MAX . "\n";
    return $this;
}

sub init {
    return 0;
}

sub search {
 my $this = shift;
 my ( $base, $scope, $deref, $sizeLim, $timeLim, $filterStr, $attrOnly,
      @attrs )
    = @_;

 my $filterCN = '';
 my $filterSN = '';
 my $filterPhone = '';

 my $cn;
 my @match_entries;

 my @CONNSTR = ("dbi:Pg:dbname=pbx","ldap","",{PrintError => 0});
 our $conn;
 my $prep;
 my $res;
 my $sql;
 my @r;
 my $str;
# CREATE USER ldap WITH PASSWORD 'WyNiCl9PuWUI' NOCREATEDB NOCREATEROLE  NOCREATEUSER;
# SELECT 'GRANT SELECT ON "'||tablename||'" TO ldap;' from pg_tables where schemaname = 'public';
 print {*STDERR} "====$filterStr====\n";

 while ($filterStr =~ m/(\([\d\w\\ =:*]+\))/g) {
  $str = $1;
  $str =~ s/\\([0-9A-F]+)/pack('U', hex($1))/eg;
  print $str . "\n";

  if ($str =~ m/^\(cn=/i) {
   $filterCN = $str;
   $filterCN =~ s/\(cn=[ ]*|\)$//g;
   $filterCN =~ s/[*]/%/g;
  }

  if ($str =~ m/^\(sn=/i) {
   $filterSN = $str;
   $filterSN =~ s/\(sn=[ ]*|\)$//g;
   $filterSN =~ s/[*]/%/g;
  }

  if ($str =~ m/^\(telephoneNumber=/i) {
   $filterPhone = $str;
   $filterPhone =~ s/\(telephoneNumber=[ ]*|\)$//g;
   $filterPhone =~ s/[*]/%/g;
  }
 }
# syslog(LOG_NOTICE, "LDAP search: " . $filterStr);

# open(LOG, '>', "/tmp/ldap1.log");
# binmode LOG;
# print LOG $base . "\n";
# print LOG $filterSN . "\n";
# print LOG $filterCN . "\n";
## $filterStr = decode 'unicode-escape', $filterStr;
# print LOG $filterStr . "\n";
# close(LOG);

#dn: cn=xxx,dc=telefonn,dc=ru
#cn: xxx
#objectClass: person
#objectClass: top
#sn: aaa

#ldapsearch -LLLD "cn=punk,dc=telefonn,dc=ru" -w 'lmd4ever' -h 192.168.5.170 -x -b "dc=telefonn,dc=ru" "(ntelephoneNumber=1234567)"
 if ($base =~ m/^(cn=.+),(dc=vo-ix,dc=ru)$/i) {
  $base = $2;
  $filterCN = $1;
  $filterCN =~ s/cn=[ ]*//g;
 }

 if ($base =~ m/^dc=vo-ix,dc=ru$/i) {
#     push @match_entries, "dn:dc=telefonn,dc=ru\ncreateTimestamp:20111206152324Z\ndc:telefonn\ndescription:Address book\nentryCSN:20111206152324.628924Z#000000#000#000000\nentryUUID: fbce7742-b469-1030-98a2-f19a21facfbd\nhasSubordinates:TRUE\nobjectClass:dcObject\nobjectClass:organization\nobjectClass:top\nstructuralObjectClass:organization\nsubschemaSubentry:cn=Subschema";
  $conn = DBI->connect(@CONNSTR);
   if ($DBI::err) {
   syslog(LOG_ERR, "ERR: Couldn't open connection: %s", $DBI::errstr);
   return ( 1 );
  }

  if ($filterSN . $filterCN . $filterPhone ne '') {
   $sql = "SELECT Name,Surname,Phone from Address where (NOT(Name = '') and (Name ~~* ?)) or (NOT(Surname = '') and (Surname ~~* ?)) or (NOT(Phone = '') and (Phone ~~* ?)) order by Name,Surname,Phone";
   $prep = $conn->prepare($sql);
#   $res = $prep->execute($conn->quote($filterSN), $conn->quote($filterCN), $conn->quote($filterPhone));
    $res = $prep->execute(decode('utf8',$filterSN), decode('utf8',$filterCN), decode('utf8',$filterPhone));

#   open(LOG, '>', "/tmp/ldap.log");
#   print LOG $sql . "\n";
#   close(LOG);

   unless (defined $res) {
    syslog(LOG_ERR, "ERR: Query failed: %s", $conn->errstr);
    return ( 1 );
   }

   while (@r = $prep->fetchrow_array())
   {
    $r[0] //= '';
    $r[1] //= '';
    $r[2] //= '';
    if ($r[1] ne '') {
     $cn = $r[1];
    } else {
     $cn = $r[2];
    }
    $str = "dn: cn=".$cn.",$base";
     foreach my $a(@attrs) {
      if (lc($a) eq 'cn') {
       $str .= "\ncn: $cn"
      }
      if (lc($a) eq 'sn') {
       $str .= "\nsn: $r[0]"
      }
      if (lc($a) eq 'givenname') {
       $str .= "\ngivenName: $r[0]"
      }
      if (lc($a) eq 'telephonenumber') {
       $str .= "\ntelephoneNumber: $r[2]"
      }
      if (lc($a) eq 'description') {
       $str .= "\ndescription: $r[2] $r[0]"
      }
      if (lc($a) eq 'objectclass') {
       $str .= "\nobjectClass: person\nobjectClass: top"
      }
     }
    unshift @match_entries, $str;
   }
  }

#  open(LOG, '>', "/tmp/ldap.log");
#  print LOG join(", ", @attrs) . "\n";
#  print LOG $sql . "\n";
#  print LOG $filterPhone . "  $#match_entries\n";
#  print LOG join(", ", @match_entries) . "\n";
#  close(LOG);


  $filterPhone =~ s/[%]//g;
  if (($#match_entries < 0) & ((length($filterPhone) == 7) | (length($filterPhone) >= 10))) {
   $sql = "SELECT \"Region\",\"Owner\" from \"QueryABC\"(\'$filterPhone\')";
   $prep = $conn->prepare($sql);
   $res = $prep->execute();

   unless (defined $res) {
    syslog(LOG_ERR, "ERR: Query failed: %s", $conn->errstr);
    return ( 1 );
   }

   while (@r = $prep->fetchrow_array())
   {
    $r[0] //= '';
    $r[1] //= '';
    unshift @match_entries, "dn: cn=".$r[1].",dc=vo-ix,dc=ru\ncn: ".$r[1]."\nobjectClass: person\nobjectClass: top\ntelephoneNumber: ".$filterPhone."\nsn: ".$r[0]."\ndescription: ".$filterPhone." ".$r[1]." ".$r[0];
   }
  }

  undef $prep;
  $conn->disconnect;
  return ( 0, @match_entries );
 }
 return 8;
}

sub compare {
    my $this = shift;
    my ( $dn, $avaStr ) = @_;
    my $rc = 5; # LDAP_COMPARE_FALSE

    $avaStr =~ s/=/: /m;

    if ( $this->{$dn} =~ /$avaStr/im ) {
        $rc = 6; # LDAP_COMPARE_TRUE
    }

    return $rc;
}

sub modify {
    my $this = shift;

    my ( $dn, @list ) = @_;

    while ( @list > 0 ) {
        my $action = shift @list;
        my $key = shift @list;
        my $value = shift @list;

        if ( $action eq 'ADD' ) {
            $this->{$dn} .= "$key: $value\n";

        }
        elsif ( $action eq 'DELETE' ) {
            $this->{$dn} =~ s/^$key:\s*$value\n//im;

        }
        elsif ( $action eq 'REPLACE' ) {
            $this->{$dn} =~ s/$key: .*$/$key: $value/im;
        }
    }

    return 0;
}

sub add {
    my $this = shift;
    print {*STDERR} "$this\n";

    my ($entryStr) = @_;

    my ($dn) = ( $entryStr =~ /dn:\s(.*)$/m );

    #
    # This needs to be here until a normalized dn is
    # passed to this routine.
    #
    $dn = uc $dn;
    $dn =~ s/\s*//gm;

    $this->{$dn} = $entryStr;

    return 0;
}

sub modrdn {
    my $this = shift;

    my ( $dn, $newdn, $delFlag ) = @_;

    $this->{$newdn} = $this->{$dn};

    if ($delFlag) {
        delete $this->{$dn};
    }
    return 0;

}

sub delete {
    my $this = shift;

    my ($dn) = @_;

    print {*STDERR} "XXXXXX $dn XXXXXXX\n";
    delete $this->{$dn};
    return 0;
}

sub config {
    my $this = shift;

    my (@args) = @_;
    local $, = ' - ';
    print {*STDERR} @args;
    print {*STDERR} "\n";
    return 0;
}

1;