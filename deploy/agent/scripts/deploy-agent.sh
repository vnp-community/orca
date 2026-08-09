#!/usr/bin/env bash
# ══════════════════════════════════════════════════════════════════════════════
# deploy-agent.sh — Deploy/update Orca Agent Daemon to a dev server
#
# Usage:
#   ./deploy-agent.sh <target-host> <dev-server-id> [options]
#
# Options:
#   --user <ssh-user>     SSH user (default: ubuntu)
#   --key  <ssh-key>      SSH identity file (default: ~/.ssh/id_ed25519)
#   --orca-url <url>      Orca WS URL (default: wss://b15.openledger.vn/agent)
#   --token-api <url>     Token API (default: http://172.20.2.39:6769/api/agent-token)
#   --api-secret <secret> Bearer secret for POST /api/agent-token — must match
#                         ORCA_AGENT_API_SECRET on the Orca Server (default:
#                         $ORCA_AGENT_API_SECRET env var, e.g. from deploy/agent/.env)
#   --work-dir <path>     Working directory on target (default: /srv/projects)
#   --name <label>        Human name for Dev Server (default: <dev-server-id>)
#   --agent-js <path>     Local agent.js to upload
#   --dry-run             Print plan without executing
#
# Auto-reconnect mechanism:
#   - Server restart: connection drops → systemd restarts agent (StartLimitBurst=0)
#   - Agent crash: same flow
#   - start.sh uses exponential backoff for token fetch (5s → 60s max)
# ══════════════════════════════════════════════════════════════════════════════

set -euo pipefail

TARGET_HOST="${1:-}"
DEV_SERVER_ID="${2:-}"
shift 2 2>/dev/null || true

SSH_USER="ubuntu"
SSH_KEY="$HOME/.ssh/id_ed25519"
ORCA_WS_URL="wss://b15.openledger.vn/agent"
ORCA_TOKEN_API="http://172.20.2.39:6769/api/agent-token"
# Why: POST /api/agent-token requires `Authorization: Bearer <secret>` matching
# ORCA_AGENT_API_SECRET on the Orca Server (see backend/src/server/agent-token-routes.ts).
# Defaulting to the env var lets `source deploy/agent/.env` supply it without
# putting the secret on the command line / shell history.
API_SECRET="${ORCA_AGENT_API_SECRET:-}"
WORK_DIR="/srv/projects"
SERVER_NAME=""
AGENT_JS_LOCAL=""
DRY_RUN=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --user)        SSH_USER="$2";        shift 2 ;;
    --key)         SSH_KEY="$2";         shift 2 ;;
    --orca-url)    ORCA_WS_URL="$2";    shift 2 ;;
    --token-api)   ORCA_TOKEN_API="$2"; shift 2 ;;
    --api-secret)  API_SECRET="$2";      shift 2 ;;
    --work-dir)    WORK_DIR="$2";        shift 2 ;;
    --name)        SERVER_NAME="$2";     shift 2 ;;
    --agent-js)    AGENT_JS_LOCAL="$2";  shift 2 ;;
    --dry-run)     DRY_RUN=true;         shift   ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

[ -z "$TARGET_HOST" ]   && { echo "ERROR: target-host required"; exit 1; }
[ -z "$DEV_SERVER_ID" ] && { echo "ERROR: dev-server-id required"; exit 1; }
[ -z "$SERVER_NAME" ]   && SERVER_NAME="$DEV_SERVER_ID"
# Why: a missing/wrong secret used to fail silently — start.sh would retry the
# token fetch forever (401) and the agent would never come up. Fail the deploy
# up front instead (see incident: dev-01 deploy 2026-08-09).
[ -z "$API_SECRET" ] && { echo "ERROR: --api-secret required (or set ORCA_AGENT_API_SECRET — e.g. 'source deploy/agent/.env')"; exit 1; }

SSH_OPTS="-o StrictHostKeyChecking=accept-new -o ConnectTimeout=10 -i $SSH_KEY"
SCP_CMD="scp $SSH_OPTS"
SSH_CMD="ssh $SSH_OPTS $SSH_USER@$TARGET_HOST"
SERVICE_NAME="orca-agent-${DEV_SERVER_ID}"
REMOTE_DIR="/home/$SSH_USER/orca-agent"
LOG_FILE="$REMOTE_DIR/logs/agent-${DEV_SERVER_ID}.log"

if [ -z "$AGENT_JS_LOCAL" ]; then
  SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
  AGENT_JS_LOCAL="$REPO_ROOT/agent/out/agent.js"
  [ ! -f "$AGENT_JS_LOCAL" ] && AGENT_JS_LOCAL="$REPO_ROOT/deploy/agent/agent.js"
fi

echo "╔══════════════════════════════════════════════════════════╗"
echo "║         Orca Agent Deploy                                ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo "  Target:       $SSH_USER@$TARGET_HOST"
echo "  DevServerId:  $DEV_SERVER_ID"
echo "  Service:      $SERVICE_NAME.service"
echo "  Token API:    $ORCA_TOKEN_API"
echo "  API Secret:   ${API_SECRET:0:6}... (set)"
echo ""
if $DRY_RUN; then echo "[dry-run] Done."; exit 0; fi

# 1. Prepare dirs
echo "[1/5] Preparing remote directories..."
$SSH_CMD "mkdir -p $REMOTE_DIR/logs && mkdir -p $WORK_DIR"

