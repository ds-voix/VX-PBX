package main
/*
 execd v0.9
/////////////////////////////////////////////////////////////////////////////
 Copyright (C) 2019 Dmitry Svyatogorov ds@vo-ix.ru

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

 Execute json-serialized command batches, using Apache Kafka as the shared HA transport.
 Store json-serialized reports back into the Kafka journal.
 Journal offsets are commited to local storage, thus providing the way to do the consistent  state snapshot.
 All comunications are SSL-folded. Client can be authorized either by it's IP or SSL cert.

 This is the fundamental part of the shared-journals based framework.
 Can be implemented to share the state across the world.
 E.g., "multi-master" DNS (in fact, multi-slave), LDAP and so on.

 "execd" is designed to be the interface between shared messaging bus and the regular software.

 Look at the "Commands" << "Command" and "ExecResult" structs to get the reference on messaging format.

 Env: LOG=(DEBUG|INFO|...) to specify local log verbosity.
 * DEBUG althougt triggers the output to the stdout|stderr. Othewise, syslog is used.
   It's highly recommended to trigger debug while the first run, to control that all the things are working as intended.
   Debug is quit compact to be usefull.

 CLI: -d for debug, -v to enhance verbosity, -c to set the path to config, instead of "/etc/execd/execd.conf".
 * Use "--kafka.topic=initial_offset" keys on the first run to initialize journal positions.
   There are although two special offsets: -1 points to "OffsetNewest" and -2 to "OffsetOldest".
   The real offets in kafka journals starts with 0.

 Known problems: as for v0.9, execd	abstracts about all the kafka interoperation to sarama library.
 Therefore, connection problems will be hidden, except of DEBUG level.
 The current implementation will repeat attempts to get connection working for 24 hours. (!) Silently, when debug is off.
 Then, falls to panic.

 Measured resourse consumption, on idle running.
    CPU: <0.1% (less then 200 seconds per 24 hours)
    RAM: <15 MB at the host with 4GB (go-runtime consumes RAM depending on the environment)
    NET: <4 kB/s, both ingress/egress
    HDD: 0 B/s (nothing to do - nothing to RW)
*/

// Start to deal with kafka example https://gist.github.com/nilsmagnus/4b582f9a36279bff5f8f9d453f8fb9c4
// * Note new https://github.com/trivago/gollum = message router|filter|convertor
// Speed up code: https://habr.com/ru/company/oleg-bunin/blog/461291/ (beware: 50/50% reliability)

import (
	"bytes"
	"crypto/sha1"
//	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"		// TOML
	"github.com/Showmax/go-fqdn"        // Simple wrapper around net and os golang standard libraries providing Fully Qualified Domain Name
//	"github.com/samuel/go-zookeeper/zk" // zookeeper
	// https://github.com/Shopify/sarama/wiki/Frequently-Asked-Questions
	"github.com/Shopify/sarama"         // kafka
//	flag "github.com/juju/gnuflag"
	flag "github.com/spf13/pflag"       // CLI keys like python's "argparse". More flexible, comparing with "gnuflag"
	"github.com/juju/mutex"             // Interprocess mutex
	"io"
	"io/ioutil"
	"log"
	"log/syslog"
	"net"
	"os"
	"os/exec"
	"os/user"
	"os/signal"
	"regexp"
	// https://medium.com/samsara-engineering/running-go-on-low-memory-devices-536e1ca2fe8f
	"runtime/debug" // func FreeOSMemory()
	"strconv"
	"strings"
	"syscall"
	"sync"
	"time"
//	"reflect"
)

// Config tree
type ZooKeeper struct { // Currently, no need in. Kafka is enough.
	Cluster []string
	Timeout int
}
type Kafka struct {
	Brokers []string
	CAcertFile string
	PrivateKeyFile string
	PrivateKeyPassword string
	CertFile string
	InsecureSkipVerify bool
	ClientID string
	DialTimeout int
	ReadTimeout int
	WriteTimeout int
	KeepAlive int
	LocalAddr string // https://godoc.org/net#Addr
}
type Consume struct {
	Topics []string
	Partition int32
	LocalDirectory string
	FetchMax int32
	RetryBackoff int
	RetryMax int
}
type Produce struct {
	Topic string
	Partition int
	LocalDirectory string
	MaxMessageBytes int
	Timeout int
	RetryBackoff int
	RetryMax int
}
type Hooks struct {
	Start string
	Stop string
	Produce bool
	Tag string
}
type Config struct {
	ZooKeeper ZooKeeper  // ZooKeeper connection settings
	Kafka     Kafka      // Kafka connection settings
	Consume   Consume    // Kafka journal to get command batches
	Produce   Produce    // Kafka journal to output results
	Hooks     Hooks      // Run hooks after start/before stop
}

// https://goinbigdata.com/golang-pass-by-pointer-vs-pass-by-value/
// "Maps and slices are reference types in Go and should be passed by values."
// "Passing by value simplifies escape analysis in Go and gives variable a better chance to be allocated on the stack."
// !!! So: unlike C/C++, I must avoid using pointers! Except of structs to be changed inside function. !!!

// Command execution result
type ExecResult struct {
	ID string             `json:"id"`
	Processed bool        `json:"processed"` // Was this command ever been processed?
	Command string        `json:"command"`
	Args []string         `json:"args,omitempty"`
	Status int            `json:"status"`
	StdOut []byte         `json:"stdout,omitempty"`
	StdErr []byte         `json:"stderr,omitempty"`
}

type ExecResults struct {
	Results []*ExecResult `json:"results"`
}

type Command struct {
	id string        // Sequence ID may be set. Othewise, it is auto-generated.
	no_exec bool     // "Dry run", no real execution
	error_fails bool // Break execution on error
	uid, gid uint32  // "run_as" linux representation
	use_shell bool
	no_wait bool     // https://golang.org/pkg/os/exec/#Cmd.Start vs "Run()" or m.b. "Wait()"
	timeout float64
	max_reply int64
	host_regex string
// https://golang.org/pkg/os/exec/#Cmd
	set_env []string
	set_dir string
	command string
	args []string
	stdin []byte
}

// Default values for []Command array
type Defaults struct {
	no_exec bool
	error_fails bool  // Break execution on error
	uid, gid uint32   // "run_as" linux representation
	use_shell bool
	set_env []string
	set_dir string
	no_wait bool      // https://golang.org/pkg/os/exec/#Cmd.Start vs "Run()" or m.b. "Wait()"
	timeout float64
	max_reply int64
}

type Commands struct {
	producer string    // ClientID of producer
	uuid string        // UUID for reporting
	host_regex string  // Is this message intended for localhost?
	tag string         // Some tag can be set, to facilitate further analysis.
	commands []Command
// After execution, store result together with command. To facilitate reporting.
	results ExecResults // Separated structure, to be json-serialized
}

