#!/usr/bin/env bash
# ============================================================
# deploy-agents.sh — Deploy Orca Agent lên nhiều Dev Servers
# ============================================================
# Tương thích bash 3.2+ (macOS default shell).
#
# Usage:
#   bash deploy/agent/scripts/deploy-agents.sh                  # deploy tất cả
#   bash deploy/agent/scripts/deploy-agents.sh --server DEV01  # 1 server
#   bash deploy/agent/scripts/deploy-agents.sh --list           # liệt kê
#   bash deploy/agent/scripts/deploy-agents.sh --status         # kiểm tra
#   bash deploy/agent/scripts/deploy-agents.sh --stop           # dừng tất cả
#   bash deploy/agent/scripts/deploy-agents.sh --logs DEV01    # xem logs
#   bash deploy/agent/scripts/deploy-agents.sh --build          # build agent.js
#   bash deploy/agent/scripts/deploy-agents.sh --help
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="${SCRIPT_DIR}/.."
REPO_ROOT="$(cd "${DEPLOY_DIR}/../.." && pwd)"

# ── Màu sắc ──────────────────────────────────────────────────
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'
RED='\033[0;31m'; BOLD='\033[1m'; CYAN='\033[0;36m'; NC='\033[0m'
GRAY='\033[0;90m'

log()     { echo -e "${GREEN}[agents]${NC} $*"; }
warn()    { echo -e "${YELLOW}[agents] WARN:${NC} $*"; }
err()     { echo -e "${RED}[agents] ERROR:${NC} $*"; exit 1; }
section() { echo -e "\n${BOLD}${BLUE}══ $* ══${NC}"; }

# ── Load .env ──────────────────────────────────────────────────
ENV_FILE="${DEPLOY_DIR}/.env"
[ -f "${ENV_FILE}" ] || err ".env không tồn tại tại ${ENV_FILE}."
# shellcheck disable=SC1090
source "${ENV_FILE}"

# ── Load .env.agents ──────────────────────────────────────────
AGENTS_ENV_FILE="${DEPLOY_DIR}/.env.agents"
if [ ! -f "${AGENTS_ENV_FILE}" ]; then
    warn ".env.agents không tồn tại. Tạo từ template:"
    echo -e "  ${CYAN}cp ${DEPLOY_DIR}/.env.agents.example ${AGENTS_ENV_FILE}${NC}"
    exit 1
fi

# ── Defaults ──────────────────────────────────────────────────
SSH_KEY="${AGENT_SSH_KEY:-${SERVER_KEY:-${HOME}/.ssh/id_ed25519}}"
ORCA_HTTP_HOST="${SERVER_HOST:-172.20.2.39}"
ORCA_HTTP_PORT_VAL="${ORCA_HTTP_PORT:-6769}"
AGENT_ORCA_URL="${AGENT_ORCA_URL:-wss://b15.openledger.vn/agent}"
AGENT_DEPLOY_TIMEOUT="${AGENT_DEPLOY_TIMEOUT:-120}"
AUTH_HDR=""
if [ -n "${ORCA_AGENT_API_SECRET:-}" ]; then
    AUTH_HDR="Authorization: Bearer ${ORCA_AGENT_API_SECRET}"
else
    AUTH_HDR="X-Orca-Admin: 1"
fi
AGENT_JS_PATH="${REPO_ROOT}/agent/out/agent.js"

# ═══════════════════════════════════════════════════════════════
# Server list helpers (bash 3.2 compat — no declare -A)
# ═══════════════════════════════════════════════════════════════

# Lấy giá trị AGENT_SERVER_<NAME> từ file
get_server_entry() {
    local name="$1"
    local line
    line=$(grep -E "^AGENT_SERVER_${name}=" "${AGENTS_ENV_FILE}" | head -1) || true
    if [ -z "${line}" ]; then echo ""; return; fi
    local val="${line#*=}"
    val="${val//\"/}"
    val="${val//\'/}"
    echo "${val}"
}

