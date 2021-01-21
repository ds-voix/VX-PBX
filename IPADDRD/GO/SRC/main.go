package main

/*
 ipaddr-collector v0.5

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
	"execd/xlog"

	"fmt"
//	"log"
//	"log/syslog"
	"net"
	"os"
	"os/signal"
	"os/user"
	"syscall"
	"strings"
	"strconv"
	"sync"
	"time"
	"hash/crc32"
	"github.com/google/uuid"

	flag "github.com/spf13/pflag"	// CLI keys like python's "argparse"
	"github.com/sevlyar/go-daemon"	// goroutine-safe FORK through reborn() call
									// WasReborn returns true in child process (daemon) and false in parent process.
//	"runtime/pprof"
//	"net/http"
//	_ "net/http/pprof"
)


const (
	BUF_LEN = 4096 + 64 + 1 + 4 + 4 + 16 // =4185  IP(max 4096) + hostname(max 64) + flag(1) + header(4) + crc32(4) + uuid(16)
	IPv4 = 5
	IPv6 = 17 // Lenght of ip records in frame
)


type BUF struct {
	sync.RWMutex
	in_use bool
	data []byte
}


var (
	DAEMON_NAME = "ipaddr-collector"

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
	REPORT_PATH = flag.StringP("report_path", "p", "/run/ipaddrd/reports", "Store consistent reports under this path")
	REPORT_PATH_NEW = flag.StringP("report_path_tmp", "P", "/run/ipaddrd/tmp", "Store ditry reports under this path")

	xl xlog.XLog
	// Ring of 100 receive buffers
	RING [100]BUF
)


func serve(addr net.Addr, BUFFER *BUF, buf_len uint32) {
	var (
		pointer uint32 = 4 // First ip header in buf[] is at 5'th byte
		ip net.IP
		ipv4 []string
		ipv6 []string
		overhead bool
		hostname string
		frame_uuid uuid.UUID
//		report strings.Builder
	)
	defer func() {
	    BUFFER.Lock()
		BUFFER.in_use = false
		BUFFER.Unlock()
	} ()

//	buf := BUFFER.data
	length := uint32(BUFFER.data[0]) | uint32(BUFFER.data[1]) << 8 // Lenght from header

	if *DEBUG { xl.Debugf("head = %d %d %d %d", int(BUFFER.data[0]), int(BUFFER.data[1]), int(BUFFER.data[2]), int(BUFFER.data[3])) }

	if length != buf_len { // Invalid frame length
		if *DEBUG { xl.Debugf("Invalid frame length %d != %d", length, buf_len) }
		return
	}


	crc32_bytes := BUFFER.data[length-4 :]
	crc32_frame := uint32(crc32_bytes[0])
	crc32_frame |= uint32(crc32_bytes[1]) << 8
	crc32_frame |= uint32(crc32_bytes[2]) << 16
	crc32_frame |= uint32(crc32_bytes[3]) << 24
	if crc32.ChecksumIEEE(BUFFER.data[:length - 4]) != crc32_frame { // Invalid CRC
		if *DEBUG { xl.Debug("Invalid CRC") }
		return
	}

	ip_count := uint32(BUFFER.data[2]) | uint32(BUFFER.data[3]) << 8
	if ip_count <= 0 || ip_count > 256 { // Don't proceed with too many/invalid ip counts
		if *DEBUG { xl.Debugf("More than 256 ip in header: %d", ip_count) }
		return
	}

	for i := uint32(0); i < ip_count; i++ {
		head := BUFFER.data[pointer]
		if (head & 1) == 0 { // IPv4
			if pointer + IPv4 + 7 >= length { // Out of buffer! (7 = flag + min_hostname + crc)
				if *DEBUG { xl.Debug("Malformed frame: pointer went out of buffer!") }
				return
			}
			ip = BUFFER.data[pointer+1 : pointer+IPv4]
//			ipv4 = append(ipv4, fmt.Sprintf("%b %s", (head >> 1), ip.String()))
			ipv4 = append(ipv4, strconv.FormatInt(int64(head >> 1), 2) + " " + ip.String())

//	    	log.Printf("v4 %b = %s", head, ip.String())
			pointer += IPv4;
		} else { // IPv6
			if pointer + IPv6 + 7 >= length { // Out of buffer! (7 = flag + min_hostname + crc)
				if *DEBUG { xl.Debug("Malformed frame: pointer went out of buffer!") }
				return
			}
			ip = BUFFER.data[pointer+1 : pointer+IPv6]
			ipv6 = append(ipv6, fmt.Sprintf("%b %s", (head >> 1), ip.String()))

//	    	log.Printf("v6 %b = %s", head, ip.String())
			pointer += IPv6;
		}
	}
    if BUFFER.data[pointer] > 1 { // Invalid "overhead" flag
		if *DEBUG { xl.Debug("Invalid \"overhead\" flag in frame") }
		return
	}
	overhead = (BUFFER.data[pointer] == 1)
	hostname = fmt.Sprintf("%s", BUFFER.data[pointer+1 : length-4-16])
	frame_uuid, _ = uuid.FromBytes(BUFFER.data[length-4-16 : length-4])

	report_file := *REPORT_PATH_NEW + "/" + frame_uuid.String()
	f, err := os.OpenFile(report_file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640) // O_EXCL: file must not exist
	if err != nil {
		os.Remove(report_file)
		f, err = os.OpenFile(report_file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640) // O_EXCL: file must not exist
		if err != nil { // WTF!?
			xl.Emergf("%s Error: %s", report_file, err.Error())
			panic(fmt.Errorf("%s Error: %s", report_file, err.Error()))
		}
	}

	defer func() {
//		f.WriteString(report.String())
		f.Sync()
		f.Close()
//		time.Sleep(100 * time.Millisecond)
// !! Rename() found not been atomic. At least, on tmpfs. Renamed file may still not exist, while old file already been removed.
		os.Rename(report_file, *REPORT_PATH + "/" + frame_uuid.String())
	} ()

	f.WriteString(fmt.Sprintf("addr=%s\nhost=%s\n", addr.String(), hostname))
	if overhead {
		f.WriteString("overhead\n")
	}

	for i := 0; i < len(ipv4); i++ {
		f.WriteString(fmt.Sprintf("v4=%s\n", ipv4[i]))
	}

	for i := 0; i < len(ipv6); i++ {
		f.WriteString(fmt.Sprintf("v6=%s\n", ipv6[i]))
	}
}


func listen_udp() {
	var f *os.File // RRL report

	defer func() {
		if f != nil {
			f.Close()
		}
	}()

/* Doesn't work
	// CPU profiling
	pr, _ := os.Create("/run/ipaddrd/cpuprofile")
	defer pr.Close()
	pprof.StartCPUProfile(pr)
	defer pprof.StopCPUProfile()
*/

    RRL_WINDOW_MS := time.Millisecond * time.Duration(*RRL_WINDOW)

	// Listen to incoming udp packets. !!! 1 thread!
	pc, err := net.ListenPacket("udp", *LISTEN)
	if err != nil {
	 	xl.Emergf("%v", err)
	}
	defer pc.Close()

	xl.Infof("Listening on address: \"%s\"", *LISTEN)

	t0 := time.Now()
	count := 0
	rrl := 0

	for i:=0; i<100; i++ {
		RING[i].data = make([]byte, BUF_LEN)
		RING[i].in_use = false
	}
	ring_pos := 0 // Pointer to current buffer in the RING

	for {
//		buf := make([]byte, BUF_LEN)
		n, addr, err := pc.ReadFrom(RING[ring_pos].data)
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
				xl.Warning("RRL activated!")
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
					f, err = os.OpenFile(report, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0640)
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

		RING[ring_pos].in_use = true // No need in Lock(), we are inside of the single loop.
		go serve(addr, &(RING[ring_pos]), uint32(n))
		ring_pos++ // Unlike C, "++" can't be inlined
		if ring_pos > 99 { ring_pos = 0 }
		in_use := false
		for { // Wait the next buffer to be released, if not already is.
			RING[ring_pos].RLock()
			in_use = RING[ring_pos].in_use
			RING[ring_pos].RUnlock()
			if in_use { // Wait
			    time.Sleep(10 * time.Millisecond)
				continue
			}
			break
		}
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
	xl = xlog.New(DAEMON_NAME)

	defer func() { // Report panic, if one occured
		 if r := recover(); r != nil {
		 	xl.Emergf("%v", r)
		 }
	}()

   	if env_log, ok := os.LookupEnv("LOG"); ok {
		switch strings.ToUpper(env_log) {
			case "DEBUG":
				xl.LOG_LEVEL = xlog.LOG_DEBUG
				d := true
				xl.DEBUG = &d
			case "INFO":
				xl.LOG_LEVEL = xlog.LOG_INFO
			case "NOTICE":
				xl.LOG_LEVEL = xlog.LOG_NOTICE
			case "ERR":
				xl.LOG_LEVEL = xlog.LOG_ERR
			case "CRIT":
				xl.LOG_LEVEL = xlog.LOG_CRIT
			default:
				xl.LOG_LEVEL = xlog.LOG_WARNING
		}
	} else {
		xl.LOG_LEVEL = xlog.LOG_INFO
	}


//  flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()
	if *DEBUG {
		xl.LOG_LEVEL = xlog.LOG_DEBUG
		d := true
		xl.DEBUG = &d
	}

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

	if err := os.MkdirAll(*REPORT_PATH, 0750); err != nil {
		panic(fmt.Errorf("Unable to create report path \"%s\": %s", *REPORT_PATH, err.Error()))
	}

	if err := os.MkdirAll(*REPORT_PATH_NEW, 0750); err != nil {
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
			Umask:       0117,
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
		xl.Infof("Exitting by signal: \"%v\"", <-c)
		os.Exit(0)
	} ()

//	go http.ListenAndServe(":8080", nil)
	listen_udp()
}