var (
	MUTEX mutex.Releaser
	CONF *Config
	OFFSETS = make(map [string]int64)
	OFFSETS_ON_START = make(map [string]int64)        // Last offsets in journal
	OFFSETS_ON_START_OLDEST = make(map [string]int64) // First offsets in journal

	config *sarama.Config
	client sarama.Client
	PRODUCER sarama.SyncProducer
	PRODUCER_AGE time.Time

	CAcert []byte
	privateKey []byte
	cert []byte

	hookStart bool
	hookStop bool
	exitSignal os.Signal // Report exit cause (except of SIGKILL)

	// Syslog logger
	SYSLOG *syslog.Writer
	STDOUT *log.Logger
	STDERR *log.Logger

	DEBUG *bool
	// https://unix.superglobalmegacorp.com/Net2/newsrc/sys/syslog.h.html
	LOG_LEVEL syslog.Priority // LOG_EMERG=0 .. LOG_DEBUG=7
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


// https://golang.org/pkg/os/exec/#Cmd
func execCommand(c *Command) (*ExecResult) {
	r := &ExecResult{ID: c.id,
					Command: c.command,
					Args: c.args}
	cmd := exec.Command(c.command)
	if c.use_shell { // Pack into bash environment (In fact, bash is *sh's mainstream. And I unwill to deep into specifics)
		cmd = exec.Command("/bin/bash")
		// -c If the -c option is present, then commands are read from the first non-option argument command_string.  If there are arguments after the command_string, the first argu-
		//    ment is assigned to $0 and any remaining arguments are assigned to the positional parameters.  The assignment to $0 sets the name of the shell, which is used in warning
		//    and error messages.
		cmd.Args = append(cmd.Args, "-c")
		cmd.Args = append(cmd.Args, c.command)
	}
	// https://medium.com/@felixge/killing-a-child-process-and-all-of-its-children-in-go-54079af94773
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	cmd.Args = append(cmd.Args, c.args...)
	cmd.Stdin = bytes.NewReader(c.stdin)

	// https://stackoverflow.com/questions/21705950/running-external-commands-through-os-exec-under-another-user
	if c.uid > 0 {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: c.uid, Gid: c.gid}
	}
	if len(c.set_dir) > 0 {
		cmd.Dir = c.set_dir
	}
	if len(c.set_env) > 0 {
		cmd.Env = append(cmd.Env, c.set_env...)
	}

	// http://www.agardner.me/golang/garbage/collection/gc/escape/analysis/2015/10/18/go-escape-analysis.html
	// https://habr.com/ru/company/intel/blog/422447/
	var _stdout, _stderr     bytes.Buffer
	var stdoutIn, stderrIn   io.ReadCloser
	var errStdout, errStderr error

	stdout := io.Writer(&_stdout)
	stderr := io.Writer(&_stderr)
  	if ! c.no_wait { // Otherwise, just fork process with /dev/null at stdout&stderr.
		stdoutIn, _ = cmd.StdoutPipe()
		stderrIn, _ = cmd.StderrPipe()
	}

	r.Status = 0
	if c.no_exec {
		r.StdErr = []byte("no_exec")
		return r
	}

	err := cmd.Start()
	r.Processed = true
	if err != nil {
		r.StdErr= []byte(err.Error())
		if exitErr, ok := err.(*exec.ExitError); ok {
			if _status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				r.Status = _status.ExitStatus()
			}
		} else {
			r.Status = -1 // No such command at all?
		}

		return r
	}
//	fmt.Printf("pid = %d\n", cmd.Process.Pid)

	if c.no_wait {
		return r
	}

	// Wait for the process to finish or kill it after a timeout (whichever happens first):
	go func(cmd *exec.Cmd, pid int) { // !!! panic: runtime error: invalid memory address or nil pointer dereference
		for i := 0; i < int(c.timeout * 10); i++ {
			time.Sleep(time.Second / 10)
			if cmd == nil { return }
			if cmd.ProcessState != nil {
				break
			}
		}

		if cmd.ProcessState == nil {
//			cmd.Process.Signal(syscall.SIGTERM) // No, such a method terminates only this PID, leaving orphans!
			if cmd == nil { return }
			syscall.Kill(-pid, syscall.SIGTERM) // !!! [signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x7c1ef0]
			time.Sleep(time.Second / 2)
			if cmd == nil { return }
			syscall.Kill(-pid, syscall.SIGKILL)
		}
	} (cmd, cmd.Process.Pid)

	// https://blog.kowalczyk.info/article/wOYk/advanced-command-execution-in-go-with-osexec.html
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		_, errStdout = io.CopyN(stdout, stdoutIn, c.max_reply)
		if errStdout == io.EOF {
			errStdout = nil
		} else {
			io.Copy(ioutil.Discard, stdoutIn) // Discard all the rest of stdout
		}
		wg.Done()
	} ()
	_, errStderr = io.CopyN(stderr, stderrIn, c.max_reply)
	if errStderr == io.EOF {
		errStderr = nil
	} else {
		io.Copy(ioutil.Discard, stderrIn) // Discard all the rest of stderr
	}
	wg.Wait()

	err = cmd.Wait()
	if err != nil {
		r.StdErr = []byte(err.Error())
		if exitErr, ok := err.(*exec.ExitError); ok {
			if _status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				r.Status = _status.ExitStatus()
			}
		} else {
			r.Status = -1 // No such command at all?
		}

	}

	if errStdout != nil {
		r.StdOut = []byte(errStdout.Error())
		r.Status = -254
	}
	if errStderr != nil {
		r.StdErr = []byte(errStderr.Error())
		r.Status = -255
	}

	r.StdOut = _stdout.Bytes()
	r.StdErr = _stderr.Bytes()
	return r
}


