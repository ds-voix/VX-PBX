package xdns

import (
	"execd/xlog"

//	"errors"
	"fmt"
	"net"
//	"strconv"
	"time"
//	"hash/crc32"
//	"github.com/google/uuid"
	"github.com/miekg/dns"
)

// Fold miekg/dns
type Msg dns.Msg

// Callback func
type NsUpdate func(query *string, zone *string) (Rcode int)
type NsProxy func(query *Msg, zone *string)

const (
	BUF_LEN = (64 * 1024) - 1 // https://github.com/fanf2/bind-9/blob/master/bin/nsupdate/nsupdate.c

// https://godoc.org/github.com/miekg/dns
	// Message Response Codes, see https://www.iana.org/assignments/dns-parameters/dns-parameters.xhtml
	RcodeSuccess        = 0  // NoError   - No Error                          [DNS]
	RcodeFormatError    = 1  // FormErr   - Format Error                      [DNS]
	RcodeServerFailure  = 2  // ServFail  - Server Failure                    [DNS]
	RcodeNameError      = 3  // NXDomain  - Non-Existent Domain               [DNS]
	RcodeNotImplemented = 4  // NotImp    - Not Implemented                   [DNS]
	RcodeRefused        = 5  // Refused   - Query Refused                     [DNS]
	RcodeYXDomain       = 6  // YXDomain  - Name Exists when it should not    [DNS Update]
	RcodeYXRrset        = 7  // YXRRSet   - RR Set Exists when it should not  [DNS Update]
	RcodeNXRrset        = 8  // NXRRSet   - RR Set that should exist does not [DNS Update]
	RcodeNotAuth        = 9  // NotAuth   - Server Not Authoritative for zone [DNS Update]
	RcodeNotZone        = 10 // NotZone   - Name not contained in zone        [DNS Update/TSIG]
// Codes >=16 are not supported in classic nsupdate
	RcodeBadSig         = 16 // BADSIG    - TSIG Signature Failure            [TSIG]
	RcodeBadVers        = 16 // BADVERS   - Bad OPT Version                   [EDNS0]
	RcodeBadKey         = 17 // BADKEY    - Key not recognized                [TSIG]
	RcodeBadTime        = 18 // BADTIME   - Signature out of time window      [TSIG]
	RcodeBadMode        = 19 // BADMODE   - Bad TKEY Mode                     [TKEY]
	RcodeBadName        = 20 // BADNAME   - Duplicate key name                [TKEY]
	RcodeBadAlg         = 21 // BADALG    - Algorithm not supported           [TKEY]
	RcodeBadTrunc       = 22 // BADTRUNC  - Bad Truncation                    [TSIG]
	RcodeBadCookie      = 23 // BADCOOKIE - Bad/missing Server Cookie         [DNS Cookies]
)

var (
	// hmac keys
	HMAC = make(map[string]string)
	// ACL
	Allow []net.IPNet
	Deny []net.IPNet

	nsupdater NsUpdate
	nsproxy NsProxy
	tcpServer *dns.Server
	udpServer *dns.Server
)

func SetACL(acl []string, ACL *([]net.IPNet)) (error) {
	for _, cidr := range acl {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			ip := net.ParseIP(cidr)
			if ip == nil { return err } // Not CIDR nor IP
			if ip.DefaultMask() != nil { // IPv4
				*ACL = append(*ACL, net.IPNet{ip, net.CIDRMask(32, 32)})
			} else { // IPv6
				*ACL = append(*ACL, net.IPNet{ip, net.CIDRMask(128, 128)})
			}
		} else {
			*ACL = append(*ACL, *ipnet)
		}
	}
	return nil
}

func CheckACL(addr net.Addr) (bool) {
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil { // WTF?
		xlog.Debug("CheckACL: invalid addr:", err.Error())
		return false
	}
	ip := net.ParseIP(host)

	for _, cidr := range Allow {
		if cidr.Contains (ip) { return true }
	}

	for _, cidr := range Deny {
		if cidr.Contains (ip) { return false }
	}

	return true
}

