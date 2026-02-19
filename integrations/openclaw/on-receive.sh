#!/usr/bin/env bash
# on-receive hook for OpenClaw integration.
# Forwards incoming holler messages to OpenClaw's webhook endpoint.
#
# Install:
#   cp on-receive.sh ~/.holler/hooks/on-receive
#   chmod +x ~/.holler/hooks/on-receive
#
# Required env vars:
#   OPENCLAW_HOOK_TOKEN  — webhook authentication token
#
# Optional env vars:
#   OPENCLAW_URL         — base URL (default: http://localhost:3000)
#
# Requires: curl, jq

set -euo pipefail

if ! command -v jq &>/dev/null; then
  echo "on-receive: jq is required but not installed" >&2
  exit 1
fi

OPENCLAW_URL="${OPENCLAW_URL:-http://localhost:3000}"

if [ -z "${OPENCLAW_HOOK_TOKEN:-}" ]; then
  echo "on-receive: OPENCLAW_HOOK_TOKEN not set, skipping webhook" >&2
  exit 0
fi

# Read full envelope JSON from stdin
ENVELOPE=$(cat)

# Build the prompt from env vars set by the holler daemon
PROMPT="Incoming holler message from ${HOLLER_MSG_FROM:-unknown}:
Type: ${HOLLER_MSG_TYPE:-message}
Body: ${HOLLER_MSG_BODY:-}

Full envelope is attached as context. Respond if appropriate using the holler skill."

# POST to OpenClaw webhook
if ! curl -sf -X POST "${OPENCLAW_URL}/hooks/agent" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${OPENCLAW_HOOK_TOKEN}" \
  -d "$(jq -n \
    --arg prompt "$PROMPT" \
    --argjson context "$ENVELOPE" \
    '{prompt: $prompt, context: $context}'
  )" >/dev/null; then
  echo "on-receive: webhook POST to ${OPENCLAW_URL}/hooks/agent failed" >&2
  exit 1
fi

echo "on-receive: forwarded to OpenClaw" >&2
