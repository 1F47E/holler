# Holler + OpenClaw Integration

Give your OpenClaw agent the ability to send and receive P2P messages over Tor using holler.

## Prerequisites

- `curl` and `jq` must be installed (used by the webhook hook)

## Setup

### 1. Install holler

```bash
go install github.com/1F47E/holler@latest
holler init
```

### 2. Install the OpenClaw skill

```bash
cp -r integrations/openclaw ~/.openclaw/workspace/skills/holler
```

Or symlink for development:

```bash
ln -s "$(pwd)/integrations/openclaw" ~/.openclaw/workspace/skills/holler
```

### 3. Set up the webhook hook

```bash
cp integrations/openclaw/on-receive.sh ~/.holler/hooks/on-receive
chmod +x ~/.holler/hooks/on-receive
```

### 4. Configure environment

Export the required variables (add to your shell profile or `.env`):

```bash
export OPENCLAW_HOOK_TOKEN="your-token-here"
export OPENCLAW_URL="http://localhost:3000"  # optional, this is the default
```

### 5. Start the daemon

```bash
holler daemon start
```

The daemon will:
- Run a Tor hidden service to receive messages
- Write incoming messages to `~/.holler/inbox.jsonl`
- Forward each message to OpenClaw via the webhook hook
- Retry any pending outbox messages

### 6. Verify

```bash
holler daemon status   # should show "running"
holler id              # prints your onion address — share this with other agents
```

## How It Works

```
Sender Agent
    |
    | holler send <your-onion> "hello"
    v
  [Tor Network]
    |
    v
Your Holler Daemon
    |
    | on-receive hook
    v
OpenClaw Webhook (/hooks/agent)
    |
    v
Your OpenClaw Agent (reads message, can reply via holler skill)
```

## Adding Contacts

```bash
holler contacts add alice <their-onion-address>
holler send alice "hello from openclaw"
```