# Trả về danh sách tên server (NAME) từ file, sorted
list_server_names() {
    grep -E "^AGENT_SERVER_[A-Z0-9_]+=" "${AGENTS_ENV_FILE}" \
        | sed 's/^AGENT_SERVER_//' \
        | sed 's/=.*//' \
        | tr -d '\r' \
        | sort
}

# Parse "user@host[:port] [label] [work_dir]" → "user host port label work_dir"
parse_server_entry() {
    local name="$1"
    local entry
    entry="$(get_server_entry "${name}")"
    [ -z "${entry}" ] && err "Không tìm thấy AGENT_SERVER_${name} trong ${AGENTS_ENV_FILE}"

    local conn label work_dir
    conn="$(echo "${entry}" | awk '{print $1}')"
    label="$(echo "${entry}" | awk '{print $2}')"
    work_dir="$(echo "${entry}" | awk '{print $3}')"

    local ssh_user ssh_host ssh_port
    if echo "${conn}" | grep -qE "^[^@]+@[^:]+:[0-9]+$"; then
        ssh_user="${conn%%@*}"
        local hostport="${conn#*@}"
        ssh_host="${hostport%%:*}"
        ssh_port="${hostport##*:}"
    else
        ssh_user="${conn%%@*}"
        ssh_host="${conn#*@}"
        ssh_port="22"
    fi

    [ -z "${label}" ]    && label="${name}"
    [ -z "${work_dir}" ] && work_dir="/srv/projects"
    echo "${ssh_user} ${ssh_host} ${ssh_port} ${label} ${work_dir}"
}

# ── Argument parsing ──────────────────────────────────────────
ACTION="deploy"
FILTER_SERVER=""
BUILD_FIRST=false

while [ $# -gt 0 ]; do
    case "$1" in
        --list)    ACTION="list" ;;
        --status)  ACTION="status" ;;
        --stop)    ACTION="stop" ;;
        --build)   BUILD_FIRST=true ;;
        --server)
            shift
            [ $# -gt 0 ] || err "--server cần tên server. Ví dụ: --server DEV01"
            FILTER_SERVER="$1"
            ;;
        --logs)
            ACTION="logs"
            shift
            [ $# -gt 0 ] || err "--logs cần tên server. Ví dụ: --logs DEV01"
            FILTER_SERVER="$1"
            ;;
        --help|-h) ACTION="help" ;;
        *) warn "Unknown option: $1" ;;
    esac
    shift
done

# Uppercase FILTER_SERVER
[ -n "${FILTER_SERVER}" ] && FILTER_SERVER="$(echo "${FILTER_SERVER}" | tr '[:lower:]' '[:upper:]')"

# ── Help ───────────────────────────────────────────────────────
if [ "${ACTION}" = "help" ]; then
    cat << 'HELP'
Usage: deploy-agents.sh [OPTIONS]

OPTIONS:
  (no option)          Deploy agent lên tất cả servers trong .env.agents
  --server <NAME>      Chỉ thao tác với server NAME (vd: DEV01)
  --list               Liệt kê tất cả servers trong .env.agents
  --status             Kiểm tra trạng thái agent trên tất cả servers
  --stop               Dừng agent service trên tất cả servers
  --logs <NAME>        Xem logs realtime của server NAME
  --build              Build agent.js (no Electron, ~3s) trước khi deploy
  --help               Hiển thị help này

.env.agents format:
  AGENT_SERVER_<NAME>="user@host[:port] [label] [work_dir]"

  Examples:
    AGENT_SERVER_DEV01="ubuntu@172.20.2.31 dev-01 /srv/projects"
    AGENT_SERVER_DEV02="ubuntu@172.20.2.32:2222 dev-ai /home/ubuntu"
HELP
    exit 0
fi

# ── Load & filter servers ──────────────────────────────────────
ALL_NAMES="$(list_server_names)"
[ -z "${ALL_NAMES}" ] && err "Không tìm thấy AGENT_SERVER_* nào trong ${AGENTS_ENV_FILE}."

