package xdns


import (
	"execd/xlog"

//	"errors"
//	"encoding/hex"
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
type NsUpdate func(query *string, zone *string, id uint16, sig_name *string, ipaddr *string) (Rcode int)
type NsProxy func(query *dns.Msg, zone *string, tsig map[string]string) (answer *dns.Msg)

const (
	BUF_LEN = (64 * 1024) - 1 // https://github.com/fanf2/bind-9/blob/master/bin/nsupdate/nsupdate.c

/* http://users.isc.org/~each/doxygen/bind9/rcode_8c-source.html
#define RCODENAMES \
        //standard rcodes \
        { dns_rcode_noerror, "NOERROR", 0}, \
        { dns_rcode_formerr, "FORMERR", 0}, \
        { dns_rcode_servfail, "SERVFAIL", 0}, \
        { dns_rcode_nxdomain, "NXDOMAIN", 0}, \
        { dns_rcode_notimp, "NOTIMP", 0}, \
        { dns_rcode_refused, "REFUSED", 0}, \
        { dns_rcode_yxdomain, "YXDOMAIN", 0}, \
        { dns_rcode_yxrrset, "YXRRSET", 0}, \
        { dns_rcode_nxrrset, "NXRRSET", 0}, \
        { dns_rcode_notauth, "NOTAUTH", 0}, \
        { dns_rcode_notzone, "NOTZONE", 0},

#define ERCODENAMES \
        //extended rcodes \
        { dns_rcode_badvers, "BADVERS", 0}, \
        { 0, NULL, 0 }

#define TSIGRCODENAMES \
        //extended rcodes \
        { dns_tsigerror_badsig, "BADSIG", 0}, \
        { dns_tsigerror_badkey, "BADKEY", 0}, \
        { dns_tsigerror_badtime, "BADTIME", 0}, \
        { dns_tsigerror_badmode, "BADMODE", 0}, \
        { dns_tsigerror_badname, "BADNAME", 0}, \
        { dns_tsigerror_badalg, "BADALG", 0}, \
        { dns_tsigerror_badtrunc, "BADTRUNC", 0}, \
        { 0, NULL, 0 }
*/
// https://godoc.org/github.com/miekg/dns
	// Message Response Codes, see https://www.iana.org/assignments/dns-parameters/dns-parameters.xhtml
	RcodeSuccess        = 0  // NoError   - No Error                          [DNS]               *** nsupdate error string ***
	RcodeFormatError    = 1  // FormErr   - Format Error                      [DNS]               FORMERR >>> cat query | nsupdate 2>&1 | grep FORMERR
	RcodeServerFailure  = 2  // ServFail  - Server Failure                    [DNS]               SERVFAIL
	RcodeNameError      = 3  // NXDomain  - Non-Existent Domain               [DNS]               NXDOMAIN
	RcodeNotImplemented = 4  // NotImp    - Not Implemented                   [DNS]               NOTIMP
	RcodeRefused        = 5  // Refused   - Query Refused                     [DNS]               REFUSED
	RcodeYXDomain       = 6  // YXDomain  - Name Exists when it should not    [DNS Update]        YXDOMAIN
	RcodeYXRrset        = 7  // YXRRSet   - RR Set Exists when it should not  [DNS Update]        YXRRSET
	RcodeNXRrset        = 8  // NXRRSet   - RR Set that should exist does not [DNS Update]        NXRRSET
	RcodeNotAuth        = 9  // NotAuth   - Server Not Authoritative for zone [DNS Update]        NOTAUTH
	RcodeNotZone        = 10 // NotZone   - Name not contained in zone        [DNS Update/TSIG]   NOTZONE
// Codes >=16 are not supported in classic nsupdate
	RcodeBadSig         = 16 // BADSIG    - TSIG Signature Failure            [TSIG]              BADSIG
//;; TSIG PSEUDOSECTION:
//soam.                   0       ANY     TSIG    hmac-md5.sig-alg.reg.int. 1592324184 300 0  43064 BADSIG 0
	RcodeBadVers        = 16 // BADVERS   - Bad OPT Version                   [EDNS0]             BADVERS
	RcodeBadKey         = 17 // BADKEY    - Key not recognized                [TSIG]              BADKEY
//;; TSIG PSEUDOSECTION:
//soamx.                  0       ANY     TSIG    hmac-md5.sig-alg.reg.int. 1592324358 300 0  22096 BADKEY 0
	RcodeBadTime        = 18 // BADTIME   - Signature out of time window      [TSIG]              BADTIME
	RcodeBadMode        = 19 // BADMODE   - Bad TKEY Mode                     [TKEY]              BADMODE
	RcodeBadName        = 20 // BADNAME   - Duplicate key name                [TKEY]              BADNAME
	RcodeBadAlg         = 21 // BADALG    - Algorithm not supported           [TKEY]              BADALG
	RcodeBadTrunc       = 22 // BADTRUNC  - Bad Truncation                    [TSIG]              BADTRUNC
	RcodeBadCookie      = 23 // BADCOOKIE - Bad/missing Server Cookie         [DNS Cookies]
)

var (
	// hmac keys
	HMAC = make(map[string]string)
	// ACL
	Allow []net.IPNet
	Deny []net.IPNet

    xl xlog.XLog
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
		xl.Debug("CheckACL: invalid addr: " + err.Error())
		return false
	}
	ip := net.ParseIP(host)

	for _, cidr := range Allow {
		if cidr.Contains(ip) { return true }
	}

	for _, cidr := range Deny {
		if cidr.Contains(ip) { return false }
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

// Unholly shit! miekg becames crazy together with sjw's?
// He broke MsgAcceptAction() behaviour on-the-fly! (So, I've to rewrite it with no other reason)
func msgAccept(dh dns.Header) (dns.MsgAcceptAction) {
	// Header.Bits
	const _QR = 1 << 15 // query/response (response=1)
	if isResponse := dh.Bits&_QR != 0; isResponse {
		return dns.MsgIgnore
	}
	return dns.MsgAccept
}

// Return validated TSIG name, otherwise empty string
func checkTSIG(w dns.ResponseWriter, r *dns.Msg, m *dns.Msg) (string) {
    var sig_name string
    if m == nil || r == nil {
		xl.Emerg("xdns.checkTSIG: Called with empty Msg! Query from ip = " + w.RemoteAddr().String())
		return ""
    }
	if len(r.Extra) < 1 { // Tsig is absent
		xl.Warn("xdns.checkTSIG: TSIG absent in query from ip = " + w.RemoteAddr().String())
		return ""
    }

	if r.IsTsig() != nil {
		if w.TsigStatus() == nil {
			sig_name = r.Extra[len(r.Extra)-1].Header().Name
			// *Msg r has an TSIG record and it was validated
			if _, ok := HMAC[sig_name]; !ok { // Empty HMAC also leads to TsigStatus() == nil !!!
                m.MsgHdr.Rcode = dns.RcodeServerFailure
				xl.Emerg("xdns.checkTSIG: Empty HMAC slice! Unable to validate TSIG for ip = " + w.RemoteAddr().String())
				return ""
			}

			xl.Debugf("xdns.checkTSIG #%d: Query TSIG = \"%s\"", r.MsgHdr.Id, sig_name)
			return sig_name
		} else {
			// *Msg r has an TSIG records and it was not validated
// See RFC 2845 and RFC 4635. https://github.com/miekg/dns/blob/master/tsig.go
// TSIG errors MUST be returned inside r.Extra[0], together with RCODE = 9 (NOTAUTH).
// TYPE TSIG (250: Transaction SIGnature) + https://tools.ietf.org/html/rfc3597
			m.MsgHdr.Rcode = dns.RcodeNotAuth
			m.Extra = append(m.Extra, r.Extra[len(r.Extra)-1])
			rr := m.Extra[len(m.Extra)-1].(*dns.TSIG)
			rr.TimeSigned = uint64(time.Now().Unix())
			rr.MACSize = 0
			rr.MAC = ""
			rr.Error = dns.RcodeBadKey

			sig := r.Extra[len(r.Extra)-1].Header().Name
			if _, ok := HMAC[sig]; !ok {
				xl.Warnf("xdns.checkTSIG: TSIG \"%s\" KEY is unknown in query from ip = %s", sig, w.RemoteAddr().String())
			} else {
	            rr.Error = dns.RcodeBadSig
				xl.Warnf("xdns.checkTSIG: TSIG \"%s\" HMAC is invalid in query from ip = %s", sig, w.RemoteAddr().String())
			}

			switch dns.CanonicalName(rr.Algorithm) {
			case dns.HmacMD5:
			case dns.HmacSHA1:
			case dns.HmacSHA256:
			case dns.HmacSHA512:
			default:
	            rr.Error = dns.RcodeBadAlg
				xl.Warnf("xdns.checkTSIG: TSIG \"%s\" with unknown algorithm \"%s\" in query from ip = %s", sig, rr.Algorithm, w.RemoteAddr().String())
			}

			data, err := m.Pack()
			if err != nil {
			    xl.Warn("xdns.checkTSIG: Error while packing response: " + err.Error())
				return ""
			}
			_, err = w.Write(data)
			if err != nil {
			    xl.Warn("xdns.checkTSIG: Error while sending response: " + err.Error())
				return ""
			}
			m = nil  // WriteMsg() sets improper TSIG in such case
			return ""
		}
	}
	xl.Warn("xdns.checkTSIG: Unsigned query from ip = " + w.RemoteAddr().String())
	return ""
}

// TSIG is locally validated, but workers was returned the error (? WTF)
// Fabriq the correct claim
func tsigError(w dns.ResponseWriter, r *dns.Msg, m *dns.Msg) {
// m.MsgHdr.Rcode contains the error
	if m == nil || r == nil {
		xl.Emerg("xdns.tsigError: Called with empty Msg! Query from ip = " + w.RemoteAddr().String())
		return
	}
	if (m.MsgHdr.Rcode <= dns.RcodeNotZone) || (m.MsgHdr.Rcode >= dns.RcodeBadCookie) {
		xl.Emerg("xdns.tsigError: Unable to deal with extra code in Msg! Query from ip = " + w.RemoteAddr().String())
		return
	}

	m.Extra = append(m.Extra, r.Extra[0])
	rr := m.Extra[len(m.Extra)-1].(*dns.TSIG)
	rr.TimeSigned = uint64(time.Now().Unix())
	rr.Error = uint16(m.MsgHdr.Rcode)

	m.MsgHdr.Rcode = dns.RcodeNotAuth // TSIG error must return rcode = 9

	data, err := m.Pack()
	if err != nil {
	    xl.Warn("xdns.tsigError: Error while packing response: " + err.Error())
		return
	}
	_, err = w.Write(data) // Note, the response is unsigned in this case! But dns.TsigGenerate() must be rewritten to sign errors.
	if err != nil {
	    xl.Warn("xdns.tsigError: Error while sending response: " + err.Error())
		return
	}
	m = nil  // WriteMsg() sets improper TSIG in such case
	return
}

func serve(w dns.ResponseWriter, r *dns.Msg) {
	// Dynamic updates reuses the DNS message format, but renames three of the sections.
    // Question is Zone, Answer is Prerequisite, Authority is Update, only the Additional is not renamed. See RFC 2136 for the gory details.
//	xl.Debug("xdns.serve: *** Entry point")
	var m *dns.Msg
	var sig_name string
	var sig_algo string

	defer func() { // Report panic, if one occured
		 if r := recover(); r != nil {
		 	xl.Critf("xdns.serve: %v", r)
		 }
	}()

	InitMsg := func(rcode int) {
	    m = new(dns.Msg)
		m.SetReply(r)
		m.MsgHdr.Rcode = rcode
	}
	defer func() {
		if m != nil {
		    if sig_name != "" {
				m.SetTsig(sig_name, sig_algo, 300, time.Now().Unix())
			}
			w.WriteMsg(m)
		}
	}()

    if r.MsgHdr.Opcode != dns.OpcodeUpdate { // Not nsupdate
		if len(r.Question) != 1 { // Query exact 1 zone
		    InitMsg(dns.RcodeFormatError)
			xl.Debug("xdns.serve: WTF: Question length != 1")
			return
		}
		// Check ACL
		if !CheckACL(w.RemoteAddr()) {
		    InitMsg(dns.RcodeRefused)
			xl.Warn("xdns.serve: CheckACL failed for ip = " + w.RemoteAddr().String())
			return
		}
		zone := r.Question[0].Name
		// XFR? Check signature first. https://tools.ietf.org/html/rfc5936
		if r.Question[0].Qtype == dns.TypeAXFR || r.Question[0].Qtype == dns.TypeIXFR {
			// TSIG is mandatory
			InitMsg(dns.RcodeRefused)
			sig := checkTSIG(w, r, m)
			if sig == "" { return } // TSIG will be added at nsproxy(), so keep sig_name empty
			sig_algo = r.Extra[len(r.Extra)-1].(*dns.TSIG).Algorithm
			xl.Infof("xdns.serve #%d: XFR requested for zone %s from ip = %s", r.MsgHdr.Id, zone, w.RemoteAddr().String())
			m = nsproxy(r, &zone, map[string]string{sig: HMAC[sig]})
		} else {
			// TSIG is optional
			if r.IsTsig() != nil {
				InitMsg(dns.RcodeRefused)
				sig := checkTSIG(w, r, m)
				if sig == "" { return } // TSIG will be added at nsproxy(), so keep sig_name empty
				sig_algo = r.Extra[len(r.Extra)-1].(*dns.TSIG).Algorithm
				xl.Infof("xdns.serve #%d: Sined query for zone %s from ip = %s", r.MsgHdr.Id, zone, w.RemoteAddr().String())
				m = nsproxy(r, &zone, map[string]string{sig: HMAC[sig]})
			} else {
				m = nsproxy(r, &zone, nil)
			}
		}

		if m == nil {
		    InitMsg(dns.RcodeRefused)
			xl.Error("xdns.serve: nsproxy returned NULL pointer!")
		}
		return
	}

    InitMsg(dns.RcodeRefused)

// Check ACL
	if !CheckACL(w.RemoteAddr()) {
		xl.Warn("xdns.serve: CheckACL failed for ip = " + w.RemoteAddr().String())
		return
	}

	sig_name = checkTSIG(w, r, m)
	if sig_name == "" { return }
	sig_algo = r.Extra[len(r.Extra)-1].(*dns.TSIG).Algorithm

	xl.Debugf("xdns.serve #%d: Query TSIG = \"%s\"", r.MsgHdr.Id, sig_name)
	query, zone, err := nsupdate(r)
	if err != nil {
		m.MsgHdr.Rcode = dns.RcodeFormatError
		xl.Warnf("xdns.serve: nsupdate #%d zone = %s, err = %s :: %s", r.MsgHdr.Id, zone, query, err.Error())
		return
	}
	query = fmt.Sprintf("key %s %s\n", sig_name, HMAC[sig_name]) + query
	ipaddr := w.RemoteAddr().String()
	m.MsgHdr.Rcode = nsupdater(&query, &zone, r.MsgHdr.Id, &sig_name, &ipaddr)
	if m.MsgHdr.Rcode > dns.RcodeNotZone {
		tsigError(w, r, m)
	}
	xl.Debugf("xdns.serve #%d: Answer code = %d, TSIG = \"%s\"", r.MsgHdr.Id, m.MsgHdr.Rcode, sig_name)
}


func SetKeys() {
	udpServer.TsigSecret = HMAC
	tcpServer.TsigSecret = HMAC
}


func Init(ListenTCP string, ListenUDP string, callback_update NsUpdate, callback_proxy NsProxy, XLOG xlog.XLog) {
    nsupdater = callback_update
    nsproxy = callback_proxy
    xl = XLOG

	udpServer = &dns.Server{Addr: ListenUDP, Net: "udp", MsgAcceptFunc: msgAccept}
	tcpServer = &dns.Server{Addr: ListenTCP, Net: "tcp", MsgAcceptFunc: msgAccept}
	dns.HandleFunc(".", serve)
}

func Start() {
	go func() {
		if err := udpServer.ListenAndServe(); err != nil {
			xl.Emerg(err.Error())
		}
	}()
	go func() {
		if err := tcpServer.ListenAndServe(); err != nil {
			xl.Emerg(err.Error())
		}
	}()
}

func Stop() {
	udpServer.Shutdown()
	tcpServer.Shutdown()
}
