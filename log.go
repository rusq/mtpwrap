package mtpwrap

import (
	"os"

	"github.com/rusq/dlog"
)

// Logger is the logger interface that is used throughout the package.
type Logger interface {
	Print(...any)
	Printf(string, ...any)
	Println(...any)
	Debug(...any)
	Debugf(string, ...any)
	Debugln(...any)
}

// Log is the global logger, replace it in the downstream, if needed, or go
// with the default one.
var Log Logger = dlog.New(os.Stderr, "", 0, false)
