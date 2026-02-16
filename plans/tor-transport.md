# Tor Transport for Holler

## Context

Holler currently exposes real IP addresses via TCP/QUIC/WebRTC. Adding `--tor` mode routes ALL traffic through Tor, hiding the peer's IP. This enables truly anonymous agent-to-agent communication. Requires an external `tor` daemon running on the machine (SOCKS5 on port 9050, control on port 9051).

## Approach: Custom libp2p Transport + External Tor Daemon

We'll create a custom `transport.Transport` implementation that:
- **Dials** through Tor's SOCKS5 proxy (127.0.0.1:9050)
- **Listens** by creating an ephemeral v3 onion service via Tor control port (127.0.0.1:9051)
- Uses `/onion3/<addr>:<port>` multiaddr format (protocol code 445, supported by `go-multiaddr`)

### Key reference implementations researched:
- `berty/go-libp2p-tor-transport` — full libp2p transport, but tightly coupled with embedded Tor
- `cpacia/go-onion-transport` — similar, embeds Tor via go-libtor
- `cretz/bine` — Go library for Tor control protocol, creates ephemeral onion services
- `cretz/tor-dht-poc` — proof-of-concept: DHT over Tor with libp2p + bine

Our approach: **Write our own minimal transport** using `golang.org/x/net/proxy` (SOCKS5) + `github.com/cretz/bine` (control port for onion services). Keep it simple and self-contained.

## New Dependencies

```
github.com/cretz/bine          — Tor control protocol (onion service creation) [already in go.mod]
golang.org/x/net/proxy          — SOCKS5 dialing (already in go.mod as golang.org/x/net)
```

## Files to Create/Modify

### 1. `node/tor.go` (NEW) — Tor transport implementation

The `libp2p.Transport()` option uses **fx dependency injection** — it accepts a constructor function whose parameters are automatically resolved. Follow the TCP transport pattern:

```go
// Constructor — libp2p injects upgrader and rcmgr automatically
func NewTorTransport(upgrader transport.Upgrader, rcmgr network.ResourceManager) (*TorTransport, error) {
    // Connect to external Tor daemon via bine control port
    t, err := tor.Start(context.Background(), &tor.StartConf{
        NoProcessCreation: true,          // use external daemon, don't spawn
        ControlPort:       9051,
        DataDir:           "",            // not needed for external
    })
    if err != nil {
        return nil, fmt.Errorf("connect to tor control port: %w", err)
    }

    return &TorTransport{
        upgrader:    upgrader,
        rcmgr:       rcmgr,
        torCtrl:     t,
        socksAddr:   "127.0.0.1:9050",
    }, nil
}

type TorTransport struct {
    upgrader    transport.Upgrader
    rcmgr       network.ResourceManager
    socksAddr   string           // "127.0.0.1:9050"
    torCtrl     *tor.Tor         // bine Tor controller (control port connection)
}
```

Methods implementing `transport.Transport`:
- `Dial(ctx, raddr, peerID)` — extract onion host:port from `/onion3/...` multiaddr, dial via SOCKS5, then `upgrader.Upgrade(ctx, t, rawConn, network.DirOutbound, peerID, connScope)` → `CapableConn`
- `CanDial(addr)` — returns true if addr contains `/onion3/` (protocol code 445)
- `Listen(laddr)` — create ephemeral onion service via bine, return `TorListener`
- `Protocols()` — returns `[]int{445}` (ma.P_ONION3)
- `Proxy()` — returns `true`

### 2. `node/tor_listener.go` (NEW) — Tor listener wrapping onion service

Wraps bine's `OnionService` as a `transport.Listener`. The flow is:
1. `bine.Listen()` → creates ephemeral v3 onion service, returns `OnionService` with embedded `net.Listener`
2. Raw TCP accept → `upgrader.Upgrade(ctx, t, rawConn, network.DirInbound, "", connScope)` → `CapableConn`

```go
type TorListener struct {
    onion     *tor.OnionService  // bine onion service (has Accept())
    transport *TorTransport
    upgrader  transport.Upgrader
    maddr     ma.Multiaddr       // /onion3/<base32addr>:<port>
}
```

Methods:
- `Accept()` — `onion.Accept()` returns raw net.Conn, then upgrade via upgrader
- `Close()` — tear down onion service
- `Addr()` — returns net.Addr
- `Multiaddr()` — returns `/onion3/<base32-addr>:<port>`

Onion service creation:
```go
onion, err := torCtrl.Listen(ctx, &tor.ListenConf{
    Version3:    true,
    RemotePorts: []int{9000},       // fixed port on the onion side
    Key:         loadedOnionKey,    // nil = generate new, or load from tor_key
})
// onion.ID = "abc123...xyz" (56-char base32, without .onion)
// onion.Key = ed25519 private key (save for persistence)
```

