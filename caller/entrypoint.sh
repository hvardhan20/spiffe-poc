#!/usr/bin/env bash
set -euo pipefail

API_URL="${API_URL:?set API_URL}"
AUDIENCE="${AUDIENCE:?set AUDIENCE}"
SOCK="${WORKLOAD_SOCKET:-/spire-agent/socket/agent.sock}"
PRINT_TOKEN="${PRINT_TOKEN:-false}"
POLL_SECONDS="${POLL_SECONDS:-15}"

echo "[caller] uid=$(id -u) gid=$(id -g)"
echo "[caller] waiting for agent socket: $SOCK"
for i in $(seq 1 90); do
  if [ -S "$SOCK" ]; then
    break
  fi
  sleep 1
done

if [ ! -S "$SOCK" ]; then
  echo "[caller] ERROR: agent socket not found at $SOCK"
  exit 1
fi

while true; do
  echo "[caller] fetching JWT-SVID (aud=$AUDIENCE)"
  JWT="$(spire-agent api fetch jwt -socketPath "$SOCK" -audience "$AUDIENCE" | tr -d '\r\n')"

  if [ "$PRINT_TOKEN" = "true" ]; then
    echo "[caller] JWT-SVID: $JWT"
  fi

  echo "[caller] calling API Gateway: $API_URL"
  curl -sS -i -H "Authorization: Bearer $JWT" "$API_URL" || true
  echo
  echo "[caller] sleeping ${POLL_SECONDS}s..."
  sleep "$POLL_SECONDS"
done
