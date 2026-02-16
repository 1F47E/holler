package node

import (
	"fmt"
	"net"
	"time"
)

// TorMode enables Tor transport mode (set by --tor flag).
var TorMode bool

// CheckTorAvailable verifies the Tor daemon is reachable on SOCKS5 (9050) and control (9051) ports.
func CheckTorAvailable() error {
	// Check SOCKS5 proxy port
	conn, err := net.DialTimeout("tcp", "127.0.0.1:9050", 2*time.Second)
	if err != nil {
		return fmt.Errorf("Tor SOCKS5 proxy not reachable on 127.0.0.1:9050 — install and start tor:\n  macOS: brew install tor && brew services start tor\n  Linux: sudo apt install tor && sudo systemctl start tor")
	}
	conn.Close()

	// Check control port
	conn, err = net.DialTimeout("tcp", "127.0.0.1:9051", 2*time.Second)
	if err != nil {
		return fmt.Errorf("Tor control port not reachable on 127.0.0.1:9051 — enable it in torrc:\n  ControlPort 9051\n  CookieAuthentication 1")
	}
	conn.Close()

	logf("tor: SOCKS5 and control port reachable")
	return nil
}