### 3. `node/tor_check.go` (NEW) — Pre-flight checks

```go
func CheckTorAvailable() error
```

- Verify SOCKS5 proxy at 127.0.0.1:9050 (TCP connect test)
- Verify control port at 127.0.0.1:9051 (TCP connect test)
- Return actionable errors:
  - macOS: `"Tor not running. Install: brew install tor && brew services start tor"`
  - Linux: `"Tor not running. Install: sudo apt install tor && sudo systemctl start tor"`
  - Also remind: `"Ensure 'CookieAuthentication 1' or 'HashedControlPassword ...' is set in /etc/tor/torrc"`

### 4. `node/tor_key.go` (NEW) — Onion key persistence

```go
func LoadOrCreateOnionKey(hollerDir string) (crypto.PrivateKey, error)
func SaveOnionKey(hollerDir string, key crypto.PrivateKey) error
```

- Key file: `~/.holler/tor_key` (0600 permissions)
- On first run: bine generates a new Ed25519 key → save to `tor_key`
- On subsequent runs: load from `tor_key` → pass to `ListenConf.Key`
- Stable `.onion` address across restarts — agents can save it permanently

### 5. `node/host.go` (MODIFY) — Add Tor mode to NewHost

New function `NewHostTor()` — stripped-down host with only TorTransport:

```go
func NewHostTor(ctx context.Context, privKey crypto.PrivKey, dhtPtr **dht.IpfsDHT) (host.Host, error) {
    cm, _ := connmgr.NewConnManager(10, 100)

    h, err := libp2p.New(
        libp2p.Identity(privKey),
        libp2p.Transport(NewTorTransport),   // fx injects upgrader+rcmgr
        libp2p.ListenAddrStrings(
            "/onion3/0000...0000:9000",      // placeholder — TorTransport.Listen() replaces with real addr
        ),
        libp2p.DisableRelay(),               // no relay over Tor
        libp2p.ConnectionManager(cm),
        // NO: AutoRelay, NATPortMap, HolePunching, AutoNAT — useless over Tor
    )
    return h, err
}
```

### 6. `cmd/root.go` (MODIFY) — Add `--tor` as persistent flag

```go
var TorMode bool

func init() {
    rootCmd.PersistentFlags().BoolVar(&TorMode, "tor", false, "Route all traffic through Tor (requires tor daemon)")
}
```

All subcommands read `TorMode` to decide between `NewHost()` and `NewHostTor()`.

### 7. `cmd/listen.go` (MODIFY) — Tor-aware listener

When `TorMode`:
1. `CheckTorAvailable()` — fail fast with helpful error
2. `NewHostTor()` instead of `NewHost()`
3. Print onion address: `Listening as <peerID> via Tor: <addr>.onion:9000`
4. DHT bootstrap goes through SOCKS5 (all connections use TorTransport)
5. **Outbox retry also uses Tor** — `retryOutboxLoop` inherits the Tor host, so all outbox deliveries go through SOCKS5 automatically (no code change needed — it uses the same `h host.Host`)

### 8. `cmd/send.go` (MODIFY) — Tor-aware sender

When `TorMode`:
1. `CheckTorAvailable()`
2. `NewHostTor()`
3. `--peer` flag accepts `/onion3/...` multiaddrs
4. DHT lookup returns `/onion3/` addrs for Tor peers

### 9. `cmd/ping.go` (MODIFY) — Same pattern

## Connection Flow

### Sending (Dial)
```
holler send --tor vrgo "hello"
  → CheckTorAvailable() — verify SOCKS5 + control port
  → NewHostTor() — libp2p host with TorTransport only
  → DHT bootstrap peers dialed through SOCKS5 proxy
  → DHT FindPeer returns /onion3/<addr>:<port> multiaddr for Tor peers
  → TorTransport.Dial():
      → SOCKS5 connect to <addr>.onion:<port>
      → upgrader.Upgrade() handles Noise + yamux handshake
  → Send message over upgraded stream
```

### Receiving (Listen)
```
holler listen --tor
  → CheckTorAvailable() — verify SOCKS5 + control port
  → LoadOrCreateOnionKey() — persistent .onion address
  → NewHostTor() — creates onion service via bine control port
  → Print: "Listening via Tor: <56char>.onion:9000"
  → Advertise /onion3/<addr>:9000 on DHT (through Tor)
  → Accept loop:
      → bine onion service accepts raw TCP
      → upgrader.Upgrade() handles Noise + yamux
      → RegisterHandler processes message
  → Outbox retry also goes through Tor (same host)
```

