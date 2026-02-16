package node

import (
	"context"
	"fmt"
	"os"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

const RendezvousNS = "holler/v1"

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
	connected := 0
	for _, addr := range dht.DefaultBootstrapPeers {
		pi, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			continue
		}
		go func(pi peer.AddrInfo) {
			connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			if err := h.Connect(connectCtx, pi); err != nil {
				logf("bootstrap peer %s: failed: %v", pi.ID.String()[:16], err)
			} else {
				logf("bootstrap peer %s: connected", pi.ID.String()[:16])
			}
		}(*pi)
		connected++
	}
	logf("connecting to %d bootstrap peers...", connected)

	return d, nil
}

// Advertise announces this peer on the DHT rendezvous namespace so others can find it.
func Advertise(ctx context.Context, h host.Host, d *dht.IpfsDHT) {
	routingDiscovery := drouting.NewRoutingDiscovery(d)
	dutil.Advertise(ctx, routingDiscovery, RendezvousNS)
	logf("advertising on DHT rendezvous: %s", RendezvousNS)
}

// FindPeer looks up a peer by ID on the DHT.
func FindPeer(ctx context.Context, d *dht.IpfsDHT, id peer.ID) (peer.AddrInfo, error) {
	logf("DHT FindPeer: %s", id.String()[:16])
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	ai, err := d.FindPeer(ctx, id)
	if err != nil {
		logf("DHT FindPeer failed: %v", err)
		return ai, err
	}
	logf("DHT FindPeer found %d addrs:", len(ai.Addrs))
	for _, a := range ai.Addrs {
		logf("  %s", a)
	}
	return ai, nil
}

// FindPeersRendezvous discovers peers via the holler rendezvous namespace.
// Returns the AddrInfo for targetID if found among rendezvous peers.
func FindPeersRendezvous(ctx context.Context, h host.Host, d *dht.IpfsDHT, targetID peer.ID) (peer.AddrInfo, error) {
	logf("searching rendezvous namespace: %s for peer %s", RendezvousNS, targetID.String()[:16])
	routingDiscovery := drouting.NewRoutingDiscovery(d)
	peerChan, err := routingDiscovery.FindPeers(ctx, RendezvousNS)
	if err != nil {
		return peer.AddrInfo{}, fmt.Errorf("find rendezvous peers: %w", err)
	}

	timeout := time.After(30 * time.Second)
	for {
		select {
		case p, ok := <-peerChan:
			if !ok {
				return peer.AddrInfo{}, fmt.Errorf("peer %s not found via rendezvous", targetID.String()[:16])
			}
			if p.ID == "" || p.ID == h.ID() {
				continue
			}
			logf("rendezvous found peer: %s (%d addrs)", p.ID.String()[:16], len(p.Addrs))
			if p.ID == targetID {
				// Rendezvous may return 0 addrs — supplement with DHT lookup
				if len(p.Addrs) == 0 {
					logf("rendezvous returned 0 addrs, doing DHT lookup for addresses...")
					findCtx, findCancel := context.WithTimeout(ctx, 15*time.Second)
					ai, err := d.FindPeer(findCtx, targetID)
					findCancel()
					if err == nil && len(ai.Addrs) > 0 {
						logf("DHT returned %d addrs for peer", len(ai.Addrs))
						return ai, nil
					}
					// Also check peerstore — libp2p may have learned addrs already
					storeAddrs := h.Peerstore().Addrs(targetID)
					if len(storeAddrs) > 0 {
						logf("peerstore has %d addrs for peer", len(storeAddrs))
						return peer.AddrInfo{ID: targetID, Addrs: storeAddrs}, nil
					}
					logf("no addresses found for peer, returning peer ID only")
				}
				return p, nil
			}
		case <-timeout:
			return peer.AddrInfo{}, fmt.Errorf("timeout: peer %s not found via rendezvous", targetID.String()[:16])
		case <-ctx.Done():
			return peer.AddrInfo{}, ctx.Err()
		}
	}
}

// WaitForBootstrap waits for at least one bootstrap connection, with progress logging.
func WaitForBootstrap(ctx context.Context, h host.Host, d *dht.IpfsDHT, duration time.Duration) {
	deadline := time.After(duration)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			rt := d.RoutingTable()
			fmt.Fprintf(os.Stderr, "DHT ready: %d peers in routing table, %d connected\n",
				rt.Size(), len(h.Network().Peers()))
			return
		case <-ticker.C:
			peers := len(h.Network().Peers())
			if Verbose {
				logf("bootstrap: %d connected peers", peers)
			}
			if peers > 0 && time.Since(time.Now()) > 2*time.Second {
				// At least one peer connected, give it one more second
			}
		case <-ctx.Done():
			return
		}
	}
}
