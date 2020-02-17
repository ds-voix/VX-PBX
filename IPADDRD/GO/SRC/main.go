package main

/*
 ipaddr-collector v0.1

/////////////////////////////////////////////////////////////////////////////
 Copyright (C) 2020 Dmitry Svyatogorov ds@vo-ix.ru

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU Affero General Public License as
    published by the Free Software Foundation, either version 3 of the
    License, or (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU Affero General Public License for more details.

    You should have received a copy of the GNU Affero General Public License
    along with this program.  If not, see <http://www.gnu.org/licenses/>.
/////////////////////////////////////////////////////////////////////////////
*/

import (
	"fmt"
	"log"
	"log/syslog"
	"net"
	"os"
	"os/signal"
	"os/user"
	"syscall"
	"strconv"
	"time"
	"hash/crc32"
	"github.com/google/uuid"

	flag "github.com/spf13/pflag"	// CLI keys like python's "argparse"
	"github.com/sevlyar/go-daemon"	// goroutine-safe FORK through reborn() call
									// WasReborn returns true in child process (daemon) and false in parent process.
)

const (
	BUF_LEN = 4096 + 64 + 1 + 4 + 4 + 16 // IP(max 4096) + hostname(max 64) + flag(1) + header(4) + crc32(4) + uuid(16)
	IPv4 = 5
	IPv6 = 17 // Lenght of ip records in frame
)

var (
	// Command line parser "pflag"
    DEBUG = flag.BoolP("debug", "d", false, "Debug mode switch")
	userName = flag.StringP("user", "u", "root", "Run daemon under this user")
	groupName = flag.StringP("group", "g", "", "Run daemon under this group")

	LISTEN = flag.StringP("listen", "l", ":3333", "Listen UDP IP:PORT")
	// RRL. The simple one: on fixed window.
	// Because there it is just an attack marker.
    RRL_THRESHOLD = flag.IntP("rrl_threshold", "t", 1000, "RRL threshold (number of packets per window length)")
    RRL_WINDOW = flag.IntP("rrl_window", "w", 1000, "RRL window, ms >=100")
    RRL_FACTOR = flag.IntP("rrl_factor", "r", 10, "RRL factor, integer > 0")
	RRL_FILE = flag.StringP("rrl_file_name", "f", "RRL", "File to put RRL stats into (relative to temporary path)")
    RRL_FILE_MAX = flag.Int64P("rrl_file_max", "m", 1024 * 1024, "RRL file maximum size, in bytes")
    // Prefer to use tmpfs for reporting
	REPORT_PATH = flag.StringP("report_path", "p", "/run/ipaddrd", "Store consistent reports under this path")
	REPORT_PATH_NEW = flag.StringP("report_path_tmp", "P", "/run/ipaddrd.tmp", "Store ditry reports under this path")

	// Syslog logger
	SYSLOG *syslog.Writer
	STDOUT *log.Logger
	STDERR *log.Logger

	// https://unix.superglobalmegacorp.com/Net2/newsrc/sys/syslog.h.html
    LOG_LEVEL syslog.Priority = syslog.LOG_INFO // LOG_EMERG=0 .. LOG_DEBUG=7
)


// void: Log to syslog/stdout/stderr, depending on settings
func LOG(severity syslog.Priority, message_ ...interface{}) {
    if severity > LOG_LEVEL { return }
//    message := (strings.Trim(fmt.Sprint(message_...), "[]"))
	message := fmt.Sprint(message_...)
	message = message[1:]
	message = message[:(len(message)-1)]

    var err error
	level := "DEBUG"
	switch severity {
		case syslog.LOG_EMERG:
			level = "EMERG"
		case syslog.LOG_ALERT:
			level = "ALERT"
		case syslog.LOG_CRIT:
			level = "CRIT"
		case syslog.LOG_ERR:
			level = "ERR"
		case syslog.LOG_WARNING:
			level = "WARN"
		case syslog.LOG_NOTICE:
			level = "NOTICE"
		case syslog.LOG_INFO:
			level = "INFO"
	}

	if (DEBUG != nil && *DEBUG) || (SYSLOG == nil) {
		if severity > syslog.LOG_WARNING {
			STDERR.Printf("%s: %s", level, message)
		} else {
			STDOUT.Printf("%s: %s", level, message)
		}
	} else {
		switch severity { // Double work, due to the absence of "syslog.Log(string, severity)"
			case syslog.LOG_EMERG:
				err = SYSLOG.Emerg(message)
			case syslog.LOG_ALERT:
				err = SYSLOG.Alert(message)
			case syslog.LOG_CRIT:
				err = SYSLOG.Crit(message)
			case syslog.LOG_ERR:
				err = SYSLOG.Err(message)
			case syslog.LOG_WARNING:
				err = SYSLOG.Warning(message)
			case syslog.LOG_NOTICE:
				err = SYSLOG.Notice(message)
			case syslog.LOG_INFO:
				err = SYSLOG.Info(message)
			default:
				err = SYSLOG.Debug(message)
		}
		if err != nil {
			STDERR.Printf("SYSLOG failed \"%s\" to write %s. Message was: \"%s\"", err.Error(), level, message)
		}
	}
}