// https://www.w3schools.com/js/js_json_datatypes.asp
func jsonCommand(msg []byte, DEFAULTS Defaults, seq int) (COMMAND *Command, ERROR string) {
	var result map [string]interface{}
	cmd := &Command{id: string(seq),
					no_exec: DEFAULTS.no_exec,
					error_fails: DEFAULTS.error_fails,
					uid: DEFAULTS.uid,
					gid: DEFAULTS.gid,
					use_shell: DEFAULTS.use_shell,
					set_env: DEFAULTS.set_env,
					set_dir: DEFAULTS.set_dir,
					no_wait: DEFAULTS.no_wait,
					timeout: DEFAULTS.timeout,
					max_reply: DEFAULTS.max_reply,
					}

	err := json.Unmarshal(msg, &result)
	if err != nil {
		return nil, err.Error()
	}

	// Parse "soft" structure: most of message values can be of more then 1 type
	for key, value := range result {
		switch key { // strings.Trim(key, "\"'`")
		case "id":
			switch t := value.(type) {
			case string:
				cmd.id, _ = value.(string)
			case float64:
				id, _ := value.(float64)
				if id == float64(int(id)) { // Is it integer?
					cmd.id = fmt.Sprintf("%d", int(id))
				} else {
					cmd.id = fmt.Sprintf("%f", id)
				}
			default:
				return nil, fmt.Sprintf("\"id\" field isn't [string|number], but [%T]", t)
			} // switch t
		case "no_exec":
			switch t := value.(type) {
			case float64:
				cmd.no_exec = (value.(float64) > 0)
			case bool:
				cmd.no_exec, _ = value.(bool)
			default:
				return nil, fmt.Sprintf("\"no_exec\" field isn't [bool|number], but [%T]", t)
			} // switch t
		case "run_as":
			switch t := value.(type) {
			case string:
				run_as := value.(string)
				cred := strings.Split(run_as, ":")
				// https://golang.org/pkg/os/user/#User
				user_, err := user.Lookup(cred[0])
				if err != nil {
					return nil, fmt.Sprintf("user invalid in \"run_as\": %s", cred[0])
				}
				u64, err := strconv.ParseUint(user_.Uid, 10, 32)
				if err != nil {
					return nil, fmt.Sprintf("non-integer uid! Is it linux?")
				}
				cmd.uid = uint32(u64)

				gid := ""
				if len(cred) > 1 {
					group, err := user.LookupGroup(cred[1])
					if err != nil {
						return nil, fmt.Sprintf("group invalid in \"run_as\": %s", cred[1])
					}
					gid = group.Gid
				} else {
					gid = user_.Gid
				}
				g64, err := strconv.ParseUint(gid, 10, 32)
				if err != nil {
					return nil, fmt.Sprintf("non-integer gid! Is it linux?")
				}
				cmd.gid = uint32(g64)

			default:
				return nil, fmt.Sprintf("\"run_as\" field isn't [string], but [%T]", t)
			} // switch t
		case "use_shell":
			switch t := value.(type) {
			case float64:
				cmd.use_shell = (value.(float64) > 0)
			case bool:
				cmd.use_shell, _ = value.(bool)
			default:
				return nil, fmt.Sprintf("\"use_shell\" field isn't [bool|number], but [%T]", t)
			} // switch t
		case "no_wait":
			switch t := value.(type) {
			case float64:
				cmd.no_wait = (value.(float64) > 0)
			case bool:
				cmd.no_wait, _ = value.(bool)
			default:
				return nil, fmt.Sprintf("\"no_wait\" field isn't [bool|number], but [%T]", t)
			} // switch t
		case "timeout":
			switch t := value.(type) {
			case float64:
				cmd.timeout = value.(float64)
				if cmd.timeout < 0 {
					return nil, "\"timeout\" less than 0!"
				}
			default:
				return nil, fmt.Sprintf("\"timeout\" field isn't [number], but [%T]", t)
			} // switch t
		case "max_reply":
			switch t := value.(type) {
			case float64:
				cmd.max_reply = int64(value.(float64))
				if cmd.max_reply < 0 {
					return nil, "\"max_reply\" less than 0!"
				}
			default:
				return nil, fmt.Sprintf("\"max_reply\" field isn't [number], but [%T]", t)
			} // switch t
		case "host_regex":
			switch t := value.(type) {
			case string:
				cmd.host_regex= value.(string)
			default:
				return nil, fmt.Sprintf("\"host_regex\" field isn't [string], but [%T]", t)
			} // switch t
		case "set_env":
			switch t := value.(type) {
			case string:
				cmd.set_env = append(cmd.set_env, value.(string))
			case []interface{}:
				for i, v := range t {
					switch vt := v.(type) {
					case string:
						cmd.set_env = append(cmd.set_env, v.(string))
//						fmt.Printf("env = %s\n", v.(string))
					default:
						return nil, fmt.Sprintf("\"set_env[%d]\" field isn't [string], but [%T]", i, vt)
					}
				}
			default:
				return nil, fmt.Sprintf("\"set_env\" field isn't [string], but [%T]", t)
			} // switch t
		case "set_dir":
			switch t := value.(type) {
			case string:
				cmd.set_dir = value.(string)
			default:
				return nil, fmt.Sprintf("\"set_dir\" field isn't [string], but [%T]", t)
			} // switch t
		case "command":
			switch t := value.(type) {
			case string:
				cmd.command = value.(string)
//				command_b, err := base64.StdEncoding.DecodeString(cmd.command) // []byte
//				if err != nil {
//					cmd.command = string(command_b)
//				}
			default:
				return nil, fmt.Sprintf("\"command\" field isn't [string], but [%T]", t)
			} // switch t
		case "stdin":
			switch t := value.(type) {
			case string:
				cmd.stdin = []byte(value.(string))
			default:
				return nil, fmt.Sprintf("\"stdin\" field isn't [string], but [%T]", t)
			} // switch t
		case "stdin_64":
			switch t := value.(type) {
			case string:
				stdin, err := base64.StdEncoding.DecodeString(value.(string)) // []byte
				if err != nil {
					return nil, fmt.Sprintf("\"stdin_64\" field isn't [base64-encoded string]: %s", err.Error())
				}
				cmd.stdin = stdin
			default:
				return nil, fmt.Sprintf("\"stdin_64\" field isn't [string], but [%T]", t)
			} // switch t

		case "args":
			switch t := value.(type) {
			case string:
				cmd.args = append(cmd.args, value.(string))
			case []interface{}:
				for i, v := range t {
					switch vt := v.(type) {
					case string:
						cmd.args = append(cmd.args, v.(string))
//						fmt.Printf("env = %s\n", v.(string))
					default:
						return nil, fmt.Sprintf("\"args[%d]\" field isn't [string], but [%T]", i, vt)
					}
				}
			default:
				return nil, fmt.Sprintf("\"args\" field isn't [string], but [%T]", t)
			} // switch t

		default:
			return nil, fmt.Sprintf("Unknown key in message: \"%s\"", key)
		} // switch key
	} // for key, value

	if cmd.command == "" {
		return nil, "Mandatory \"command\" field not in json"
	}

	return cmd, ""
}


