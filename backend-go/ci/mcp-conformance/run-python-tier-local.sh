#!/usr/bin/env bash
# Runs the Python reference-client tier (1b) against the test-only dev server
# (services/api-gateway/cmd/mcpconformance-devserver) - no Docker, no database.
# For a real dev stack set MCP_CONF_BASE_URL / MCP_CONF_TOKEN and run
# run_reference_client.py directly instead.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$HERE/../.."
WORK="${WORK_DIR:-$(mktemp -d)}"
ADDR="${MCP_CONF_ADDR:-127.0.0.1:8099}"
TOKEN="${MCP_CONF_TOKEN:-conformance-token}"

(cd "$ROOT/services/api-gateway" && go build -o "$WORK/devserver" ./cmd/mcpconformance-devserver)
python3 -m venv "$WORK/venv"
"$WORK/venv/bin/pip" install --quiet -r "$HERE/requirements.txt"

MCP_CONF_ADDR="$ADDR" MCP_CONF_TOKEN="$TOKEN" "$WORK/devserver" >"$WORK/devserver.log" 2>&1 &
SERVER_PID=$!
trap 'kill "$SERVER_PID" 2>/dev/null || true' EXIT
for _ in $(seq 1 50); do curl -fsS "http://$ADDR/.well-known/oauth-protected-resource" >/dev/null 2>&1 && break; sleep 0.2; done

MCP_CONF_BASE_URL="http://$ADDR" MCP_CONF_TOKEN="$TOKEN" PYTHONDONTWRITEBYTECODE=1 "$WORK/venv/bin/python" "$HERE/run_reference_client.py"
