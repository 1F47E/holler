package identity

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cretz/bine/control"
	"github.com/cretz/bine/torutil"
	bineed25519 "github.com/cretz/bine/torutil/ed25519"
)

const defaultDir = ".holler"

// DirOverride is set by the --dir flag. Empty means use default ~/.holler/.
var DirOverride string

// HollerDir returns the holler data directory, creating it if needed.
func HollerDir() (string, error) {
	dir := DirOverride
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home dir: %w", err)
		}
		dir = filepath.Join(home, defaultDir)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create holler dir: %w", err)
	}
	return dir, nil
}

// OnionKeyPairFromBine extracts the bine ed25519 KeyPair from a control.ED25519Key.
func OnionKeyPairFromBine(key *control.ED25519Key) bineed25519.KeyPair {
	return key.KeyPair
}

// OnionAddrFromKey computes the 56-char lowercase onion service ID from a bine key.
func OnionAddrFromKey(key *control.ED25519Key) string {
	return strings.ToLower(torutil.OnionServiceIDFromPrivateKey(key.KeyPair))
}
