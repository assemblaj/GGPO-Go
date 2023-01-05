package ggpo

import (
	"log"
	"os"

	"github.com/assemblaj/ggpo/internal/util"
)

// SetLogger sets the log.Logger used when printing log messages. Use this
// method for more precise control over the logging output and format. Use
// EnableLogs and DisableLogs for simple logging to os.Stdout. Log messages
// are not printed by default.
func SetLogger(l *log.Logger) {
	util.Log = l
}

// EnableLogs enables printing log messages to os.Stdout. Use SetLogger for
// more precise control over the logging output and format. Log messages are
// not printed by default.
func EnableLogs() {
	SetLogger(log.New(os.Stdout, "GGPO ", log.Ldate|log.Ltime|log.Lmsgprefix))
}

// DisableLogs disables printing log messages.
func DisableLogs() {
	SetLogger(util.DiscardLogger())
}
