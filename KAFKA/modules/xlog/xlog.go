package xlog
// Extended logging

import (
	"fmt"
    "log"
	"log/syslog"
	"os"
)

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
var (
	// Syslog logger
	SYSLOG *syslog.Writer
	STDOUT *log.Logger
	STDERR *log.Logger

    DEBUG *bool
    LOG_LEVEL syslog.Priority // LOG_EMERG=0 .. LOG_DEBUG=7
)


func New(daemon_name string) {
	// Initialize logging pathes
	SYSLOG, _ = syslog.New(syslog.LOG_DEBUG | syslog.LOG_DAEMON, daemon_name) // M.b. NULL pointer, in case of some error
	STDOUT = log.New(os.Stdout, "", log.LstdFlags)
	STDERR = log.New(os.Stderr, "", log.LstdFlags)
}


// void: Log to syslog/stdout/stderr, depending on settings
func Log(severity syslog.Priority, message_ ...interface{}) {
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
func Debug (message ...interface{}) { Log(syslog.LOG_DEBUG, message) }
func Info (message ...interface{}) { Log(syslog.LOG_INFO, message) }
func Notice (message ...interface{}) { Log(syslog.LOG_NOTICE, message) }
//
func Warning (message ...interface{}) { Log(syslog.LOG_WARNING, message) }
func Warn (message ...interface{}) { Log(syslog.LOG_WARNING, message) }
//
func Err (message ...interface{}) { Log(syslog.LOG_ERR, message) }
func Error (message ...interface{}) { Log(syslog.LOG_ERR, message) }
//
func Crit (message ...interface{}) { Log(syslog.LOG_CRIT, message) }
func Critical (message ...interface{}) { Log(syslog.LOG_CRIT, message) }
//
func Alert (message ...interface{}) { Log(syslog.LOG_ALERT, message) }
//
func Emerg (message ...interface{}) { Log(syslog.LOG_EMERG, message) }
func Fatal (message ...interface{}) { Log(syslog.LOG_EMERG, message) }