// Unified log implementation. Less code >> more CPU.
func logDebug (message ...interface{}) { LOG(syslog.LOG_DEBUG, message) }
func logInfo (message ...interface{}) { LOG(syslog.LOG_INFO, message) }
func logNotice (message ...interface{}) { LOG(syslog.LOG_NOTICE, message) }
func logWarning (message ...interface{}) { LOG(syslog.LOG_WARNING, message) }
func logErr (message ...interface{}) { LOG(syslog.LOG_ERR, message) }
func logCrit (message ...interface{}) { LOG(syslog.LOG_CRIT, message) }
func logAlert (message ...interface{}) { LOG(syslog.LOG_ALERT, message) }
func logEmerg (message ...interface{}) { LOG(syslog.LOG_EMERG, message) }


func serve(pc net.PacketConn, addr net.Addr, buf []byte) {
	var (
		length int
		ip net.IP
		ipv4 []string
		ipv6 []string
		overhead bool
		hostname string
		frame_uuid uuid.UUID
	)
	if length = int(buf[0]) | int(buf[1] << 8); length != len(buf) { return } // Invalid frame length

	crc32_bytes := buf[length-4 :]
	crc32_frame := uint32(crc32_bytes[0])
	crc32_frame |= uint32(crc32_bytes[1]) << 8
	crc32_frame |= uint32(crc32_bytes[2]) << 16
	crc32_frame |= uint32(crc32_bytes[3]) << 24
	if crc32.ChecksumIEEE(buf[:length - 4]) != crc32_frame { return } // Invalid CRC

	ip_count := int(buf[2]) | int(buf[3] << 8)
	if ip_count <= 0 || ip_count > 256 { return } // Don't proceed with too many/invalid ip counts

	pointer := 4 // First ip header in buf[] is at 5'th byte
	for i := 0; i < ip_count; i++ {
		head := buf[pointer]
		if (head & 1) == 0 { // IPv4
			if length <= pointer + IPv4 + 7 { return } // Out of buffer! (7 = flag + min_hostname + crc)
			ip = buf[pointer+1 : pointer+IPv4]
			ipv4 = append(ipv4, fmt.Sprintf("%b %s", (head >> 1), ip.String()))

//	    	log.Printf("v4 %b = %s", head, ip.String())
			pointer += IPv4;
		} else { // IPv6
			if length <= pointer + IPv6 + 7 { return } // Out of buffer!
			ip = buf[pointer+1 : pointer+IPv6]
			ipv6 = append(ipv6, fmt.Sprintf("%b %s", (head >> 1), ip.String()))

//	    	log.Printf("v6 %b = %s", head, ip.String())
			pointer += IPv6;
		}
	}
    if buf[pointer] > 1 { return } // Invalid "overhead" flag
	overhead = (buf[pointer] == 1)
	hostname = fmt.Sprintf("%s", buf[pointer+1 : length-4-16])
	frame_uuid, _ = uuid.FromBytes(buf[length-4-16 : length-4])

	report_file := *REPORT_PATH_NEW + "/" + frame_uuid.String()
	f, err := os.OpenFile(report_file, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		panic(fmt.Errorf("%s Error: %s", report_file, err.Error()))
	}
	defer f.Close()

	report := fmt.Sprintf("addr=%s\nhost=%s\n", addr.String(), hostname)
	if overhead {
		report += "overhead\n"
	}

	for i := 0; i < len(ipv4); i++ {
		report += fmt.Sprintf("v4=%s\n", ipv4[i])
	}

	for i := 0; i < len(ipv6); i++ {
		report += fmt.Sprintf("v6=%s\n", ipv6[i])
	}

	f.WriteString(report)
	os.Rename(report_file, *REPORT_PATH + "/" + frame_uuid.String())

//	log.Printf("v4 %s", ipv4)
//	log.Printf("v6 %s", ipv6)
//  log.Printf("addr=%s len=%d hostname=\"%s\", %t, uuid=%s", addr.String(), len(buf), hostname, overhead, frame_uuid.String())

}


