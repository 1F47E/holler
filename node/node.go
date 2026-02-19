package node

import (
	"fmt"
	"os"

	"github.com/1F47E/holler/message"
)

// Verbose enables debug logging (set by --verbose flag).
var Verbose bool

// MessageHandler processes a received, verified envelope.
type MessageHandler func(env *message.Envelope)

func logf(format string, args ...interface{}) {
	if Verbose {
		fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", args...)
	}
}
