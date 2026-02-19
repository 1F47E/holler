package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/1F47E/holler/message"
)

const hookTimeout = 10 * time.Second

// RunReceiveHook runs the on-receive hook if it exists and is executable.
// Skips ack and ping message types. Errors are logged, never fatal.
func RunReceiveHook(dir string, env *message.Envelope) {
	// Skip internal message types
	if env.Type == "ack" || env.Type == "ping" {
		return
	}

	hookPath := filepath.Join(dir, "hooks", "on-receive")
	info, err := os.Stat(hookPath)
	if err != nil || info.Mode()&0111 == 0 {
		return // hook doesn't exist or isn't executable
	}

	raw, err := json.Marshal(env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hook: marshal error: %v\n", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), hookTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, hookPath)
	cmd.Stdin = bytes.NewReader(raw)
	cmd.Stdout = os.Stderr // hook output goes to daemon log
	cmd.Stderr = os.Stderr

	// Set environment variables (sanitize newlines to prevent env var injection)
	sanitize := func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\r", " ")
	}
	body := env.Body
	if len(body) > 256 {
		body = body[:256]
	}
	cmd.Env = append(os.Environ(),
		"HOLLER_MSG_ID="+env.ID,
		"HOLLER_MSG_FROM="+env.From,
		"HOLLER_MSG_TYPE="+sanitize(env.Type),
		"HOLLER_MSG_BODY="+sanitize(body),
		fmt.Sprintf("HOLLER_MSG_TS=%d", env.Ts),
	)

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "hook: on-receive error: %v\n", err)
	}
}