TARGET_NAMES=""
for name in ${ALL_NAMES}; do
    if [ -z "${FILTER_SERVER}" ] || [ "${name}" = "${FILTER_SERVER}" ]; then
        TARGET_NAMES="${TARGET_NAMES} ${name}"
    fi
done
TARGET_NAMES="${TARGET_NAMES# }"

[ -z "${TARGET_NAMES}" ] && err "Không tìm thấy server '${FILTER_SERVER}' trong .env.agents."
SERVER_COUNT=$(echo "${TARGET_NAMES}" | wc -w | tr -d ' ')

# ── ACTION: list ──────────────────────────────────────────────
if [ "${ACTION}" = "list" ]; then
    section "Dev Servers (.env.agents)"
    printf "%-12s %-28s %-18s %s\n" "NAME" "CONNECTION" "LABEL" "WORK_DIR"
    printf "%-12s %-28s %-18s %s\n" "────────────" "────────────────────────────" "──────────────────" "───────────"
    for name in ${TARGET_NAMES}; do
        read -r ssh_user ssh_host ssh_port label work_dir <<< "$(parse_server_entry "${name}")"
        printf "%-12s %-28s %-18s %s\n" "${name}" "${ssh_user}@${ssh_host}:${ssh_port}" "${label}" "${work_dir}"
    done
    echo ""
    exit 0
fi

# ── ACTION: logs ──────────────────────────────────────────────
if [ "${ACTION}" = "logs" ]; then
    name="$(echo "${TARGET_NAMES}" | awk '{print $1}')"
    read -r ssh_user ssh_host ssh_port label work_dir <<< "$(parse_server_entry "${name}")"
    SSH_OPTS="-i ${SSH_KEY} -p ${ssh_port} -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10"
    log "Xem logs agent trên ${name} (${ssh_user}@${ssh_host}) label=${label}..."
    # shellcheck disable=SC2086
    ssh ${SSH_OPTS} "${ssh_user}@${ssh_host}" \
        "tail -f /home/${ssh_user}/orca-agent/logs/agent-${label}.log 2>/dev/null \
         || journalctl -u orca-agent-${label} -f 2>/dev/null \
         || journalctl -u orca-agent-direct -f 2>/dev/null \
         || echo 'Không tìm thấy logs'"
    exit 0
fi

# ── ACTION: status ────────────────────────────────────────────
if [ "${ACTION}" = "status" ]; then
    section "Agent Status"
    for name in ${TARGET_NAMES}; do
        read -r ssh_user ssh_host ssh_port label work_dir <<< "$(parse_server_entry "${name}")"
        SSH_OPTS="-i ${SSH_KEY} -p ${ssh_port} -o StrictHostKeyChecking=accept-new -o ConnectTimeout=8"
        echo -e "\n${BOLD}[${name}]${NC} ${CYAN}${ssh_user}@${ssh_host}${NC} (${label})"
        # shellcheck disable=SC2086
        ssh ${SSH_OPTS} "${ssh_user}@${ssh_host}" \
            "sudo systemctl status orca-agent-${label} --no-pager -l 2>/dev/null | head -20 \
             || sudo systemctl status orca-agent-direct --no-pager -l 2>/dev/null | head -20 \
             || echo 'Service không tồn tại hoặc chưa deploy'" 2>/dev/null \
            || echo -e "  ${RED}✗ Không kết nối được${NC}"
    done
    echo ""
    exit 0
fi

