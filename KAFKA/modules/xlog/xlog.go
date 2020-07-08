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
	XLOG.SYSLOG, _ = syslog.New(syslog.LOG_WARNING | syslog.LOG_DAEMON, daemon_name) // M.b. NULL pointer, in case of some error
	XLOG.STDOUT = log.New(os.Stdout, "", log.LstdFlags)
	XLOG.STDERR = log.New(os.Stderr, "", log.LstdFlags)
	XLOG.DEBUG = &Debug
	return XLOG
}


// void: Log to syslog/stdout/stderr, depending on settings
func (xl XLog) Log(severity syslog.Priority, message string) {
    if severity > xl.LOG_LEVEL { return }

    var err error
	level := "debug"
	switch severity {
		case syslog.LOG_EMERG:
			level = "emerg"
		case syslog.LOG_ALERT:
			level = "alert"
		case syslog.LOG_CRIT:
			level = "crit"
		case syslog.LOG_ERR:
			level = "err"
		case syslog.LOG_WARNING:
			level = "warn"
		case syslog.LOG_NOTICE:
			level = "notice"
		case syslog.LOG_INFO:
			level = "info"
	}

	if (xl.DEBUG != nil && *xl.DEBUG) || (xl.SYSLOG == nil) {
		if severity > syslog.LOG_WARNING {
			xl.STDERR.Printf("%s: %s", level, message)
		} else {
			xl.STDOUT.Printf("%s: %s", level, message)
		}
	} else {
	    message = level + ": " + message
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
func (xl XLog) Debug (message string) { xl.Log(syslog.LOG_DEBUG, message) }
func (xl XLog) Debugf (message string, values ...interface{}) { xl.Log(syslog.LOG_DEBUG, fmt.Sprintf(message, values...)) }

func (xl XLog) Info (message string) { xl.Log(syslog.LOG_INFO, message) }
func (xl XLog) Infof (message string, values ...interface{}) { xl.Log(syslog.LOG_INFO, fmt.Sprintf(message, values...)) }

func (xl XLog) Notice (message string) { xl.Log(syslog.LOG_NOTICE, message) }
func (xl XLog) Noticef (message string, values ...interface{}) { xl.Log(syslog.LOG_NOTICE, fmt.Sprintf(message, values...)) }
//
func (xl XLog) Warning (message string) { xl.Log(syslog.LOG_WARNING, message) }
func (xl XLog) Warn (message string) { xl.Log(syslog.LOG_WARNING, message) }
func (xl XLog) Warnf (message string, values ...interface{}) { xl.Log(syslog.LOG_WARNING, fmt.Sprintf(message, values...)) }
//
func (xl XLog) Err (message string) { xl.Log(syslog.LOG_ERR, message) }
func (xl XLog) Error (message string) { xl.Log(syslog.LOG_ERR, message) }
func (xl XLog) Errf (message string, values ...interface{}) { xl.Log(syslog.LOG_ERR, fmt.Sprintf(message, values...)) }
//
func (xl XLog) Crit (message string) { xl.Log(syslog.LOG_CRIT, message) }
func (xl XLog) Critical (message string) { xl.Log(syslog.LOG_CRIT, message) }
func (xl XLog) Critf (message string, values ...interface{}) { xl.Log(syslog.LOG_CRIT, fmt.Sprintf(message, values...)) }
//
func (xl XLog) Alert (message string) { xl.Log(syslog.LOG_ALERT, message) }
func (xl XLog) Alertf (message string, values ...interface{}) { xl.Log(syslog.LOG_ALERT, fmt.Sprintf(message, values...)) }
//
func (xl XLog) Emerg (message string) { xl.Log(syslog.LOG_EMERG, message) }
func (xl XLog) Fatal (message string) { xl.Log(syslog.LOG_EMERG, message) }
func (xl XLog) Emergf (message string, values ...interface{}) { xl.Log(syslog.LOG_EMERG, fmt.Sprintf(message, values...)) }