func recordType(rt uint16) (string, error) { // Type-to-string
	switch rt { // Most of popular types, but about 1/3 from all
		case dns.TypeA: return "A", nil
		case dns.TypeNS: return "NS", nil
		case dns.TypeCNAME: return "CNAME", nil
		case dns.TypeSOA: return "SOA", nil
		case dns.TypePTR: return "PTR", nil
		case dns.TypeMX: return "MX", nil
		case dns.TypeTXT: return "TXT", nil
		case dns.TypeAAAA: return "AAAA", nil
		case dns.TypeSRV: return "SRV", nil
		case dns.TypeDNAME: return "DNAME", nil
		case dns.TypeOPT: return "OPT", nil
		case dns.TypeNSEC: return "NSEC", nil
		case dns.TypeDNSKEY: return "DNSKEY", nil
	}
	return "", fmt.Errorf("Type unknown: %d", rt)
}

func rrTXT(rr dns.RR) (data string) {
	for i := 1; i <= dns.NumField(rr); i++ {
		data += " " + dns.Field(rr, i)
	}
	return data
}

// Validate & parse rfc-2136 binary query to its text representation
// https://tools.ietf.org/html/rfc2136
func nsupdate(r *dns.Msg) (query string, zone string, err error) {
	query = ""
	// Question is Zone
	if len(r.Question) != 1 { // Update exact 1 zone
		return "", "", fmt.Errorf("Zone count != 1")
	}
	zone = r.Question[0].Name
	query += fmt.Sprintf("zone %s\n", zone)

	for _, prereq := range r.Answer { // Answer is Prerequisite
		h := prereq.Header()
		switch h.Class {
			case dns.ClassANY: // YX
				switch h.Rrtype {
					case dns.TypeANY: // yxdomain {domain-name}
						query += fmt.Sprintf("yxdomain %s\n", h.Name)
		            default: // yxrrset {domain-name} [class] {type}
		            	rt, err := recordType(h.Rrtype)
		            	if err != nil {
							return fmt.Sprintf("yxrrset %s #ERR#\n", h.Name), zone, err
		            	}
						query += fmt.Sprintf("yxrrset %s IN %s\n", h.Name, rt)
				}

            case dns.ClassINET: // yxrrset {domain-name} [class] {type} {data...}
            	rt, err := recordType(h.Rrtype)
            	if err != nil {
					return fmt.Sprintf("yxrrset %s #ERR#\n", h.Name), zone, err
            	}
				query += fmt.Sprintf("yxrrset %s IN %s%s\n", h.Name, rt, rrTXT(prereq))

			case dns.ClassNONE: // NX
				switch h.Rrtype {
					case dns.TypeANY: // nxdomain {domain-name}
						query += fmt.Sprintf("nxdomain %s\n", h.Name)
		            default: // nxrrset {domain-name} [class] {type}
		            	rt, err := recordType(h.Rrtype)
		            	if err != nil {
							return fmt.Sprintf("nxrrset %s #ERR#\n", h.Name), zone, err
		            	}
						query += fmt.Sprintf("nxrrset %s IN %s\n", h.Name, rt)
				}

            default: // ERROR
				return "", zone, fmt.Errorf("Class unknown in prereq: %d", h.Class)
		}

//		query += fmt.Sprintf("prereq %s %s\n", h.Name, prereq.String())
	}

	for _, update := range r.Ns { // Authority aka "Ns" is Update
		h := update.Header()

//		fmt.Println(update)
		switch h.Class {
            case dns.ClassINET: // Add to an RRset
            	rt, err := recordType(h.Rrtype)
            	if err != nil {
					return fmt.Sprintf("add %s #ERR#\n", h.Name), zone, err
            	}
				query += fmt.Sprintf("add %s %d %s%s\n", h.Name, h.Ttl, rt, rrTXT(update))

			case dns.ClassNONE: // Delete an RR from RRs
            	rt, err := recordType(h.Rrtype)
            	if err != nil {
					return fmt.Sprintf("del %s #ERR#\n", h.Name), zone, err
            	}
				query += fmt.Sprintf("del %s %s%s\n", h.Name, rt, rrTXT(update))

			case dns.ClassANY:
				switch h.Rrtype {
					case dns.TypeANY: // Delete all RRsets from name
						query += fmt.Sprintf("del %s\n", h.Name)
		            default: // Delete an RRset
		            	rt, err := recordType(h.Rrtype)
		            	if err != nil {
							return fmt.Sprintf("del %s #ERR#\n", h.Name), zone, err
		            	}
						query += fmt.Sprintf("del %s IN %s\n", h.Name, rt)
				}

            default: // ERROR
				return "", zone, fmt.Errorf("Class unknown in update: %d", h.Class)
		}

//		query += fmt.Sprintf("update %s\n", update.String())
	}
	return query, zone, err
}