# ── ACTION: stop ──────────────────────────────────────────────
if [ "${ACTION}" = "stop" ]; then
    section "Dừng Agent"
    for name in ${TARGET_NAMES}; do
        read -r ssh_user ssh_host ssh_port label work_dir <<< "$(parse_server_entry "${name}")"
        SSH_OPTS="-i ${SSH_KEY} -p ${ssh_port} -o StrictHostKeyChecking=accept-new -o ConnectTimeout=8"
        echo -e "\n${BOLD}[${name}]${NC} ${CYAN}${ssh_user}@${ssh_host}${NC}..."
        # shellcheck disable=SC2086
        ssh ${SSH_OPTS} "${ssh_user}@${ssh_host}" \
            "sudo systemctl stop orca-agent-${label} 2>/dev/null && echo '✅ Stopped' \
             || sudo systemctl stop orca-agent-direct 2>/dev/null && echo '✅ Stopped (legacy)' \
             || echo '⚠️  Service không chạy'" 2>/dev/null \
            || echo -e "  ${RED}✗ Không kết nối được${NC}"
    done
    echo ""
    exit 0
fi

# ══════════════════════════════════════════════════════════════
# ACTION: deploy
# ══════════════════════════════════════════════════════════════

# ── Build agent.js ────────────────────────────────────────────
do_build_agent() {
    section "Build agent.js (Node.js only, no Electron)"
    export NVM_DIR="${HOME}/.nvm"
    # shellcheck disable=SC1091
    [ -s "${NVM_DIR}/nvm.sh" ] && source "${NVM_DIR}/nvm.sh" && nvm use 24 2>/dev/null || true
    log "Chạy: node agent/build.mjs"
    (cd "${REPO_ROOT}" && node agent/build.mjs) \
        || err "Build agent.js thất bại!"
    log "✅ Build xong"
}

[ "${BUILD_FIRST}" = "true" ] && do_build_agent

if [ ! -f "${AGENT_JS_PATH}" ]; then
    warn "agent.js chưa tồn tại. Tự động build..."
    do_build_agent
fi

AGENT_JS_SIZE=$(du -sh "${AGENT_JS_PATH}" | cut -f1)
log "agent.js: ${AGENT_JS_PATH} (${AGENT_JS_SIZE})"

# ── Kế hoạch ──────────────────────────────────────────────────
section "Kế hoạch deploy Agent"
echo -e "  Orca server:  ${CYAN}${ORCA_HTTP_HOST}:${ORCA_HTTP_PORT_VAL}${NC}"
echo -e "  Agent URL:    ${CYAN}${AGENT_ORCA_URL}${NC}"
echo -e "  SSH key:      ${GRAY}${SSH_KEY}${NC}"
echo -e "  Timeout:      ${AGENT_DEPLOY_TIMEOUT}s mỗi server"
echo -e "  Servers (${SERVER_COUNT}):"
for name in ${TARGET_NAMES}; do
    read -r ssh_user ssh_host ssh_port label work_dir <<< "$(parse_server_entry "${name}")"
    echo -e "    ${BOLD}${name}${NC}  →  ${ssh_user}@${ssh_host}:${ssh_port}  [${label}]  ${GRAY}${work_dir}${NC}"
done
echo ""

