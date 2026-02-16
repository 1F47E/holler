package node

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
)

// NewHost creates a libp2p host with the given private key and all transports enabled.
func NewHost(ctx context.Context, privKey crypto.PrivKey) (host.Host, error) {
	cm, err := connmgr.NewConnManager(10, 100)
	if err != nil {
		return nil, fmt.Errorf("create conn manager: %w", err)
	}

	h, err := libp2p.New(
		libp2p.Identity(privKey),
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic-v1",
			"/ip6/::/tcp/0",
			"/ip6/::/udp/0/quic-v1",
		),
		libp2p.EnableRelay(),
		libp2p.EnableHolePunching(),
		libp2p.NATPortMap(),
		libp2p.ConnectionManager(cm),
	)
	if err != nil {
		return nil, fmt.Errorf("create libp2p host: %w", err)
	}
	return h, nil
}