## Mixed Network: Tor ↔ Clearnet

**Tor-only peers can only talk to other Tor peers.** This is by design — if we allowed Tor→clearnet dialing, it would leak information through exit nodes and defeat the purpose.

The network naturally partitions:
- **Clearnet peers** advertise `/ip4/...` and `/ip6/...` on DHT — other clearnet peers dial them directly
- **Tor peers** advertise `/onion3/...` on DHT — other Tor peers dial them through Tor
- **A peer can run both modes** simultaneously (two separate `holler listen` processes with the same `--dir`) to bridge the two networks — but this is an advanced use case, not a default

DHT FindPeer returns ALL addresses for a peer. A Tor sender filters for `/onion3/` addrs only (`CanDial` returns false for non-onion addrs). If the target peer has no onion addr, the send fails with: `"peer has no Tor-reachable address — they may not be running in --tor mode"`.

## Critical Details

1. **No IP leak**: When `--tor`, only TorTransport is registered. `CanDial()` rejects non-onion addrs. No TCP/QUIC/WebRTC. All outbound go through SOCKS5.

2. **DHT over Tor**: DHT bootstrap peers are clearnet nodes dialed through SOCKS5 exit circuits — this is slow (~10-30s) but works. Increase bootstrap timeout to 30s in Tor mode:
   ```go
   node.WaitForBootstrap(ctx, h, d, 30*time.Second)  // was 5s
   ```

3. **Onion service key persistence**: `~/.holler/tor_key` (Ed25519, 0600). Stable `.onion` address across restarts. Add to data directory docs.

4. **Control port auth**: bine supports both cookie auth and hashed password auth. Document setup for both:
   - Cookie (default on most installs): `CookieAuthentication 1` in torrc
   - Password: `HashedControlPassword <hash>` in torrc + env var `HOLLER_TOR_CONTROL_PASSWORD`

5. **Outbox retry safety**: `retryOutboxLoop` uses the same `h host.Host` that was created with `NewHostTor()`. All connections from that host go through TorTransport — no IP leak on retry. No code change needed.

6. **Multiaddr format**: `/onion3/<base32-addr>:<port>` — 56-char base32 address (no .onion suffix) + colon + port. Protocol code 445.

7. **No DNS leak**: TorTransport only dials onion addresses (resolved by Tor internally). No DNS resolution happens outside Tor. DHT bootstrap nodes are dialed by IP through SOCKS5.

## Data Directory (updated)

```
~/.holler/
  key.bin         Ed25519 private key for libp2p identity (0600)
  tor_key         Ed25519 private key for .onion address (0600, created on first --tor run)
  contacts.json   alias → PeerID map
  inbox.jsonl     received messages
  sent.jsonl      sent message history
  outbox.jsonl    pending messages awaiting delivery
```

## Verification

1. **Build**: `go build ./...` — verify compilation

2. **Pre-flight check** (no tor running):
   ```bash
   holler listen --tor
   # Error: Tor not running. Install: brew install tor && brew services start tor
   ```

3. **Onion service creation**:
   ```bash
   brew install tor && brew services start tor
   holler listen --tor -v
   # [debug] Tor control port connected
   # [debug] Onion service created: <56char>.onion:9000
   # Listening as 12D3KooW... via Tor
   # Verify: no real IP in any output
   ```

4. **Direct Tor send** (two terminals, same machine for testing):
   ```bash
   # Terminal 1:
   holler --dir /tmp/agent-a listen --tor -v
   # → prints onion address

   # Terminal 2:
   holler --dir /tmp/agent-b send --tor --peer /onion3/<addr>:9000/p2p/<peerID> "test"
   # → message delivered via Tor
   ```

5. **DHT discovery over Tor** (slower, real-world test):
   ```bash
   # Terminal 1: listen + advertise on DHT
   holler --dir /tmp/agent-a listen --tor

   # Terminal 2: find via DHT (no --peer)
   holler --dir /tmp/agent-b send --tor <peerID> "found you via DHT over Tor"
   ```

6. **Key persistence**:
   ```bash
   holler listen --tor -v  # note onion address
   # Ctrl+C
   holler listen --tor -v  # same onion address (loaded from ~/.holler/tor_key)
   ```

7. **Backwards compat** — non-Tor peers unaffected:
   ```bash
   holler send feesh9 "normal message"  # works as before, no Tor involved
   ```

## Version Bump

Bump to v0.3.0 in `cmd/version.go` — Tor transport is a major new feature.