# ── Hàm deploy 1 server ───────────────────────────────────────
deploy_one_server() {
    local name="$1"
    local log_prefix="[${name}]"

    read -r ssh_user ssh_host ssh_port label work_dir <<< "$(parse_server_entry "${name}")"

    local SSH_OPTS="-i ${SSH_KEY} -p ${ssh_port} -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10 -o ServerAliveInterval=30"
    local agent_dir="/home/${ssh_user}/orca-agent"

    echo "${log_prefix} Bắt đầu → ${ssh_user}@${ssh_host}:${ssh_port} [${label}]"

    # 1. Test SSH
    # shellcheck disable=SC2086
    ssh ${SSH_OPTS} "${ssh_user}@${ssh_host}" "echo '${log_prefix} SSH OK'" 2>/dev/null \
        || { echo "${log_prefix} ✗ Không kết nối SSH được"; return 1; }

    # 2. Detect node binary path trên remote
    #    Systemd không load .bashrc/.profile/.nvm — phải hardcode full path trong start.sh
    echo "${log_prefix} Detecting node binary..."
    # shellcheck disable=SC2086
    NODE_BIN=$(ssh ${SSH_OPTS} "${ssh_user}@${ssh_host}" '
        # Thử các vị trí phổ biến trước
        for p in /usr/bin/node /usr/local/bin/node /opt/node/bin/node; do
            [ -x "${p}" ] && echo "${p}" && exit 0
        done
        # NVM: lấy version mới nhất
        NVM_BASE="${HOME}/.nvm/versions/node"
        if [ -d "${NVM_BASE}" ]; then
            LATEST=$(ls -1 "${NVM_BASE}" 2>/dev/null | sort -t. -k1,1n -k2,2n -k3,3n | tail -1)
            if [ -n "${LATEST}" ] && [ -x "${NVM_BASE}/${LATEST}/bin/node" ]; then
                echo "${NVM_BASE}/${LATEST}/bin/node"
                exit 0
            fi
        fi
        # Fallback: which
        which node 2>/dev/null || true
    ' 2>/dev/null | tail -1 | tr -d '\r')

    if [ -z "${NODE_BIN}" ]; then
        echo "${log_prefix} node không tìm thấy → tự động cài Node.js 22 LTS..."
        # shellcheck disable=SC2086
        ssh ${SSH_OPTS} "${ssh_user}@${ssh_host}" '
            set -e
            echo "[install-node] Cài Node.js 22 LTS via NodeSource..."
            curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
            sudo apt-get install -y nodejs
            echo "[install-node] Done: $(node --version)"
        ' 2>&1 || { echo "${log_prefix} ✗ Cài Node.js thất bại. Cài thủ công rồi chạy lại."; return 1; }

        # Detect lại sau khi cài
        # shellcheck disable=SC2086
        NODE_BIN=$(ssh ${SSH_OPTS} "${ssh_user}@${ssh_host}" \
            'for p in /usr/bin/node /usr/local/bin/node; do [ -x "${p}" ] && echo "${p}" && exit 0; done' \
            2>/dev/null | tail -1 | tr -d '\r')

        [ -z "${NODE_BIN}" ] && { echo "${log_prefix} ✗ Vẫn không tìm thấy node sau khi cài."; return 1; }
        echo "${log_prefix} ✓ node installed: ${NODE_BIN}"
    fi
    echo "${log_prefix} ✓ node: ${NODE_BIN}"

    # 3. Upload agent.js
    echo "${log_prefix} Uploading agent.js..."
    # shellcheck disable=SC2086
    ssh ${SSH_OPTS} "${ssh_user}@${ssh_host}" "mkdir -p ${agent_dir}/logs"
    # shellcheck disable=SC2086
    scp -i "${SSH_KEY}" -P "${ssh_port}" \
        -o StrictHostKeyChecking=accept-new \
        "${AGENT_JS_PATH}" \
        "${ssh_user}@${ssh_host}:${agent_dir}/agent.js"
    echo "${log_prefix} ✓ agent.js uploaded"

    # 4. Install start.sh + systemd service trên remote
    local AUTH_HDR_VAL="${AUTH_HDR}"
    local ORCA_HTTP_HOST_V="${ORCA_HTTP_HOST}"
    local ORCA_HTTP_PORT_V="${ORCA_HTTP_PORT_VAL}"
    local ORCA_URL_V="${AGENT_ORCA_URL}"
    local LABEL_V="${label}"
    local WORK_DIR_V="${work_dir}"
    local SSH_USER_V="${ssh_user}"
    local NODE_BIN_V="${NODE_BIN}"
    # FIX BUG-DS-AWS: the generated start.sh template below embeds
    # ${ORCA_AGENT_API_SECRET} directly into the agent's exec env so
    # AgentTokenManager can renew its own token on auth failure. Without
    # forwarding it here, the remote heredoc only ever sees AUTH_HEADER
    # (used for the initial curl), so the agent always launches with an
    # empty secret and silently falls back to the no-renewal legacy path.
    local API_SECRET_V="${ORCA_AGENT_API_SECRET:-}"

    echo "${log_prefix} Installing systemd service (orca-agent-${LABEL_V})..."
    # shellcheck disable=SC2086
    ssh ${SSH_OPTS} "${ssh_user}@${ssh_host}" \
        "AGENT_DIR='${agent_dir}' \
         ORCA_HTTP_HOST='${ORCA_HTTP_HOST_V}' \
         ORCA_HTTP_PORT='${ORCA_HTTP_PORT_V}' \
         ORCA_URL='${ORCA_URL_V}' \
         DEV_LABEL='${LABEL_V}' \
         WORK_DIR='${WORK_DIR_V}' \
         SSH_USER='${SSH_USER_V}' \
         AUTH_HEADER='${AUTH_HDR_VAL}' \
         ORCA_AGENT_API_SECRET='${API_SECRET_V}' \
         NODE_BIN='${NODE_BIN_V}' \
         bash -s" << 'REMOTE_SCRIPT'
mkdir -p "${AGENT_DIR}/logs"

# ── start.sh: wrapper tự lấy token mỗi lần restart ──────────
# Log path dùng label để mỗi agent có file riêng
LOG_FILE="${AGENT_DIR}/logs/agent-${DEV_LABEL}.log"

cat > "${AGENT_DIR}/start-${DEV_LABEL}.sh" << WRAPPER
#!/usr/bin/env bash
# Orca Agent Starter — ${DEV_LABEL}
# node: ${NODE_BIN}  (detected at deploy time, full path for systemd)
exec >> "${AGENT_DIR}/logs/agent-${DEV_LABEL}.log" 2>&1

set -uo pipefail

TS() { date -u +%FT%TZ; }

echo "[\$(TS)] ══ Agent starting (${DEV_LABEL}) ══"
echo "[\$(TS)] node:    ${NODE_BIN}"
echo "[\$(TS)] agent:   ${AGENT_DIR}/agent.js"
echo "[\$(TS)] orca:    http://${ORCA_HTTP_HOST}:${ORCA_HTTP_PORT}/api/agent-token"

[ -x "${NODE_BIN}" ]             || { echo "[\$(TS)] FATAL: node not executable: ${NODE_BIN}"; exit 1; }
[ -f "${AGENT_DIR}/agent.js" ]   || { echo "[\$(TS)] FATAL: agent.js missing: ${AGENT_DIR}/agent.js"; exit 1; }

echo "[\$(TS)] Fetching agent token..."

# Exponential backoff: 5s → 10s → 20s → 40s → 60s (max)
RETRY=0; MAX_RETRY=10; WAIT=5; API_RESP=""
while [ \$RETRY -lt \$MAX_RETRY ]; do
  API_RESP=\$(curl -sf --max-time 10 -X POST \\
    -H "Content-Type: application/json" \\
    -H "${AUTH_HEADER}" \\
    -d "{\"devServerId\":\"${DEV_LABEL}\",\"name\":\"${DEV_LABEL}\",\"ttl\":86400,\"permanent\":true}" \\
    "http://${ORCA_HTTP_HOST}:${ORCA_HTTP_PORT}/api/agent-token" 2>/dev/null) && break || true
  RETRY=\$((RETRY+1))
  echo "[\$(TS)] Token fetch failed (attempt \$RETRY/\$MAX_RETRY). Wait \${WAIT}s..."
  sleep \$WAIT; WAIT=\$(( WAIT*2 )); [ \$WAIT -gt 60 ] && WAIT=60
done

if [ -z "\${API_RESP:-}" ]; then
    echo "[\$(TS)] FATAL: Cannot reach Orca Server after \$MAX_RETRY attempts. Exit."
    exit 1
fi

NEW_TOKEN=\$(echo "\${API_RESP}" | python3 -c "import json,sys; print(json.load(sys.stdin)['token'])" 2>/dev/null \
    || echo "\${API_RESP}" | grep -o '"token":"[^"]*"' | cut -d'"' -f4 || true)

if [ -z "\${NEW_TOKEN:-}" ]; then
    echo "[\$(TS)] ERROR: No token in response: \${API_RESP}"
    sleep 10
    exit 1
fi

echo "[\$(TS)] Token OK. Starting agent (mode=direct-websocket)..."
exec env \
  ORCA_URL="${ORCA_URL}" \
  AGENT_TOKEN="\${NEW_TOKEN}" \
  ORCA_AGENT_API_SECRET="${ORCA_AGENT_API_SECRET}" \
  ORCA_HTTP_URL="http://${ORCA_HTTP_HOST}:${ORCA_HTTP_PORT}" \
  DEV_SERVER_ID="${DEV_LABEL}" \
  MODE="direct-websocket" \
  AGENT_WORK_DIR="${WORK_DIR}" \
  HOME="/home/${SSH_USER}" \
  "${NODE_BIN}" "${AGENT_DIR}/agent.js"
WRAPPER
chmod +x "${AGENT_DIR}/start-${DEV_LABEL}.sh"

# ── Systemd service — per-agent name (orca-agent-<label>) ────
# Why StartLimitBurst=0:
#   Orca Server restarts cause rapid connection drops. The agent exits
#   on drop so systemd can reconnect. With burst=5 in 300s, multiple
#   server restarts exhaust the limit and systemd stops retrying.
#   burst=0 = unlimited: systemd will always restart, backoff is
#   handled by start.sh exponential sleep.
SVC_NAME="orca-agent-${DEV_LABEL}"
sudo tee /etc/systemd/system/${SVC_NAME}.service > /dev/null << SERVICE
[Unit]
Description=Orca Dev Server Agent (${DEV_LABEL})
Documentation=https://b15.openledger.vn
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SSH_USER}
WorkingDirectory=${AGENT_DIR}
Environment=NODE_ENV=production
Environment=PATH=/home/${SSH_USER}/.local/bin:/usr/local/bin:/usr/bin:/bin
ExecStart=/bin/bash ${AGENT_DIR}/start-${DEV_LABEL}.sh

# Auto-reconnect: unlimited restarts, 15s cooldown between attempts
# start.sh handles exponential backoff for token fetch failures
Restart=always
RestartSec=15
StartLimitBurst=0

# Why KillMode=process (default is control-group): the agent spawns a
# detached pty-daemon child process that holds live terminal PTYs so they
# survive an agent restart (see src/relay/pty-daemon-server.ts). The default
# control-group KillMode would kill that daemon (and every shell under it)
# along with the agent on every "systemctl restart" — defeating the entire
# point. process scopes every stop/restart signal to only the tracked agent
# PID, leaving the daemon and its PTYs alone.
KillMode=process

TimeoutStopSec=15
MemoryMax=512M

[Install]
WantedBy=multi-user.target
SERVICE

sudo systemctl daemon-reload
sudo systemctl enable ${SVC_NAME}
# Stop legacy service name if running on same machine
sudo systemctl stop orca-agent-direct 2>/dev/null || true
sudo systemctl disable orca-agent-direct 2>/dev/null || true
sudo systemctl stop ${SVC_NAME} 2>/dev/null || true
sudo systemctl reset-failed ${SVC_NAME} 2>/dev/null || true
sudo systemctl start ${SVC_NAME}

# Poll tối đa 20s
waited=0; STATUS="unknown"
while [ "${waited}" -lt 20 ]; do
    STATUS=$(sudo systemctl is-active ${SVC_NAME} 2>/dev/null || echo "unknown")
    [ "${STATUS}" = "active" ] && break
    [ "${STATUS}" = "failed" ] && break
    sleep 1; waited=$((waited + 1))
done

if [ "${STATUS}" = "active" ]; then
    echo "Service ${SVC_NAME}: active ✅"
elif [ "${STATUS}" = "failed" ]; then
    echo "Service ${SVC_NAME}: FAILED ❌"
    tail -20 "${AGENT_DIR}/logs/agent-${DEV_LABEL}.log" 2>/dev/null || \
        sudo journalctl -u ${SVC_NAME} -n 20 --no-pager 2>/dev/null || true
else
    echo "Service ${SVC_NAME}: ${STATUS} (still starting...)"
fi
REMOTE_SCRIPT

    echo "${log_prefix} ✅ Deploy hoàn thành!"
    return 0
}