func serve(w dns.ResponseWriter, r *dns.Msg) {
	// Dynamic updates reuses the DNS message format, but renames three of the sections.
    // Question is Zone, Answer is Prerequisite, Authority is Update, only the Additional is not renamed. See RFC 2136 for the gory details.
	m := new(dns.Msg)
	m.SetReply(r)
	defer w.WriteMsg(m)

    if r.MsgHdr.Opcode != dns.OpcodeUpdate { // Not nsupdate
		if len(r.Question) != 1 { // Query exact 1 zone
			return
		}
		zone := r.Question[0].Name
		nsproxy((*Msg)(r), &zone)
		return
	}

	if len(r.Extra) < 1 { // Tsig is absent
//		fmt.Println("x", m)
		m.MsgHdr.Rcode = dns.RcodeRefused
		return
    }
// Check ACL
	if !CheckACL(w.RemoteAddr()) {
		m.MsgHdr.Rcode = dns.RcodeRefused
		return
	}

	if r.IsTsig() != nil {
		if w.TsigStatus() == nil {
			// *Msg r has an TSIG record and it was validated
			sig_name := r.Extra[0].Header().Name
			if _, ok := HMAC[sig_name]; !ok { // Empty HMAC also leads to TsigStatus() == nil !!!
				m.MsgHdr.Rcode = dns.RcodeRefused
				return
			}

			xlog.Debug(fmt.Sprintf("xdns.serve: Query TSIG = %s", sig_name))
			m.SetTsig(sig_name, dns.HmacMD5, 300, time.Now().Unix())
//			fmt.Println("*", r)
			query, zone, err := nsupdate(r)
			if err != nil {
				xlog.Warn(fmt.Sprintf("xdns.serve: nsupdate zone = %s, err = %s :: %s", zone, query, err.Error()))
                m.MsgHdr.Rcode = dns.RcodeFormatError
                return
			}
			query = fmt.Sprintf("key %s %s\n", sig_name, HMAC[sig_name]) + query
//			fmt.Printf(">\"%s\"\n", query)
			m.MsgHdr.Rcode = nsupdater(&query, &zone)
			// No, shit! "dns_request_getresponse: FORMERR"
//			m.SetTsig(sig_name, dns.HmacMD5, 300, time.Now().Unix()) // M.b. some seconds passed ("time.Now()")
		} else {
			// *Msg r has an TSIG records and it was not validated
//			fmt.Println("o", w.TsigStatus())
			m.MsgHdr.Rcode = dns.RcodeRefused
			return
		}
	}
}


func SetKeys() {
	udpServer.TsigSecret = HMAC
	tcpServer.TsigSecret = HMAC
}


func Init(ListenTCP string, ListenUDP string, callback_update NsUpdate, callback_proxy NsProxy) {
    nsupdater = callback_update
    nsproxy = callback_proxy

	udpServer = &dns.Server{Addr: ListenUDP, Net: "udp"}
	tcpServer = &dns.Server{Addr: ListenTCP, Net: "tcp"}
	dns.HandleFunc(".", serve)
}

func Start() {
	go func() {
		if err := udpServer.ListenAndServe(); err != nil {
			xlog.Emerg(err)
		}
	}()
	go func() {
		if err := tcpServer.ListenAndServe(); err != nil {
			xlog.Emerg(err)
		}
	}()
}

func Stop() {
	udpServer.Shutdown()
	tcpServer.Shutdown()
}