func jsonMessage(msg []byte) (COMMANDS *Commands, ERROR string) {
	var result map [string]interface{}
	// https://github.com/golang/go/wiki/InterfaceSlice
	// Slice of interfaces must be initialized. At least, with nil.
	var commands []interface{} = nil // Parse commands only after collecting defaults!
	def := Defaults{timeout: 60.0,
					max_reply: 16384,}

	cmd := &Commands{}

	err := json.Unmarshal(msg, &result)
	if err != nil {
		return nil, "Invalid json: " + err.Error()
	}

	for key, value := range result {
		switch key {
		case "producer":
			switch t := value.(type) {
			case string:
				cmd.producer = value.(string)
			case float64:
			   	cmd.producer = fmt.Sprintf("%f", value.(float64))
			default:
				return nil, fmt.Sprintf("\"producer\" field isn't [string], but [%T]", t)
			} // switch t
			if len(cmd.producer) < 1 {
				return nil, "Empty \"producer\" field!"
			}
		case "uuid":
			switch t := value.(type) {
			case string:
				cmd.uuid = value.(string)
			case float64:
			   	cmd.uuid = fmt.Sprintf("%f", value.(float64))
			default:
				return nil, fmt.Sprintf("\"uuid\" field isn't [string], but [%T]", t)
			} // switch t
			if len(cmd.uuid) < 8 { // M.b., no need in???
				return nil, "\"uuid\" has length < 8!"
			}
		case "error_fails":
			switch t := value.(type) {
			case float64:
				def.error_fails = (value.(float64) > 0)
			case bool:
				def.error_fails, _ = value.(bool)
			default:
				return nil, fmt.Sprintf("\"error_fails\" field isn't [bool|number], but [%T]", t)
			} // switch t
		case "no_exec":
			switch t := value.(type) {
			case float64:
				def.no_exec = (value.(float64) > 0)
			case bool:
				def.no_exec, _ = value.(bool)
			default:
				return nil, fmt.Sprintf("\"no_exec\" field isn't [bool|number], but [%T]", t)
			} // switch t
		case "run_as":
			switch t := value.(type) {
			case string:
				run_as := value.(string)
				cred := strings.Split(run_as, ":")
				// https://golang.org/pkg/os/user/#User
				user_, err := user.Lookup(cred[0])
				if err != nil {
					return nil, fmt.Sprintf("user invalid in \"run_as\": %s", cred[0])
				}
				u64, err := strconv.ParseUint(user_.Uid, 10, 32)
				if err != nil {
					return nil, fmt.Sprintf("non-integer uid! Is it linux?")
				}
				def.uid = uint32(u64)

				gid := ""
				if len(cred) > 1 {
					group, err := user.LookupGroup(cred[1])
					if err != nil {
						return nil, fmt.Sprintf("group invalid in \"run_as\": %s", cred[1])
					}
					gid = group.Gid
				} else {
					gid = user_.Gid
				}
				g64, err := strconv.ParseUint(gid, 10, 32)
				if err != nil {
					return nil, fmt.Sprintf("non-integer gid! Is it linux?")
				}
				def.gid = uint32(g64)

			default:
				return nil, fmt.Sprintf("\"run_as\" field isn't [string], but [%T]", t)
			} // switch t
		case "use_shell":
			switch t := value.(type) {
			case float64:
				def.use_shell = (value.(float64) > 0)
			case bool:
				def.use_shell, _ = value.(bool)
			default:
				return nil, fmt.Sprintf("\"use_shell\" field isn't [bool|number], but [%T]", t)
			} // switch t
		case "no_wait":
			switch t := value.(type) {
			case float64:
				def.no_wait = (value.(float64) > 0)
			case bool:
				def.no_wait, _ = value.(bool)
			default:
				return nil, fmt.Sprintf("\"no_wait\" field isn't [bool|number], but [%T]", t)
			} // switch t
		case "timeout":
			switch t := value.(type) {
			case float64:
				def.timeout = value.(float64)
				if def.timeout < 0 {
					return nil, "\"timeout\" less than 0!"
				}
			default:
				return nil, fmt.Sprintf("\"timeout\" field isn't [number], but [%T]", t)
			} // switch t
		case "max_reply":
			switch t := value.(type) {
			case float64:
				def.max_reply = int64(value.(float64))
				if def.max_reply < 0 {
					return nil, "\"max_reply\" less than 0!"
				}
			default:
				return nil, fmt.Sprintf("\"max_reply\" field isn't [number], but [%T]", t)
			} // switch t
		case "host_regex":
			switch t := value.(type) {
			case string:
				cmd.host_regex = value.(string)
			default:
				return nil, fmt.Sprintf("\"host_regex\" field isn't [string], but [%T]", t)
			} // switch t
		case "tag":
			switch t := value.(type) {
			case string:
				cmd.tag = value.(string)
			default:
				return nil, fmt.Sprintf("\"tag\" field isn't [string], but [%T]", t)
			} // switch t
		case "set_env":
			switch t := value.(type) {
			case string:
				def.set_env = append(def.set_env, value.(string))
			case []interface{}:
				for i, v := range t {
					switch vt := v.(type) {
					case string:
						def.set_env = append(def.set_env, v.(string))
//						fmt.Printf("env = %s\n", v.(string))
					default:
						return nil, fmt.Sprintf("\"set_env[%d]\" field isn't [string], but [%T]", i, vt)
					}
				}
			default:
				return nil, fmt.Sprintf("\"set_env\" field isn't [string], but [%T]", t)
			} // switch t
		case "set_dir":
			switch t := value.(type) {
			case string:
				def.set_dir = value.(string)
			default:
				return nil, fmt.Sprintf("\"set_dir\" field isn't [string], but [%T]", t)
			} // switch t

		case "commands":
			switch t := value.(type) {
			case []interface{}:
				commands = t
			default:
				return nil, fmt.Sprintf("\"commands\" field isn't [commands], but [%T]", t)
			} // switch t

		default:
			return nil, fmt.Sprintf("Unknown key in message: \"%s\"", key)
		} // switch key
	} // for key, value

	for i, v := range commands {
		command_, _ := json.Marshal(v)
		command, err := jsonCommand(command_, def, i)
		if err != "" {
			return nil, fmt.Sprintf("Error while parsing command #%d: %s", i, err)
		}
		cmd.commands = append(cmd.commands, *command)
	}

	if cmd.producer == "" {
		return nil, "Mandatory \"producer\" field not in json"
	}
	if cmd.uuid == "" {
		return nil, "Mandatory \"uuid\" field not in json"
	}

	return cmd, ""
}


/*
func zooWrite(path []string, value []byte) (ERROR string) {
	var exists bool;
	var _path string;

	if len(path) <1 {
		return "Path must have at least 1 element!"
	}

	c, _, err := zk.Connect(CONF.ZooKeeper.Cluster, time.Second * time.Duration(CONF.ZooKeeper.Timeout), zk.WithLogInfo(false))
	if err != nil {
		return fmt.Sprintf("ZOO error connecting cluster: %s", err.Error())
	}
	defer c.Close()

	for _, p := range path {
		_path = _path + "/" + p
		fmt.Println("Path =" + _path)
		exists, _, err = c.Exists(_path)
		if err != nil {
			return fmt.Sprintf("ZOO error checking path: %s", err.Error())
		}

		if !exists {
			_, err = c.Create(_path, []byte{}, int32(0), zk.WorldACL(zk.PermAll))
		}
		if err != nil {
			return fmt.Sprintf("ZOO error making path: %s", err.Error())
		}
		_, err = c.Set(_path, value, int32(-1)) // "-1" triggers version autoincrement
		if err != nil {
			return fmt.Sprintf("ZOO error setting value: %s", err.Error())
		}
	}
	return ""
}
*/


// Void commitOffset() must panic on error, avoiding inconsistency
func commitOffset(topic string, offset int64) () {
	OFFSETS[topic] = offset  // Update local offsets map. E.g. to check HookStart.

	if err := os.Rename(CONF.Consume.LocalDirectory + topic + ".offset", CONF.Consume.LocalDirectory + topic + ".offset.old"); err != nil {
		panic(fmt.Errorf("Commit error: Error renaming old offset to \"offset.old\": %s", err.Error()))
	}

	f, err := os.OpenFile(CONF.Consume.LocalDirectory + topic + ".offset", os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		panic(fmt.Errorf("Commit error: unable to open file \"%s\": %s", topic + ".offset", err.Error()))
	}
	defer f.Close()

	if _, err := f.WriteString(fmt.Sprintf("%d", offset)); err != nil {
		panic(fmt.Errorf("Commit error: unable to commit offset \"%s\": %s", topic + ".offset", err.Error()))
	} else {
		if err := f.Sync(); err != nil {
			panic(fmt.Errorf("Commit error: unable to sync offset \"%s\": %s", topic + ".offset", err.Error()))
		}
	}
	return
}


func consume(topics []string, master sarama.Consumer) (chan *sarama.ConsumerMessage, chan *sarama.ConsumerError) {
	consumers := make(chan *sarama.ConsumerMessage)
	errors := make(chan *sarama.ConsumerError)
	for _, topic := range topics {
		if strings.Contains(topic, "__consumer_offsets") {
			continue
		}
		partitions, _ := master.Partitions(topic)
		// this only consumes partition #1, you would probably want to consume all partitions
		commitOffset(topic, OFFSETS[topic])
		consumer, err := master.ConsumePartition(topic, CONF.Consume.Partition, OFFSETS[topic]) // sarama.OffsetOldest
		if err != nil {
			panic(fmt.Errorf("Consume error: unable to consume topic \"%s\"%s: %s", topic, partitions, err.Error()))
		}
		logNotice("Start consuming topic:", "\""+topic+"\"", "from offset:", OFFSETS[topic])

		go func(topic string, consumer sarama.PartitionConsumer) {
			for {
				select {
				case consumerError := <-consumer.Errors():
					errors <- consumerError
					logErr("consumerError: ", consumerError.Err)

				case msg := <-consumer.Messages():
					consumers <- msg
//					fmt.Println("Got message on topic ", topic, msg.Value)
				}
			}
		}(topic, consumer)
	}

	return consumers, errors
}