# ── Deploy song song ───────────────────────────────────────────
section "Deploy Agent (song song)"

TMPDIR_AGENT="/tmp/orca-agents-deploy-$$"
mkdir -p "${TMPDIR_AGENT}"

PIDS=""
for name in ${TARGET_NAMES}; do
    LOG_FILE="${TMPDIR_AGENT}/${name}.log"
    (
        if deploy_one_server "${name}" > "${LOG_FILE}" 2>&1; then
            echo "0" > "${TMPDIR_AGENT}/${name}.exit"
        else
            echo "1" > "${TMPDIR_AGENT}/${name}.exit"
        fi
    ) &
    PID=$!
    PIDS="${PIDS} ${name}:${PID}"
    echo -e "  ${GRAY}→ ${name} (PID ${PID})${NC}"
done
echo ""
log "Đang deploy ${SERVER_COUNT} servers song song (timeout ${AGENT_DEPLOY_TIMEOUT}s)..."

# ── Kết quả ───────────────────────────────────────────────────
section "Kết quả"

FAIL_COUNT=0
for entry in ${PIDS}; do
    name="${entry%%:*}"
    pid="${entry##*:}"
    log_file="${TMPDIR_AGENT}/${name}.log"

    # Chờ với timeout
    waited=0
    while kill -0 "${pid}" 2>/dev/null; do
        sleep 1
        waited=$((waited + 1))
        if [ "${waited}" -ge "${AGENT_DEPLOY_TIMEOUT}" ]; then
            kill "${pid}" 2>/dev/null || true
            echo -e "${RED}[${name}] ✗ TIMEOUT sau ${AGENT_DEPLOY_TIMEOUT}s${NC}"
            FAIL_COUNT=$((FAIL_COUNT + 1))
            pid=""
            break
        fi
    done

    [ -z "${pid}" ] && continue

    exit_code="$(cat "${TMPDIR_AGENT}/${name}.exit" 2>/dev/null || echo "1")"
    if [ "${exit_code}" = "0" ]; then
        echo -e "${GREEN}[${name}] ✅ OK${NC}"
        sed 's/^/  /' "${log_file}" 2>/dev/null || true
    else
        echo -e "${RED}[${name}] ✗ FAILED${NC}"
        sed 's/^/  /' "${log_file}" 2>/dev/null || true
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
done

rm -rf "${TMPDIR_AGENT}"

# ── Summary ───────────────────────────────────────────────────
echo ""
echo "══════════════════════════════════════════════════════════"
if [ "${FAIL_COUNT}" -eq 0 ]; then
    echo -e "✅ ${GREEN}Deploy hoàn thành: ${SERVER_COUNT}/${SERVER_COUNT} servers thành công${NC}"
else
    SUCCESS=$((SERVER_COUNT - FAIL_COUNT))
    echo -e "⚠️  ${YELLOW}Deploy: ${SUCCESS}/${SERVER_COUNT} thành công, ${FAIL_COUNT} thất bại${NC}"
fi
echo "══════════════════════════════════════════════════════════"
echo ""
echo "Lệnh hữu ích:"
echo -e "  Xem status:  ${CYAN}bash $0 --status${NC}"
echo -e "  Xem logs:    ${CYAN}bash $0 --logs <NAME>${NC}"
echo -e "  Dừng tất cả: ${CYAN}bash $0 --stop${NC}"
echo ""

[ "${FAIL_COUNT}" -gt 0 ] && exit 1 || exit 0
