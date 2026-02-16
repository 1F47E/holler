package node

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/cretz/bine/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/transport"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	"golang.org/x/net/proxy"
)

const (
	torSOCKSAddr = "127.0.0.1:9050"
	onion3Code   = 445 // ma.P_ONION3
)

// TorTransport implements transport.Transport for dialing and listening over Tor.
type TorTransport struct {
	upgrader transport.Upgrader
	rcmgr    network.ResourceManager
	onionKey *control.ED25519Key // set before Listen is called
}

// NewTorTransport constructs a TorTransport. Parameters are injected by libp2p's fx.
func NewTorTransport(upgrader transport.Upgrader, rcmgr network.ResourceManager) (*TorTransport, error) {
	return &TorTransport{
		upgrader: upgrader,
		rcmgr:    rcmgr,
	}, nil
}

// Dial connects to a remote peer through Tor SOCKS5 proxy.
func (t *TorTransport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (transport.CapableConn, error) {
	onionHost, port, err := parseOnion3(raddr)
	if err != nil {
		return nil, fmt.Errorf("tor dial: %w", err)
	}
	target := fmt.Sprintf("%s.onion:%d", onionHost, port)
	logf("tor: dialing %s via SOCKS5", target)

	// Open resource scope
	scope, err := t.rcmgr.OpenConnection(network.DirOutbound, true, raddr)
	if err != nil {
		return nil, fmt.Errorf("tor dial: resource manager: %w", err)
	}
	if err := scope.SetPeer(p); err != nil {
		scope.Done()
		return nil, fmt.Errorf("tor dial: set peer: %w", err)
	}

	// Dial via SOCKS5
	dialer, err := proxy.SOCKS5("tcp", torSOCKSAddr, nil, proxy.Direct)
	if err != nil {
		scope.Done()
		return nil, fmt.Errorf("tor dial: create SOCKS5 dialer: %w", err)
	}

	// Use context-aware dialing if available
	var rawConn net.Conn
	if ctxDialer, ok := dialer.(proxy.ContextDialer); ok {
		rawConn, err = ctxDialer.DialContext(ctx, "tcp", target)
	} else {
		rawConn, err = dialer.Dial("tcp", target)
	}
	if err != nil {
		scope.Done()
		return nil, fmt.Errorf("tor dial: SOCKS5 connect to %s: %w", target, err)
	}

	// Wrap raw conn with multiaddr endpoints
	maConn := &onionConn{
		Conn:   rawConn,
		laddr:  localOnionMultiaddr(),
		raddr:  raddr,
	}

	// Upgrade to secure multiplexed connection.
	// The upgrader takes ownership of the scope — don't call scope.Done() on failure.
	capConn, err := t.upgrader.Upgrade(ctx, t, maConn, network.DirOutbound, p, scope)
	if err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("tor dial: upgrade: %w", err)
	}
	return capConn, nil
}

// CanDial returns true for onion3 multiaddrs.
func (t *TorTransport) CanDial(addr ma.Multiaddr) bool {
	return hasOnion3(addr)
}

// Listen creates a Tor onion service and returns a listener.
// The onion key must be set via SetOnionKey before calling Listen.
func (t *TorTransport) Listen(laddr ma.Multiaddr) (transport.Listener, error) {
	if t.onionKey == nil {
		return nil, fmt.Errorf("tor listen: onion key not set — call SetOnionKey first")
	}
	return torListen(t, t.onionKey)
}

// SetOnionKey sets the ED25519 key for the onion service. Must be called before Listen.
func (t *TorTransport) SetOnionKey(key *control.ED25519Key) {
	t.onionKey = key
}

// Protocols returns the protocol codes this transport handles.
func (t *TorTransport) Protocols() []int {
	return []int{onion3Code}
}

// Proxy returns true — this transport proxies through Tor.
func (t *TorTransport) Proxy() bool {
	return true
}

// parseOnion3 extracts the 56-char base32 onion host and port from an onion3 multiaddr.
// Format: /onion3/<56-char-base32>:<port>[/p2p/<peerid>]
func parseOnion3(addr ma.Multiaddr) (host string, port int, err error) {
	var found bool
	ma.ForEach(addr, func(c ma.Component) bool {
		if c.Protocol().Code == onion3Code {
			// Value is "host:port"
			val := c.Value()
			parts := strings.SplitN(val, ":", 2)
			if len(parts) != 2 {
				err = fmt.Errorf("invalid onion3 value: %s", val)
				return false
			}
			host = parts[0]
			var p int
			if _, scanErr := fmt.Sscanf(parts[1], "%d", &p); scanErr != nil {
				err = fmt.Errorf("invalid onion3 port: %s", parts[1])
				return false
			}
			port = p
			found = true
			return false
		}
		return true
	})
	if err != nil {
		return "", 0, err
	}
	if !found {
		return "", 0, fmt.Errorf("no onion3 component in multiaddr: %s", addr)
	}
	return host, port, nil
}

// hasOnion3 checks if a multiaddr contains an onion3 component.
func hasOnion3(addr ma.Multiaddr) bool {
	found := false
	ma.ForEach(addr, func(c ma.Component) bool {
		if c.Protocol().Code == onion3Code {
			found = true
			return false
		}
		return true
	})
	return found
}

// localOnionMultiaddr returns a placeholder local multiaddr for Tor connections.
// We use the SOCKS5 proxy address since we don't know our own onion addr during dial.
func localOnionMultiaddr() ma.Multiaddr {
	addr, _ := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/9050")
	return addr
}

// onionConn wraps a net.Conn with explicit multiaddr endpoints for Tor connections.
type onionConn struct {
	net.Conn
	laddr ma.Multiaddr
	raddr ma.Multiaddr
}

// Ensure onionConn satisfies manet.Conn.
var _ manet.Conn = (*onionConn)(nil)

func (c *onionConn) LocalMultiaddr() ma.Multiaddr  { return c.laddr }
func (c *onionConn) RemoteMultiaddr() ma.Multiaddr { return c.raddr }