func listen_udp() {
	var f *os.File // RRL report

	defer func() {
		if f != nil {
			f.Close()
		}
	}()

    RRL_WINDOW_MS := time.Millisecond * time.Duration(*RRL_WINDOW)

	// Listen to incoming udp packets. !!! 1 thread!
	pc, err := net.ListenPacket("udp", *LISTEN)
	if err != nil {
		logEmerg(err)
	}
	defer pc.Close()

	logInfo(fmt.Sprintf("Listening on address: \"%s\"", *LISTEN))

	t0 := time.Now()
	count := 0
	rrl := 0

	for {
		buf := make([]byte, BUF_LEN)
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			continue
		}
		t1 := time.Now()
		if t1.Sub(t0) > RRL_WINDOW_MS {
			count = 0
			t0 = t1
			rrl = 0
		}
		count += 1
		if count > *RRL_THRESHOLD { // RRL
			if rrl == 0 {
				logWarning("RRL activated!")
				report := *REPORT_PATH_NEW + "/" + *RRL_FILE
				fi, err := os.Stat(report);
				if err == nil {
				    if fi.Size() > *RRL_FILE_MAX {
				    	os.Rename(report, report + ".bak")
						if f != nil {
							f.Close()
						}
				    }
				}

				if f == nil {
					f, err = os.OpenFile(report, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
					if err != nil {
						panic(fmt.Errorf("%s Error: %s", report, err.Error()))
					}
				}
				rrl = 1
                f.WriteString("RRL: " + t1.Format("2006-01-02 15:04:05-0700") + "\n")
			}
			if count % *RRL_FACTOR != 0 {
				continue
			}
		    f.WriteString(addr.String() + "\n")

		}
		go serve(pc, addr, buf[:n])
	}

	return
}


func main() {
	var (
		err error
		userRecord *user.User
		groupRecord *user.Group
		uid int
		gid int
	)
//    FALSE := false
//    DEBUG = &FALSE

	// Initialize logging pathes
	SYSLOG, err = syslog.New(syslog.LOG_DEBUG | syslog.LOG_DAEMON, "ipaddr-collector") // M.b. NULL pointer, in case of some error
	STDOUT = log.New(os.Stdout, "", log.LstdFlags)
	STDERR = log.New(os.Stderr, "", log.LstdFlags)

	defer func() { // Report panic, if one occured
		if r := recover(); r != nil {
			logEmerg(r)
		 }
	}()

//  flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()
    if *RRL_THRESHOLD < 1 {
		panic(fmt.Errorf("RRL threshold must be >0, but is \"%d\"", *RRL_THRESHOLD))
    }
    if *RRL_WINDOW < 100 {
		panic(fmt.Errorf("RRL window must be >100, but is \"%d\"", *RRL_WINDOW))
    }
    if *RRL_FACTOR < 1 {
		panic(fmt.Errorf("RRL factor must be >0, but is \"%d\"", *RRL_FACTOR))
    }
    if (*RRL_FILE_MAX < 1024) || (*RRL_FILE_MAX > 100 * 1024 * 1024) {
		panic(fmt.Errorf("RRL factor must be from 1k to 100M, but is \"%d\"", *RRL_FILE_MAX))
    }

	if err := os.MkdirAll(*REPORT_PATH, 0700); err != nil {
		panic(fmt.Errorf("Unable to create report path \"%s\": %s", *REPORT_PATH, err.Error()))
	}

	if err := os.MkdirAll(*REPORT_PATH_NEW, 0700); err != nil {
		panic(fmt.Errorf("Unable to create report path \"%s\": %s", *REPORT_PATH_NEW, err.Error()))
	}

  // Syslog for production mode
	if ! (*DEBUG) {
		userRecord, err = user.Lookup(*userName)
		if err != nil {
			panic("No such user: " + *userName)
		}
		uid, err = strconv.Atoi(userRecord.Uid)
		if err != nil {
			panic("Non-integer user id for: " + *userName)
		}

		if *groupName == "" {
			gid, err = strconv.Atoi(userRecord.Gid)
		} else {
			groupRecord, err = user.LookupGroup(*groupName)
			if err != nil {
				panic("No such group: " + *groupName)
			}
	      gid, err = strconv.Atoi(groupRecord.Gid)
    	}
		if err != nil {
			panic("Not integer group id for: " + *groupName)
		}

        if err = os.Chown(*REPORT_PATH, uid, gid); err != nil {
			panic("Unable to chown() of \"%s\"" + *REPORT_PATH)
        }
        if err = os.Chown(*REPORT_PATH_NEW, uid, gid); err != nil {
			panic("Unable to chown() of \"%s\"" + *REPORT_PATH_NEW)
        }

		cred := &syscall.Credential {
			Uid: uint32(uid),
			Gid: uint32(gid),
		}

		dcnt := &daemon.Context {
			PidFileName: "/run/ipaddr-collector.pid",
			PidFilePerm: 0644,
			Umask:       177,
			Credential: cred,
		}

		d, err := dcnt.Reborn()
		if err != nil {
			panic("Unable to fork! Error: " + err.Error())
		}

		if d != nil {
			return
		} else {
			defer dcnt.Release()
		}
	}

	// Catch (SIGINT & SIGTERM)
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		logInfo(fmt.Sprintf("Exitting by signal: \"%v\"", <-c))
		os.Exit(0)
	} ()

	listen_udp()
}
