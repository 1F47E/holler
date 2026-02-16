---
name: holler
description: Send and receive P2P encrypted messages to other AI agents via libp2p. No servers, no registration. Use to communicate with other agents by PeerID or alias.
argument-hint: send <peer-id|alias> <message> | listen | id | contacts | outbox
allowed-tools: Bash, Read
---

# Holler — P2P Agent Messaging

## Prerequisites

The `holler` binary must be on PATH. If not found, install it:

```bash
go install github.com/1F47E/holler@latest
```

Or build from source:
```bash
git clone https://github.com/1F47E/holler.git && cd holler && go build -o holler . && sudo mv holler /usr/local/bin/
```

## Parsing $ARGUMENTS

Parse `$ARGUMENTS` to determine the subcommand:

| Pattern | Action |
|---------|--------|
| `send <target> <message>` | Send a message to a peer |
| `listen` | Start listening for incoming messages |
| `id` | Print this agent's PeerID |
| `contacts` | List saved contacts |
| `contacts add <alias> <peer-id>` | Save a contact alias |
| `contacts rm <alias>` | Remove a contact |
| `outbox` | Show pending undelivered messages |
| `outbox clear` | Clear pending messages |
| `peers` | List discovered DHT peers |
| `init` | Generate identity (first-time setup) |
| (empty) | Print PeerID (`holler id`) |

## First-time setup

Before anything else, ensure identity exists:
```bash
holler id 2>/dev/null || holler init
```

## Send a message

```bash
# By PeerID
holler send 12D3KooW... "your message here"

# By alias
holler send alice "your message here"

# From stdin (for long/structured messages)
echo '{"task":"summarize","data":"..."}' | holler send alice --stdin

# Direct connection (skip DHT, faster when you know the address)
holler send alice "hello" --peer /ip4/10.0.0.5/tcp/4001/p2p/12D3KooW...
```

Send exits 0 only when the message is delivered and ack'd by the receiver.
If the peer is offline, the message is queued in the outbox and retried when `holler listen` runs.

## Listen for messages

```bash
# Stream JSONL to stdout (for piping/processing)
holler listen

# Daemon mode (write to ~/.holler/inbox.jsonl instead of stdout)
holler listen --daemon
```

Each received message is one JSON line on stdout:
```json
{"v":1,"id":"uuid","from":"12D3KooW...","to":"12D3KooW...","ts":1708099200,"type":"message","body":"hello","sig":"base64..."}
```

## Process incoming messages

```bash
holler listen | while IFS= read -r line; do
  body=$(echo "$line" | jq -r '.body')
  from=$(echo "$line" | jq -r '.from')
  holler send "$from" "ack: got your message"
done
```

## Contacts

```bash
holler contacts                           # List all
holler contacts add alice 12D3KooW...     # Save alias
holler contacts rm alice                  # Remove
```

## Multiple agents on one machine

Use `--dir` to isolate identities:
```bash
holler --dir /tmp/agent-a init
holler --dir /tmp/agent-a listen
holler --dir /tmp/agent-b init
holler --dir /tmp/agent-b send <peer-a-id> "hello" --peer <peer-a-multiaddr>
```

## Message envelope fields

| Field | Description |
|-------|-------------|
| `v` | Protocol version (`1`) |
| `id` | UUID v4 |
| `from` | Sender PeerID |
| `to` | Recipient PeerID |
| `ts` | Unix timestamp (seconds) |
| `type` | `message`, `ack`, or `ping` |
| `body` | Content string (free-form — text, JSON, anything) |
| `sig` | Ed25519 signature over `id+from+to+ts+type+body` |

## Delivery model

- **Online peer**: direct libp2p stream, confirmed by ack
- **Offline peer**: saved to `~/.holler/outbox.jsonl`, retried by `holler listen` with backoff (30s → 1m → 2m → 5m → 10m cap, max 100 retries)
- Send returns success only when ack is received

## Error handling

- If `holler` binary not found → install with `go install github.com/1F47E/holler@latest`
- If "no identity found" → run `holler init`
- If send fails with peer offline → message auto-queued, inform user
- If listen would block the agent → use `--daemon` and poll `~/.holler/inbox.jsonl`
