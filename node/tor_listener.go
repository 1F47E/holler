package node

import (
	"fmt"
	"net"
	"net/textproto"
	"strings"

	"github.com/cretz/bine/control"
	"github.com/cretz/bine/torutil"
	"github.com/libp2p/go-libp2p/core/transport"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

const onionVirtualPort = 9000

// TorListener wraps a local TCP listener with a Tor onion service.
// Tor forwards connections from <onion>.onion:9000 → localhost:<localport>.
type TorListener struct {
	inner     transport.Listener
	ctrl      *control.Conn
	serviceID string
	onionAddr ma.Multiaddr
}

// torListen creates an onion service and returns a TorListener.
// It connects to the Tor control port, registers the onion service using the
// provided key, and upgrades the underlying TCP listener for libp2p.
func torListen(t *TorTransport, onionKey *control.ED25519Key) (*TorListener, error) {
	// Start a local TCP listener for Tor to forward connections to
	localListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("tor listen: bind local: %w", err)
	}
	localPort := localListener.Addr().(*net.TCPAddr).Port
	logf("tor: local listener on 127.0.0.1:%d", localPort)

	// Connect to Tor control port
	textConn, err := textproto.Dial("tcp", "127.0.0.1:9051")
	if err != nil {
		localListener.Close()
		return nil, fmt.Errorf("tor listen: connect to control port: %w", err)
	}
	ctrl := control.NewConn(textConn)

	if err := ctrl.Authenticate(""); err != nil {
		localListener.Close()
		ctrl.Close()
		return nil, fmt.Errorf("tor listen: authenticate with control port: %w", err)
	}
	logf("tor: authenticated with control port")

	// Create onion service: virtual port 9000 → local port
	resp, err := ctrl.AddOnion(&control.AddOnionRequest{
		Key: onionKey,
		Ports: []*control.KeyVal{
			{Key: fmt.Sprintf("%d", onionVirtualPort), Val: fmt.Sprintf("127.0.0.1:%d", localPort)},
		},
	})
	if err != nil {
		localListener.Close()
		ctrl.Close()
		return nil, fmt.Errorf("tor listen: create onion service: %w", err)
	}
	logf("tor: onion service created: %s.onion:%d", resp.ServiceID, onionVirtualPort)

	// Build the onion3 multiaddr
	onionAddr, err := ma.NewMultiaddr(fmt.Sprintf("/onion3/%s:%d", resp.ServiceID, onionVirtualPort))
	if err != nil {
		localListener.Close()
		ctrl.Close()
		return nil, fmt.Errorf("tor listen: build multiaddr: %w", err)
	}

	// Wrap as manet.Listener and upgrade for libp2p
	maListener, err := manet.WrapNetListener(localListener)
	if err != nil {
		localListener.Close()
		ctrl.Close()
		return nil, fmt.Errorf("tor listen: wrap listener: %w", err)
	}

	// Gate and upgrade the listener for libp2p
	wrappedMaListener := &onionMaListener{
		Listener: maListener,
		addr:     onionAddr,
	}
	gated := t.upgrader.GateMaListener(wrappedMaListener)
	upgraded := t.upgrader.UpgradeGatedMaListener(t, gated)

	return &TorListener{
		inner:     upgraded,
		ctrl:      ctrl,
		serviceID: resp.ServiceID,
		onionAddr: onionAddr,
	}, nil
}

// Accept waits for the next inbound connection.
func (l *TorListener) Accept() (transport.CapableConn, error) {
	return l.inner.Accept()
}

// Close tears down the onion service and closes everything.
func (l *TorListener) Close() error {
	var firstErr error
	// Remove onion service from Tor
	if l.ctrl != nil && l.serviceID != "" {
		if err := l.ctrl.DelOnion(l.serviceID); err != nil {
			firstErr = fmt.Errorf("remove onion service: %w", err)
		}
		if err := l.ctrl.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close tor control: %w", err)
		}
	}
	if err := l.inner.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// Addr returns the net.Addr of the listener.
func (l *TorListener) Addr() net.Addr {
	return l.inner.Addr()
}

// Multiaddr returns the onion3 multiaddr for this listener.
func (l *TorListener) Multiaddr() ma.Multiaddr {
	return l.onionAddr
}

// OnionServiceID returns the 56-char base32 onion service ID.
func (l *TorListener) OnionServiceID() string {
	return l.serviceID
}

// OnionHost returns the full .onion hostname (serviceID + ".onion").
func (l *TorListener) OnionHost() string {
	return l.serviceID + ".onion"
}

// onionMaListener wraps a manet.Listener but overrides Multiaddr to return the onion address.
type onionMaListener struct {
	manet.Listener
	addr ma.Multiaddr
}

func (l *onionMaListener) Multiaddr() ma.Multiaddr {
	return l.addr
}

// OnionAddressFromKey computes the 56-char onion service ID from an ED25519 key
// without creating the actual service. Useful for displaying what the onion address
// will be before starting to listen.
func OnionAddressFromKey(key *control.ED25519Key) string {
	return strings.ToLower(torutil.OnionServiceIDFromPrivateKey(key.KeyPair))
}