// Produce report of executed command
func produceExecReport(COMMANDS *Commands, RESULTS *ExecResults, message *sarama.ConsumerMessage, errorText string) {
	// https://golang.org/ref/spec#Exported_identifiers
	type Report struct {
		ClientID string       `json:"client_id"`
		MSG string            `json:"msg"` // message.Topic, message.Partition, message.Offset
		ERROR string          `json:"error,omitempty"`
		UUID string           `json:"uuid,omitempty"`
		TAG string            `json:"tag,omitempty"`
		PRODUCER string       `json:"producer,omitempty"`
		RESULTS []*ExecResult `json:"exec,omitempty"`
		TimeStamp string      `json:"timestamp,omitempty"`
	}
	t := time.Now()
	report := Report{ClientID: CONF.Kafka.ClientID,
					MSG: fmt.Sprintf("[%s](%d):%d", message.Topic, message.Partition, message.Offset),
					ERROR: errorText,
					// https://programming.guide/go/format-parse-string-time-date-example.html
					TimeStamp: t.Format("2006-01-02T15:04:05-0700"), // ISO 8601
					}
	if COMMANDS != nil {
		report.UUID = COMMANDS.uuid
		report.PRODUCER = COMMANDS.producer
		report.TAG = COMMANDS.tag
		if RESULTS != nil {
			report.RESULTS = RESULTS.Results
		}
	}

	msg := &sarama.ProducerMessage{
		Topic: CONF.Produce.Topic,
	}

	report_, err := json.MarshalIndent(report, "", "\t")
	if err != nil {
		panic(fmt.Errorf("Produce error: unable to marshal json: %s", err.Error()))
	}
	msg.Value = sarama.StringEncoder(report_)

	now := time.Now()
	if now.Sub(PRODUCER_AGE) >= time.Duration(CONF.Kafka.KeepAlive) * time.Second { // "write: broken pipe" on producer, keepAlive does not works
		logInfo("Respawing producer...")
		go func (p sarama.SyncProducer) {
			if err := p.Close(); err != nil {
				// Should not reach here
				panic(fmt.Errorf("Shutdown error: unable to close producer: %s", err.Error()))
			}
		} (PRODUCER)
		PRODUCER, err = sarama.NewSyncProducer(CONF.Kafka.Brokers, config)
		if err != nil {
			panic(fmt.Errorf("Init error: unable to start producer: %s", err.Error()))
		}
	}

	partition, offset, err := PRODUCER.SendMessage(msg)
	PRODUCER_AGE = time.Now()
	if err != nil {
		// Try to leave local copy of last report before aborting
		report_file := fmt.Sprintf("%s/%s:%d:%d.crash", CONF.Consume.LocalDirectory, message.Topic,  message.Partition, message.Offset)
		if f, err := os.OpenFile(report_file, os.O_RDWR|os.O_CREATE, 0600); err == nil {
			defer f.Close()
			f.Write(report_)
		}
		panic(fmt.Errorf("Produce error: unable to send report: %s", err.Error()))
	} else {
		logInfo(fmt.Sprintf("Report is stored in topic(%s)/partition(%d)/offset(%d)", msg.Topic, partition, offset))
	}

	if len(CONF.Produce.LocalDirectory) > 0 {
		report_path := fmt.Sprintf("%s/%s/%d/", CONF.Produce.LocalDirectory, msg.Topic, partition)
		if err = os.MkdirAll(report_path, 0700); err != nil {
			panic(fmt.Errorf("Produce error: unable to create local report path \"%s\": %s", report_path, err.Error()))
		}

		report_file := fmt.Sprintf("%s/%d", report_path, offset)
		f, err := os.OpenFile(report_file, os.O_RDWR|os.O_CREATE, 0600)
	 	if err != nil {
 			panic(fmt.Errorf("Produce error: unable to open report file \"%s\": %s", report_file, err.Error()))
	 	}

	 	defer f.Close()

	 	if _, err := f.Write(report_); err != nil {
 			panic(fmt.Errorf("Produce error: unable to write report \"%s\": %s", report_file, err.Error()))
	 	}
	}
	return
}

// Void ProcessMessage() for messages received from kafka
func processMessage(msg *sarama.ConsumerMessage) { // https://godoc.org/github.com/Shopify/sarama#ConsumerMessage
	fqdn := fqdn.Get()
	defer commitOffset(msg.Topic, msg.Offset + 1) // Offset must be incremented only after processMessage()

	RESULTS := &ExecResults{}
	COMMANDS, ERR := jsonMessage(msg.Value)
	if ERR != "" { // Bad message format
//		fmt.Println("Error in message: ", msg.Offset, ERR)
		produceExecReport(COMMANDS, RESULTS, msg, "Error parsing json: " + ERR)
		return
	}
	if COMMANDS.host_regex != "" { // Execute only those are matching hostname
		matched, err := regexp.MatchString(COMMANDS.host_regex, fqdn)
		if err != nil { // Invalid regexp syntax
//!!!			produceCommandError("Invalid regexp", COMMANDS)
			produceExecReport(COMMANDS, RESULTS, msg, "Invalid regexp: " + COMMANDS.host_regex)
			return
		}
		if !matched { // This command is not intended for this host
			return
		}
	}

	defer produceExecReport(COMMANDS, RESULTS, msg, "")

	for i, _ := range COMMANDS.commands {
		if COMMANDS.commands[i].host_regex != "" { // Execute only those are matching hostname
				matched, err := regexp.MatchString(COMMANDS.commands[i].host_regex, fqdn)
			if err != nil { // Invalid regexp syntax
				continue
			}
			if !matched { // This command is not intended for this host
				continue
			}
		}

		execResult := execCommand(&COMMANDS.commands[i])
		RESULTS.Results = append(RESULTS.Results, execResult)

		if execResult.Status != 0 && COMMANDS.commands[i].error_fails {
			return
		}
	}

	return
}


// Process hooks
// Coommon executer
func doHook(hook string, id string) {
	cmd := &Command {
		id: "hook",
		use_shell: true,
		timeout: 900,
		max_reply: 16384,
		command: hook,
	}
	execResult_ := execCommand(cmd)

	if r := recover(); r != nil { // Application was already crashed.
		panic(r) // Just thraw panic, don't try to produce kafka log.
	}

	if CONF.Hooks.Produce {
		msg := &sarama.ConsumerMessage{
								Topic: id,
								Partition: 0,
								Offset: 0,
								}
		RESULTS := &ExecResults{Results: []*ExecResult{execResult_}}
		COMMANDS := &Commands{
						producer: CONF.Kafka.ClientID,
						uuid: id,
						tag: CONF.Hooks.Tag,
						commands: []Command{*cmd},
						}
		err := ""
		if exitSignal != nil {
			err = fmt.Sprintf("Exiting on signal: %s", exitSignal)
		}
		logNotice("CONF.Hooks.Produce: sending report...")
		produceExecReport(COMMANDS, RESULTS, msg, err)
	}

	return
}

