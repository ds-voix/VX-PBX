package xlog
// Extended logging

import (
	"fmt"
    "log"
	"log/syslog"
	"os"
)


type XLog struct {
	// Syslog logger
	SYSLOG *syslog.Writer
	STDOUT *log.Logger
	STDERR *log.Logger

	DEBUG *bool
	LOG_LEVEL syslog.Priority // LOG_EMERG=0 .. LOG_DEBUG=7
}

const (
	// https://unix.superglobalmegacorp.com/Net2/newsrc/sys/syslog.h.html
	LOG_EMERG	= 0	/* system is unusable */
	LOG_ALERT	= 1	/* action must be taken immediately */
	LOG_CRIT	= 2	/* critical conditions */
	LOG_ERR		= 3	/* error conditions */
	LOG_WARNING	= 4	/* warning conditions */
	LOG_NOTICE	= 5	/* normal but significant condition */
	LOG_INFO	= 6	/* informational */
	LOG_DEBUG	= 7	/* debug-level messages */
)


func New(daemon_name string) (XLog) {
    var XLOG XLog
    Debug := false
	// Initialize logging pathes
	XLOG.SYSLOG, _ = syslog.New(syslog.LOG_DEBUG | syslog.LOG_DAEMON, daemon_name) // M.b. NULL pointer, in case of some error
	XLOG.STDOUT = log.New(os.Stdout, "", log.LstdFlags)
	XLOG.STDERR = log.New(os.Stderr, "", log.LstdFlags)
	XLOG.DEBUG = &Debug
	return XLOG
}


// void: Log to syslog/stdout/stderr, depending on settings
func (xl XLog) Log(severity syslog.Priority, message_ ...interface{}) {
    if severity > xl.LOG_LEVEL { return }
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

	if (xl.DEBUG != nil && *xl.DEBUG) || (xl.SYSLOG == nil) {
		if severity > syslog.LOG_WARNING {
			xl.STDERR.Printf("%s: %s", level, message)
		} else {
			xl.STDOUT.Printf("%s: %s", level, message)
		}
	} else {
		switch severity { // Double work, due to the absence of "syslog.Log(string, severity)"
			case syslog.LOG_EMERG:
				err = xl.SYSLOG.Emerg(message)
			case syslog.LOG_ALERT:
				err = xl.SYSLOG.Alert(message)
			case syslog.LOG_CRIT:
				err = xl.SYSLOG.Crit(message)
			case syslog.LOG_ERR:
				err = xl.SYSLOG.Err(message)
			case syslog.LOG_WARNING:
				err = xl.SYSLOG.Warning(message)
			case syslog.LOG_NOTICE:
				err = xl.SYSLOG.Notice(message)
			case syslog.LOG_INFO:
				err = xl.SYSLOG.Info(message)
			default:
				err = xl.SYSLOG.Debug(message)
		}
		if err != nil {
			xl.STDERR.Printf("SYSLOG failed \"%s\" to write %s. Message was: \"%s\"", err.Error(), level, message)
		}
	}
}

// Unified log implementation. Less code >> more CPU.
func (xl XLog) Debug (message ...interface{}) { xl.Log(syslog.LOG_DEBUG, message) }
func (xl XLog) Info (message ...interface{}) { xl.Log(syslog.LOG_INFO, message) }
func (xl XLog) Notice (message ...interface{}) { xl.Log(syslog.LOG_NOTICE, message) }
//
func (xl XLog) Warning (message ...interface{}) { xl.Log(syslog.LOG_WARNING, message) }
func (xl XLog) Warn (message ...interface{}) { xl.Log(syslog.LOG_WARNING, message) }
//
func (xl XLog) Err (message ...interface{}) { xl.Log(syslog.LOG_ERR, message) }
func (xl XLog) Error (message ...interface{}) { xl.Log(syslog.LOG_ERR, message) }
//
func (xl XLog) Crit (message ...interface{}) { xl.Log(syslog.LOG_CRIT, message) }
func (xl XLog) Critical (message ...interface{}) { xl.Log(syslog.LOG_CRIT, message) }
//
func (xl XLog) Alert (message ...interface{}) { xl.Log(syslog.LOG_ALERT, message) }
//
func (xl XLog) Emerg (message ...interface{}) { xl.Log(syslog.LOG_EMERG, message) }
func (xl XLog) Fatal (message ...interface{}) { xl.Log(syslog.LOG_EMERG, message) }
