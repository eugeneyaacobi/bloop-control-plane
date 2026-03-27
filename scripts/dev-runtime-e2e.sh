#!/usr/bin/env bash
set -euo pipefail

CP_COMPOSE="/root/.openclaw/worktree-bloop-control-plane-integration/deploy/compose/dev-full.yml"
RELAY_COMPOSE="/root/.openclaw/workspace/bloop-tunnel/deploy/compose/dev-relay-ingest.yml"
CLIENT_TEMPLATE="/root/.openclaw/workspace/bloop-tunnel/deploy/examples/client.relay-ingest.yaml"
CLIENT_RENDERED="/tmp/bloop-client.relay-ingest.rendered.yaml"

cleanup() {
  rm -f "$CLIENT_RENDERED"
  docker compose -f "$RELAY_COMPOSE" down -v --remove-orphans >/dev/null 2>&1 || true
  docker compose -f "$CP_COMPOSE" down -v --remove-orphans >/dev/null 2>&1 || true
}

if [[ "${1:-}" == "--down" ]]; then
  cleanup
  exit 0
fi

cleanup
trap cleanup EXIT

docker compose -f "$CP_COMPOSE" up -d --build
for i in {1..30}; do
  if curl -fsS http://localhost:38081/healthz >/dev/null 2>&1; then
    break
  fi
  sleep 2
done
curl -fsS http://localhost:38081/healthz >/dev/null

INSTALLATION_RESPONSE=$(curl -fsS -X POST http://localhost:38081/api/runtime/installations \
  -H 'Content-Type: application/json' \
  -d '{"name":"Dev Relay Client","environment":"local-e2e"}')
ENROLLMENT_TOKEN=$(python3 - <<'PY' "$INSTALLATION_RESPONSE"
import json,sys
print(json.loads(sys.argv[1])["enrollment"]["token"])
PY
)
python3 - <<'PY' "$CLIENT_TEMPLATE" "$CLIENT_RENDERED" "$ENROLLMENT_TOKEN"
from pathlib import Path
import sys
src = Path(sys.argv[1]).read_text()
out = src.replace("__REPLACE_AT_RUNTIME__", sys.argv[3])
Path(sys.argv[2]).write_text(out)
PY

export BLOOP_CLIENT_RENDERED_CONFIG="$CLIENT_RENDERED"
docker compose -f "$RELAY_COMPOSE" up -d --build
sleep 18

echo "=== workspace ==="
curl -sS http://localhost:38081/api/customer/workspace

echo
POSTGRES_CID=$(docker compose -f "$CP_COMPOSE" ps -q postgres)
echo "=== runtime_tunnel_snapshots ==="
docker exec "$POSTGRES_CID" psql -U bloop -d bloop_control_plane -c "SELECT tunnel_id, account_id, installation_id, hostname, access_mode, status, degraded, observed_at FROM runtime_tunnel_snapshots ORDER BY observed_at DESC LIMIT 10;"

echo "=== runtime_events ==="
docker exec "$POSTGRES_CID" psql -U bloop -d bloop_control_plane -c "SELECT installation_id, kind, level, message, occurred_at FROM runtime_events ORDER BY occurred_at DESC LIMIT 10;"

echo "=== relay logs ==="
docker compose -f "$RELAY_COMPOSE" logs --no-color bloop-relay --tail=80

echo "=== client logs ==="
docker compose -f "$RELAY_COMPOSE" logs --no-color bloop-client --tail=80

echo "=== control-plane logs ==="
docker compose -f "$CP_COMPOSE" logs --no-color control-plane --tail=80