// Hooks.Start
// Must be executed after client state actualizing:
//  journal positions must be rolled to those which where commited on start of consuming.
func doHookStart() {
// When journal is empty, (OFFSETS_ON_START[topic] == OFFSETS_ON_START_OLDEST[topic])
	for topic, _ := range OFFSETS_ON_START {
		logDebug(topic, ":", OFFSETS[topic], OFFSETS_ON_START[topic])
		if (OFFSETS[topic] == sarama.OffsetOldest) && (OFFSETS_ON_START[topic] > OFFSETS_ON_START_OLDEST[topic]) {
			continue
		}

		if (OFFSETS[topic] >= OFFSETS_ON_START[topic]) || (OFFSETS[topic] == sarama.OffsetNewest) {
			delete(OFFSETS_ON_START, topic)
		}
		if (OFFSETS[topic] == sarama.OffsetOldest) && (OFFSETS_ON_START[topic] == OFFSETS_ON_START_OLDEST[topic]) {
			delete(OFFSETS_ON_START, topic)
		}
	}
	if len(OFFSETS_ON_START) > 0 { // Not all journal positions are actualized yet
		return
	}

	logWarning("CONF.Hooks.Start: launching hook:", CONF.Hooks.Start)
	hookStart = false

	doHook(CONF.Hooks.Start, "Local.Hooks.Start")
	logNotice("CONF.Hooks.Start: done")
	return
}

// Hooks.Stop
func doHookStop() {
	logWarning("CONF.Hooks.Stop: launching hook:", CONF.Hooks.Stop)
	hookStop = false

	doHook(CONF.Hooks.Stop, "Local.Hooks.Stop")
	logNotice("CONF.Hooks.Stop: done")
	return
}


// Parse given config. Panic on errors: config MUST be clear for such a daemon.
// !!! "--test" CLI key must be realized to check syntax before daemon reloading.
func parseConfigFile() (*Config) {
	config_path_ := "/etc/execd/execd.conf"
	config_path := &config_path_

	conf := &Config{}
	// Defaults
	conf.ZooKeeper.Timeout = 3
	conf.Kafka.DialTimeout = 10
	conf.Kafka.ReadTimeout = 10
	conf.Kafka.WriteTimeout = 10
	conf.Consume.LocalDirectory = "/var/lib/execd"
	conf.Consume.FetchMax = 1048576 // 1MB is large enough. M.b. up to 256 MB messages due to hardcoded ChannelBufferSize!
	conf.Consume.RetryBackoff = 10
	conf.Consume.RetryMax = 8640  // !!! 10^4 takes about 1MB of RSS, because sarama fills some linear structure to do !!!
	conf.Produce.MaxMessageBytes = 16777216
	conf.Produce.Timeout = 30
	conf.Produce.RetryBackoff = 10
	conf.Produce.RetryMax = 8640 // !!! 10^4 takes about 1MB of RSS, because sarama fills some linear structure to do !!!


	if env_config, ok := os.LookupEnv("CONFIG"); ok {
		*config_path = env_config
	}

	F := flag.NewFlagSet("", flag.ContinueOnError)
	config_path = F.StringP("conf", "c", *config_path, "Non-default config location")
	DEBUG = F.BoolP("debug", "d", *DEBUG, "Debug log level (although switches output to stdout|stderr)")
	verbose := F.BoolP("verbose", "v", false, "Increase log level to NOTICE")
	F.StringP("uninitialized.topic.name", "", "", "Note, that \"--topic=offset\" keys have to be provided only once, at initialization phase")
	if err := F.Parse(os.Args[1:]); err != nil {
		logErr(err.Error())
	}
	if *DEBUG {
		LOG_LEVEL = syslog.LOG_DEBUG
	} else {
		if *verbose {
			LOG_LEVEL = syslog.LOG_NOTICE
		}
	}
	F.Init("", flag.ExitOnError)

	conf_file, err := os.Open(*config_path)
	if err != nil {
		panic(fmt.Errorf("Config error: unable to open config file: %s", err.Error()))
	}
	defer conf_file.Close()

	conf_, err := ioutil.ReadAll(conf_file)
	if err != nil {
		panic(fmt.Errorf("Config error: unable to read config file: %s", err.Error()))
	}

	if err := toml.Unmarshal(conf_, &conf); err != nil {
		panic(fmt.Errorf("Config error: unable to parse config file: %s", err.Error()))
	}

	if conf.Kafka.ClientID == "" {
		conf.Kafka.ClientID = fqdn.Get()
	}

	if len(conf.Kafka.CAcertFile) > 0 {
		CAcert, err = ioutil.ReadFile(conf.Kafka.CAcertFile)
		if err != nil {
			panic(fmt.Errorf("Config error: unable to read Kafka.CAcertFile: %s", err.Error()))
		}
		keyBlock, _ := pem.Decode(CAcert)
		if keyBlock == nil {
			panic(fmt.Errorf("Config error: invalid PEM inside Kafka.CAcertFile = \"%s\"", conf.Kafka.CAcertFile))
		}
	}

	if len(conf.Kafka.PrivateKeyFile) > 0 {
		privateKey, err = ioutil.ReadFile(conf.Kafka.PrivateKeyFile)
		if err != nil {
			panic(fmt.Errorf("Config error: unable to read Kafka.PrivateKeyFile: %s", err.Error()))
		}
		// https://stackoverflow.com/a/56131169
		keyBlock, _ := pem.Decode(privateKey)
		if keyBlock == nil {
			panic(fmt.Errorf("Config error: invalid PEM inside Kafka.PrivateKeyFile = \"%s\"", conf.Kafka.PrivateKeyFile))
		}
		switch keyBlock.Type {
		case "ENCRYPTED PRIVATE KEY":
/* openssl default is PKCS8 pbeWITHMD5ndDES-CBC, while https://godoc.org/github.com/youmark/pkcs8 has AES-256-CBC only
			if ! x509.IsEncryptedPEMBlock(keyBlock) {
			// Decrypt key
				keyDER, err := x509.DecryptPEMBlock(keyBlock, []byte(conf.Kafka.PrivateKeyPassword + "xxx"))
				if err != nil {
					panic(fmt.Errorf("Config error: unable to decrypt key from Kafka.PrivateKeyFile: %s", err.Error()))
				}
				// Update keyBlock with the plaintext bytes and clear the now obsolete headers.
				keyBlock.Bytes = keyDER
				keyBlock.Headers = nil
			panic(fmt.Errorf("XXX Config error: invalid key type stored in Kafka.PrivateKeyFile: \"%s\"", keyBlock.Type))
			}
			privateKey = pem.EncodeToMemory(keyBlock)
*/
			panic(fmt.Errorf("Config error: ENCRYPTED PRIVATE KEY is unsupported now. Kafka.PrivateKeyFile: \"%s\"", keyBlock.Type))
		case "PRIVATE KEY":
			privateKey = pem.EncodeToMemory(keyBlock)
		default:
			panic(fmt.Errorf("Config error: invalid key type stored in Kafka.PrivateKeyFile: \"%s\"", keyBlock.Type))
		}

		if len(conf.Kafka.CertFile) > 0 {
			cert, err = ioutil.ReadFile(conf.Kafka.CertFile)
			if err != nil {
				panic(fmt.Errorf("Config error: unable to read Kafka.CertFile: %s", err.Error()))
			}
			keyBlock, _ := pem.Decode(cert)
			if keyBlock == nil {
				panic(fmt.Errorf("Config error: invalid PEM inside Kafka.CertFile = \"%s\"", conf.Kafka.CertFile))
			}
		} else {
			panic(fmt.Errorf("Config error: Kafka.CertFile must be provided in couple with Kafka.PrivateKeyFile"))
		}
	}

	// Test Consume.LocalDirectory is available
	if len(conf.Consume.LocalDirectory) < 1 {
		panic(fmt.Errorf("Config error: Consume.LocalDirectory must be set"))
	}
	if err = os.MkdirAll(conf.Consume.LocalDirectory, 0700); err != nil {
		panic(fmt.Errorf("Config error: unable to create Consume.LocalDirectory: %s", err.Error()))
	}
	if conf.Consume.LocalDirectory[len(conf.Consume.LocalDirectory)-1 :] != "/" {
		conf.Consume.LocalDirectory = conf.Consume.LocalDirectory + "/"
	}

	// Test Produce.LocalDirectory is available, if set
	if len(conf.Produce.LocalDirectory) > 0 {
		if err = os.MkdirAll(conf.Produce.LocalDirectory, 0700); err != nil {
			panic(fmt.Errorf("Config error: unable to create Produce.LocalDirectory: %s", err.Error()))
		}
		if conf.Produce.LocalDirectory[len(conf.Produce.LocalDirectory)-1 :] != "/" {
			conf.Produce.LocalDirectory = conf.Produce.LocalDirectory + "/"
		}
	}


	cli_offsets := make(map[string]*string)
	if len(conf.Consume.Topics) > 0 {
		// Test whether local path is writable
		for _, t := range conf.Consume.Topics {
			f, err := os.OpenFile(conf.Consume.LocalDirectory + t + ".offset", os.O_RDWR|os.O_CREATE, 0600)
			defer f.Close()
			if err != nil {
				panic(fmt.Errorf("Config error: unable to create/open offset \"%s\": %s", t + ".offset", err.Error()))
			}
			if offset, err := ioutil.ReadAll(f); err != nil {
				panic(fmt.Errorf("Config error: unable to read offset \"%s\": %s", t + ".offset", err.Error()))
			} else {
				if len(offset) == 0 {
					cli_offsets[t] = F.String(t, "", fmt.Sprintf("Topic \"%s\" has no stored offset", t))
				} else {
					if OFFSETS[t], err = strconv.ParseInt(fmt.Sprintf("%s", offset), 10, 64); err != nil {
						panic(fmt.Errorf("Config error: non-integer offset for topic \"%s\": %s", t, offset))
					}
				}
			}
		}
	} else {
		panic(fmt.Errorf("Config error: at least one topic must be configured in \"Consume.Topics\"!"))
	}

//	flag.Parse()
	F.Parse(os.Args[1:])
// Empty offsets must be initialized from CLI
	for key, value := range cli_offsets {
		if *value == "" {
			panic(fmt.Errorf("Config error: empty offset for topic \"%s\"", key))
		} else {
			if OFFSETS[key], err = strconv.ParseInt(*value, 10, 64); err != nil {
				panic(fmt.Errorf("Config error: non-integer offset for topic \"%s\"", key))
			}
		}
	}

	for key, value := range OFFSETS {
		logInfo("Config: Offset for topic:", key, "=", value)
	}

	if len(conf.Hooks.Start) > 0 {
		hookStart = true
	}

	if len(conf.Hooks.Stop) > 0 {
		hookStop = true
	}

	return conf
}