# 2. Upload agent.js
echo "[2/5] Uploading agent.js..."
if [ -f "$AGENT_JS_LOCAL" ]; then
  $SCP_CMD "$AGENT_JS_LOCAL" "$SSH_USER@$TARGET_HOST:$REMOTE_DIR/agent.js"
  echo "  ✓ Uploaded $AGENT_JS_LOCAL"
else
  echo "  ⚠  agent.js not found locally — using existing on remote"
fi

# 3. Write start.sh on remote (avoid heredoc quoting issues with substitution)
echo "[3/5] Writing start-${DEV_SERVER_ID}.sh..."
$SSH_CMD bash << SSHEOF
cat > '$REMOTE_DIR/start-${DEV_SERVER_ID}.sh' << 'STARTEOF'
#!/usr/bin/env bash
# Orca Agent Starter — AUTO-GENERATED, do not edit manually
# Regenerate: ./deploy/agent/scripts/deploy-agent.sh $TARGET_HOST $DEV_SERVER_ID
exec >> "$LOG_FILE" 2>&1
set -uo pipefail
TS() { date -u +%FT%TZ; }
echo "[\$(TS)] ══ Agent starting (${DEV_SERVER_ID}) ══"
RETRY=0; MAX_RETRY=10; WAIT=5; API_RESP=""
while [ \$RETRY -lt \$MAX_RETRY ]; do
  API_RESP=\$(curl -sf --max-time 10 -X POST \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${API_SECRET}" \
    -d '{"devServerId":"${DEV_SERVER_ID}","name":"${SERVER_NAME}","ttl":600}' \
    "${ORCA_TOKEN_API}" 2>/dev/null) && break || true
  RETRY=\$((RETRY+1))
  echo "[\$(TS)] Token fetch failed (attempt \$RETRY/\$MAX_RETRY). Wait \${WAIT}s..."
  sleep \$WAIT; WAIT=\$(( WAIT*2 )); [ \$WAIT -gt 60 ] && WAIT=60
done
[ -z "\${API_RESP:-}" ] && { echo "[\$(TS)] FATAL: Cannot reach Orca Server"; exit 1; }
NEW_TOKEN=\$(echo "\$API_RESP" | python3 -c "import json,sys; print(json.load(sys.stdin)['token'])" 2>/dev/null || true)
[ -z "\${NEW_TOKEN:-}" ] && { echo "[\$(TS)] FATAL: No token in: \$API_RESP"; exit 1; }
echo "[\$(TS)] Token OK. Starting agent..."
exec env ORCA_URL="${ORCA_WS_URL}" AGENT_TOKEN="\$NEW_TOKEN" DEV_SERVER_ID="${DEV_SERVER_ID}" \
  MODE="direct-websocket" AGENT_WORK_DIR="${WORK_DIR}" \
  ORCA_AGENT_API_SECRET="${API_SECRET}" ORCA_HTTP_URL="${ORCA_TOKEN_API%/api/agent-token}" \
  HOME="/home/${SSH_USER}" PATH="\$PATH:/home/${SSH_USER}/.local/bin" \
  node "$REMOTE_DIR/agent.js"
STARTEOF
chmod +x '$REMOTE_DIR/start-${DEV_SERVER_ID}.sh'
SSHEOF

# 4. Install systemd service
echo "[4/5] Installing systemd service..."
$SSH_CMD sudo tee /etc/systemd/system/${SERVICE_NAME}.service > /dev/null << SVCEOF
[Unit]
Description=Orca Dev Server Agent (${DEV_SERVER_ID})
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SSH_USER}
WorkingDirectory=${REMOTE_DIR}
Environment=NODE_ENV=production
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/home/${SSH_USER}/.local/bin
ExecStart=/bin/bash ${REMOTE_DIR}/start-${DEV_SERVER_ID}.sh

# Auto-reconnect: StartLimitBurst=0 = unlimited restarts (no timeout)
# Why: server restarts cause rapid connection drops; we never want systemd
# to give up — agent will back off via sleep in start.sh.
Restart=always
RestartSec=15
StartLimitBurst=0

TimeoutStopSec=10
MemoryMax=512M

[Install]
WantedBy=multi-user.target
SVCEOF

$SSH_CMD sudo systemctl daemon-reload
$SSH_CMD sudo systemctl enable ${SERVICE_NAME}.service
$SSH_CMD sudo systemctl stop ${SERVICE_NAME}.service 2>/dev/null || true
sleep 2

# 5. Start and verify
echo "[5/5] Starting service..."
$SSH_CMD sudo systemctl reset-failed ${SERVICE_NAME}.service 2>/dev/null || true
$SSH_CMD sudo systemctl start ${SERVICE_NAME}.service
sleep 8

STATUS=$($SSH_CMD systemctl is-active ${SERVICE_NAME}.service 2>&1 || true)
echo ""
if [ "$STATUS" = "active" ]; then
  echo "  ✅ Service $SERVICE_NAME: active"
else
  echo "  ⚠️  Service status: $STATUS"
fi

echo ""
echo "  📋 Recent logs:"
$SSH_CMD "tail -15 $LOG_FILE" 2>/dev/null || echo "  (no log yet)"

echo ""
echo "═══════════════════════════════════════════════════════════"
echo "  ✅ Done: $SERVICE_NAME on $TARGET_HOST"
echo "  Logs: ssh $SSH_USER@$TARGET_HOST 'tail -f $LOG_FILE'"
echo "═══════════════════════════════════════════════════════════"
