package node

import (
	"context"
	"fmt"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// NewDHT creates and bootstraps a Kademlia DHT instance.
func NewDHT(ctx context.Context, h host.Host) (*dht.IpfsDHT, error) {
	d, err := dht.New(ctx, h, dht.Mode(dht.ModeAutoServer))
	if err != nil {
		return nil, fmt.Errorf("create DHT: %w", err)
	}
	if err := d.Bootstrap(ctx); err != nil {
		return nil, fmt.Errorf("bootstrap DHT: %w", err)
	}

	// Connect to default bootstrap peers
	for _, addr := range dht.DefaultBootstrapPeers {
		pi, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			continue
		}
		go func(pi peer.AddrInfo) {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			h.Connect(ctx, pi)
		}(*pi)
	}

	return d, nil
}

// FindPeer looks up a peer by ID on the DHT.
func FindPeer(ctx context.Context, d *dht.IpfsDHT, id peer.ID) (peer.AddrInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return d.FindPeer(ctx, id)
}