// Interprocess mutex by means of "github.com/juju/mutex"
// In fact,is syscall.flock() wrapper
type fakeClock struct {
	delay time.Duration
}
func (f *fakeClock) After(time.Duration) <-chan time.Time {
	return time.After(f.delay)
}

func (f *fakeClock) Now() time.Time {
	return time.Now()
}
func mutEx () {

	var err = errors.New("")
	hash := fmt.Sprintf("%x", sha1.Sum([]byte(os.Args[0])))
	hash = "X" + hash[1:]
	logDebug("Init: acquiring global IPC mutex. ID = " + hash)

	spec := mutex.Spec {Name: hash,
						Clock: &fakeClock{time.Millisecond},
						Delay: time.Millisecond,
						Timeout: time.Second,
						}
	if MUTEX, err = mutex.Acquire(spec); err != nil {
		panic(fmt.Errorf("Mutex error: another instance is already running: %s", err.Error()))
	}
	return
}


func main() {
	FALSE := false
	DEBUG = &FALSE

	err := errors.New("")

	// Initialize logging pathes
	SYSLOG, err = syslog.New(syslog.LOG_DEBUG | syslog.LOG_DAEMON, "execd") // M.b. NULL pointer, in case of some error
	STDOUT = log.New(os.Stdout, "", log.LstdFlags)
	STDERR = log.New(os.Stderr, "", log.LstdFlags)

	defer func() { // Report panic, if one occured
		 if r := recover(); r != nil {
		 	logEmerg(r)
		 }
	}()

   	if env_log, ok := os.LookupEnv("LOG"); ok {
		switch strings.ToUpper(env_log) {
			case "DEBUG":
				LOG_LEVEL = syslog.LOG_DEBUG
				d := true
				DEBUG = &d
			case "INFO":
				LOG_LEVEL = syslog.LOG_INFO
			case "NOTICE":
				LOG_LEVEL = syslog.LOG_NOTICE
			case "ERR":
				LOG_LEVEL = syslog.LOG_ERR
			case "CRIT":
				LOG_LEVEL = syslog.LOG_CRIT
			default:
				LOG_LEVEL = syslog.LOG_WARNING
		}
	} else {
		LOG_LEVEL = syslog.LOG_WARNING
	}

// Global mutex
	mutEx()
	defer MUTEX.Release()
// Config
	CONF = parseConfigFile()
/*/////////////////////////////////////////////////////////
// zookeeper test
	m := []byte("test\nvalue")
	zooWrite([]string{"knot1","test"}, m)
  /////////////////////////////////////////////////////////
*/

	// KAFKA
	if *DEBUG { // Log sarama output
		l := log.New(os.Stdout, "kafka: ", log.LstdFlags)
		sarama.Logger = l
	} else {
		if (LOG_LEVEL >= syslog.LOG_INFO) && (SYSLOG != nil) {
			if sl, err := syslog.New(syslog.LOG_INFO | syslog.LOG_DAEMON, "execd"); err == nil {
				l := log.New(sl, "kafka: ", log.LstdFlags)
				sarama.Logger = l
			}
		}
	}

	config = sarama.NewConfig()
	config.Version = sarama.V2_2_0_0 // https://github.com/Shopify/sarama/blob/master/utils.go
	config.ClientID = CONF.Kafka.ClientID
	config.Consumer.Return.Errors = true
	if CONF.Consume.FetchMax > 0 {
		config.Consumer.Fetch.Max = CONF.Consume.FetchMax
	}
	config.Consumer.IsolationLevel = sarama.ReadCommitted
	if CONF.Consume.RetryBackoff > 0 {
		config.Consumer.Retry.Backoff = time.Second * time.Duration(CONF.Consume.RetryBackoff)
		config.Metadata.Retry.Backoff = time.Second * time.Duration(CONF.Consume.RetryBackoff)
	}
//	config.Consumer.Retry.Max = 86400 >> Metadata  // 86400 == 1 day

	if CONF.Consume.RetryMax > 0 {
		config.Metadata.Retry.Max = CONF.Consume.RetryMax
	}
// Available in upcoming release.
//	config.Consumer.ChannelBufferSize = 2 // No need in speedup
// https://github.com/Shopify/sarama/blob/master/config.go
	if CONF.Kafka.DialTimeout > 0 {
		config.Net.DialTimeout = time.Second * time.Duration(CONF.Kafka.DialTimeout)
	}
	if CONF.Kafka.ReadTimeout > 0 {
		config.Net.ReadTimeout = time.Second * time.Duration(CONF.Kafka.ReadTimeout)
	}
	if CONF.Kafka.WriteTimeout > 0 {
		config.Net.WriteTimeout = time.Second * time.Duration(CONF.Kafka.WriteTimeout)
	}
	if CONF.Kafka.KeepAlive > 0 {
		config.Net.KeepAlive = time.Second * time.Duration(CONF.Kafka.KeepAlive)
	}
	if len(CONF.Kafka.LocalAddr) > 0 {
		a := net.ParseIP(CONF.Kafka.LocalAddr)
		if a == nil {
			panic(fmt.Errorf("Init error: invalid ip in CONF.Kafka.LocalAddr: %s", CONF.Kafka.LocalAddr))
		}
		addr := &net.TCPAddr{a, 0, ""}
		config.Net.LocalAddr = addr
	}
	config.Net.MaxOpenRequests = 1
// panic: kafka: client has run out of available brokers to talk to (Is your cluster reachable?)
	config.Net.TLS.Enable = true
	config.Net.TLS.Config = &tls.Config{InsecureSkipVerify: CONF.Kafka.InsecureSkipVerify}
	if len(CAcert) > 0 { // Import provided root CA
		caCertPool := x509.NewCertPool()
		if caCertPool.AppendCertsFromPEM(CAcert) {
			config.Net.TLS.Config.RootCAs = caCertPool
		} else {
			panic(fmt.Errorf("Init error: invalid CAcert in Kafka.CAcertFile \"%s\"", CONF.Kafka.CAcertFile))
		}
	}
	if len(privateKey) > 0 { // Import provided client key pair
		crt, err := tls.X509KeyPair(cert, privateKey)
		if err != nil {
			panic(fmt.Errorf("Init error: invalid key pair provided in Kafka.(privateKeyFile|certFile): %s", err.Error()))
		}
		config.Net.TLS.Config.Certificates = []tls.Certificate{crt}
	}

	if CONF.Produce.MaxMessageBytes > 0 {
		config.Producer.MaxMessageBytes = CONF.Produce.MaxMessageBytes
	}
// The maximum duration the broker will wait the receipt of the number of RequiredAcks (defaults to 10 seconds).
// This is only relevant when RequiredAcks is set to WaitForAll or a number > 1
	if CONF.Produce.Timeout > 0 {
		config.Producer.Timeout = time.Second * time.Duration(CONF.Produce.Timeout)
	}
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Idempotent = true
	config.Producer.Return.Successes = true
	if CONF.Produce.RetryBackoff > 0 {
		config.Producer.Retry.Backoff = time.Second * time.Duration(CONF.Produce.RetryBackoff)
	}
	if CONF.Produce.RetryMax > 0 {
		config.Producer.Retry.Max = CONF.Produce.RetryMax
	}

	// First, Create new client, to query for current offsets
	logInfo("Init: launching kafka auxiliary client...")
	client, err = sarama.NewClient(CONF.Kafka.Brokers, config)
	if err != nil {
		panic(fmt.Errorf("Init error: unable to start client: %s", err.Error()))
	}

	defer func() {
		logInfo("Shutdown: closing auxiliary client...")
		if err := client.Close(); err != nil {
			panic(fmt.Errorf("Shutdown error: unable to close client: %s", err.Error()))
		}
	}()

	// Check configured topics against those in kafka
	logInfo("Init: testing kafka topics...")
	conf_topics := make(map[string]int)
	for _, t := range CONF.Consume.Topics {
		conf_topics[t] = 1
	}
	topics, _ := client.Topics() // Now, check available topics against configured
	for _, topic := range topics {
		if strings.Contains(topic, "__consumer_offsets") {
			continue
		}
		for t, _ := range conf_topics {
			if t == topic {
				if o, err := client.GetOffset(topic, CONF.Consume.Partition, sarama.OffsetNewest); err != nil {
					panic(fmt.Errorf("Init error: unable to query last offset for topic \"%s\": %s", topic, err.Error()))
				} else {
					OFFSETS_ON_START[topic] = o
				}
				if o, err := client.GetOffset(topic, CONF.Consume.Partition, sarama.OffsetOldest); err != nil {
					panic(fmt.Errorf("Init error: unable to query first offset for topic \"%s\": %s", topic, err.Error()))
				} else {
					OFFSETS_ON_START_OLDEST[topic] = o
				}
				delete(conf_topics, t)
				if len(conf_topics) < 1 {
					break
				}
				continue
			}
		}
	}
	logInfo("Current offsets:", OFFSETS_ON_START)

	if len(conf_topics) > 0 {
		logCrit("!!! The following topics are unknown in kafka:")
		for t, _ := range conf_topics {
			logCrit(t)
		}
		panic(fmt.Errorf("Init error: Error found in topics config! Aborting."))
	}

	// Create new producer. It is single, *GLOBAL* object
	logInfo("Init: launching kafka producer...")
	PRODUCER, err = sarama.NewSyncProducerFromClient(client)
	PRODUCER_AGE = time.Now()
	if err != nil {
		panic(fmt.Errorf("Init error: unable to start producer: %s", err.Error()))
	}
	defer func() {
		logInfo("Shutdown: closing producer...")
		if err := PRODUCER.Close(); err != nil {
			// Should not reach here
			panic(fmt.Errorf("Shutdown error: unable to close producer: %s", err.Error()))
		}
	}()

	// Create new consumer
	logInfo("Init: launching kafka consumer...")
	master, err := sarama.NewConsumerFromClient(client)
	if err != nil {
		panic(fmt.Errorf("Init error: unable to start consumer: %s", err.Error()))
	}

	defer func() {
		logInfo("Shutdown: closing consumer...")
		if err := master.Close(); err != nil {
			panic(fmt.Errorf("Shutdown error: unable to close consumer: %s", err.Error()))
		}
	}()

	logInfo("Init: launching consume threads...")
	consumer, errors := consume(CONF.Consume.Topics, master)

	// https://golang.org/pkg/syscall
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)

	// Count how many message processed
	msgCount := 0

	// Try to execute start hook. Will be processed only in case all topic positions are actual.
	if hookStart {
		doHookStart()
	}
	// Stop hook must be exeuted at the beginning of destruction, while kafka producer is still alive.
	// But what if panic was already triggered?
	if hookStop {
		defer doHookStop()
	}

	// Get signal for finish
	doneCh := make(chan struct{})
	go func() {
		for {
			select {
			case msg := <-consumer:
				msgCount++
				logDebug("Received messages", string(msg.Key), string(msg.Value), msg.Offset, msg.Partition)
				processMessage(msg)
				// Try to exec hook, "hookStart" will be invalidated after execution.
				if hookStart {
					doHookStart()
				}
				debug.FreeOSMemory()
			case consumerError := <-errors:
				msgCount++
				logErr("Received consumerError ", string(consumerError.Topic), string(consumerError.Partition), consumerError.Err)
//				doneCh <- struct{}{}
			case exitSignal = <-signals:
				logWarning("Interrupt is detected:", exitSignal)
				doneCh <- struct{}{}
			}
		}
	}()
	logWarning("Init done. Ready to serve.")
	debug.FreeOSMemory()

	<-doneCh
	logInfo("Processed", msgCount, "messages")

}
