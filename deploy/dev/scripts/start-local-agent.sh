#!/usr/bin/env bash
# ============================================================
# start-local-agent.sh — run the Dev Server Agent on THIS machine against the local docker stack
# ============================================================
# Why this exists: the e2e suite (tests/stages/e2e/orca-flows) needs a connected dev server, and the
# agent's AI calls should use this machine's logged-in Claude Code session, never an API key. This script
# starts the agent with the API-key variables removed from its environment so that cannot happen by accident.
#
# Usage:
#   node agent/build.mjs                                  # build agent/out/agent.js first
#   ./deploy/dev/scripts/start-local-agent.sh             # DEV_SERVER_ID defaults to dev-local-mac
#   DEV_SERVER_ID=my-box AGENT_WORK_DIR=/path/to/git/repo ./deploy/dev/scripts/start-local-agent.sh
# Log: $AGENT_LOG (default /tmp/orca-local-agent.log). The token is fetched fresh and never printed.
# Stop: pkill -f agent/out/agent.js
# ============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
cd "${REPO_ROOT}"

[ -f agent/out/agent.js ] || { echo "agent/out/agent.js is missing: run 'node agent/build.mjs' first" >&2; exit 1; }
[ -f deploy/dev/.env ] || { echo "deploy/dev/.env is missing: see deploy/dev/README.md" >&2; exit 1; }

set -a
# shellcheck disable=SC1091
. deploy/dev/.env >/dev/null 2>&1
set +a

DEV_SERVER_ID="${DEV_SERVER_ID:-dev-local-mac}"
GATEWAY_PORT="${API_GATEWAY_PUBLIC_PORT:-8081}"
AGENT_LOG="${AGENT_LOG:-/tmp/orca-local-agent.log}"
AGENT_WORK_DIR="${AGENT_WORK_DIR:-${REPO_ROOT}}"

pkill -f "agent/out/agent.js" 2>/dev/null || true
sleep 1

token="$(curl -s -m 8 -X POST "http://localhost:${GATEWAY_PORT}/api/agent-token" \
  -H 'Content-Type: application/json' -H "Authorization: Bearer ${ORCA_AGENT_API_SECRET:?ORCA_AGENT_API_SECRET missing in deploy/dev/.env}" \
  -d "{\"devServerId\":\"${DEV_SERVER_ID}\",\"name\":\"Dev Server (this machine)\",\"ttl\":3600}" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])')"

env -u ANTHROPIC_API_KEY -u OPENAI_API_KEY -u GOOGLE_API_KEY \
  MODE=direct-websocket ORCA_URL="ws://localhost:${GATEWAY_PORT}/agent" AGENT_TOKEN="${token}" \
  DEV_SERVER_ID="${DEV_SERVER_ID}" AGENT_WORK_DIR="${AGENT_WORK_DIR}" LOG_LEVEL=info \
  nohup node agent/out/agent.js > "${AGENT_LOG}" 2>&1 &
echo "agent started (pid $!), log: ${AGENT_LOG}"
