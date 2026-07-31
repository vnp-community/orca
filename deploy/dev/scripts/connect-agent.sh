#!/usr/bin/env bash
# =================================================================
# connect-agent.sh — Sinh token và khởi động kết nối Agent
# =================================================================
#
# Script này:
#   1. Sinh agentToken một lần qua Orca server (direct-websocket mode)
#      hoặc in lệnh setup cho relay-websocket mode
#   2. Deploy agent TypeScript lên dev server (172.20.2.31)
#   3. Khởi động agent với token đúng
#
# Mode 1: direct-websocket (mặc định)
#   Agent kết nối VÀO Orca:  ws://172.20.2.31 → wss://b15.openledger.vn/agent
#
# Mode 2: relay-websocket
#   Orca kết nối VÀO Agent:  Orca → ws://172.20.2.31:6799/orca-relay
#
# Usage:
#   bash deploy/dev/scripts/connect-agent.sh                    # direct-ws (default)
#   bash deploy/dev/scripts/connect-agent.sh --mode relay-ws   # relay-ws mode
#   bash deploy/dev/scripts/connect-agent.sh --deploy          # deploy agent trước rồi connect
#   bash deploy/dev/scripts/connect-agent.sh --start           # chỉ khởi động agent đã deploy
#   bash deploy/dev/scripts/connect-agent.sh --status          # kiểm tra agent status
#   bash deploy/dev/scripts/connect-agent.sh --stop            # dừng agent
#   bash deploy/dev/scripts/connect-agent.sh --logs            # xem logs agent
#   bash deploy/dev/scripts/connect-agent.sh --help
# =================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="${SCRIPT_DIR}/.."

# ── Colours ──────────────────────────────────────────────────────
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'
RED='\033[0;31m'; BOLD='\033[1m'; CYAN='\033[0;36m'; NC='\033[0m'

log()  { echo -e "${GREEN}[connect-agent]${NC} $*" >&2; }
warn() { echo -e "${YELLOW}[connect-agent]${NC} $*" >&2; }
err()  { echo -e "${RED}[connect-agent] ERROR:${NC} $*" >&2; exit 1; }
info() { echo -e "${CYAN}$*${NC}" >&2; }

# ── Load .env ─────────────────────────────────────────────────────
ENV_FILE="${DEPLOY_DIR}/.env"
[ -f "${ENV_FILE}" ] || err "Không tìm thấy .env tại ${ENV_FILE}"
# shellcheck disable=SC1090
source "${ENV_FILE}"

# ── Defaults từ .env ─────────────────────────────────────────────
DEV_SERVER_HOST="${DEV_SERVER_HOST:-172.20.2.31}"
DEV_SERVER_USER="${DEV_SERVER_USER:-ubuntu}"
DEV_SERVER_PORT="${DEV_SERVER_PORT:-22}"
DEV_SERVER_KEY="${SERVER_KEY:-~/.ssh/id_ed25519}"
AGENT_ORCA_URL="${AGENT_ORCA_URL:-wss://b15.openledger.vn/agent}"
AGENT_PORT="${AGENT_PORT:-6799}"
AGENT_RELAY_TOKEN="${AGENT_RELAY_TOKEN:-relay-secret}"
AGENT_DEPLOY_DIR="${AGENT_DEPLOY_DIR:-~/orca-agent}"

# Orca server container (để gọi admin API sinh token)
ORCA_CONTAINER="orca-server"
SERVER_HOST="${SERVER_HOST:-172.20.2.39}"
SERVER_USER="${SERVER_USER:-ubuntu}"
SERVER_KEY="${SERVER_KEY:-~/.ssh/id_ed25519}"
ORCA_ADMIN_EMAIL="${ORCA_ADMIN_EMAIL:-admin@b15.openledger.vn}"
ORCA_ADMIN_PASSWORD="${ORCA_ADMIN_PASSWORD:-}"

SSH_OPTS="-i ${DEV_SERVER_KEY} -p ${DEV_SERVER_PORT} -o StrictHostKeyChecking=no -o ConnectTimeout=10 -o BatchMode=yes"
SSH_ORCA_OPTS="-i ${SERVER_KEY} -o StrictHostKeyChecking=no -o ConnectTimeout=10 -o BatchMode=yes"

# ── Parse args ────────────────────────────────────────────────────
MODE="direct-ws"
DO_DEPLOY=false
DO_START=false
DO_STATUS=false
DO_STOP=false
DO_LOGS=false

for arg in "$@"; do
    case "${arg}" in
        --mode)       ;;
        relay-ws)     MODE="relay-ws" ;;
        direct-ws)    MODE="direct-ws" ;;
        --mode=relay-ws)   MODE="relay-ws" ;;
        --mode=direct-ws)  MODE="direct-ws" ;;
        --deploy)     DO_DEPLOY=true; DO_START=true ;;
        --start)      DO_START=true ;;
        --status)     DO_STATUS=true ;;
        --stop)       DO_STOP=true ;;
        --logs)       DO_LOGS=true ;;
        --help|-h)    SHOW_HELP=true ;;
    esac
done

# ── Helpers ───────────────────────────────────────────────────────
ssh_dev()   { ssh ${SSH_OPTS}  "${DEV_SERVER_USER}@${DEV_SERVER_HOST}" "$@"; }
ssh_orca()  { ssh ${SSH_ORCA_OPTS} "${SERVER_USER}@${SERVER_HOST}"     "$@"; }

print_help() {
    echo ""
    echo -e "${BOLD}connect-agent.sh — Khởi tạo kết nối Agent với Orca${NC}"
    echo ""
    echo -e "${BOLD}Usage:${NC}"
    echo "  bash deploy/dev/scripts/connect-agent.sh [mode] [action]"
    echo ""
    echo -e "${BOLD}Modes:${NC}"
    echo "  (none)             direct-websocket: agent kết nối VÀO Orca"
    echo "  --mode=relay-ws    relay-websocket: Orca kết nối VÀO agent WS server"
    echo ""
    echo -e "${BOLD}Actions:${NC}"
    echo "  --deploy           Deploy agent lên dev server + khởi động"
    echo "  --start            Khởi động agent đã deploy"
    echo "  --status           Kiểm tra agent đang chạy không"
    echo "  --stop             Dừng agent"
    echo "  --logs             Xem logs agent (10 dòng cuối)"
    echo "  (none)             In hướng dẫn + lệnh để chạy thủ công"
    echo ""
    echo -e "${BOLD}Dev Server:${NC} ${DEV_SERVER_USER}@${DEV_SERVER_HOST}:${DEV_SERVER_PORT}"
    echo -e "${BOLD}Orca URL:${NC}   ${AGENT_ORCA_URL}"
    echo ""
}

# Show help if requested (after print_help is defined)
${SHOW_HELP:-false} && { print_help; exit 0; }

# ── Sinh agentToken qua Orca admin API ───────────────────────────
# Gọi POST /api/agent-token trên HTTP server để đăng ký slot trong RAM.
generate_agent_token() {
    log "Sinh agent token từ Orca server..."

    local ORCA_HTTP_HOST="${SERVER_HOST:-172.20.2.39}"
    local ORCA_HTTP_PORT="${ORCA_HTTP_PORT:-6769}"
    local API_SECRET="${ORCA_AGENT_API_SECRET:-}"
    local DEV_SERVER_ID="${DEV_SERVER_LABEL:-dev-local}"

    # Gọi API qua internal IP (không qua Nginx public)
    local AUTH_HEADER
    if [ -n "${API_SECRET}" ]; then
        AUTH_HEADER="Authorization: Bearer ${API_SECRET}"
    else
        AUTH_HEADER="X-Orca-Admin: 1"
    fi

    local RESULT
    RESULT=$(ssh_orca "curl -sf -X POST \
        -H 'Content-Type: application/json' \
        -H '${AUTH_HEADER}' \
        -d '{\"devServerId\":\"${DEV_SERVER_ID}\",\"ttl\":300}' \
        http://${ORCA_HTTP_HOST}:${ORCA_HTTP_PORT}/api/agent-token" 2>/dev/null) \
        || err "Không thể sinh token từ Orca server. Kiểm tra:\n  ssh ${SERVER_USER}@${SERVER_HOST} 'docker logs orca-server --tail 5'"

    echo "${RESULT}"
}

# ── Deploy agent lên dev server ───────────────────────────────────
deploy_agent() {
    log "Deploy agent lên ${DEV_SERVER_USER}@${DEV_SERVER_HOST}..."

    # Tạo thư mục agent trên dev server
    ssh_dev "mkdir -p ${AGENT_DEPLOY_DIR}"

    # Copy agent files
    local AGENT_SRC="${DEPLOY_DIR}/agent"
    if [ -d "${AGENT_SRC}" ]; then
        rsync -avz --delete \
            -e "ssh ${SSH_OPTS}" \
            "${AGENT_SRC}/" \
            "${DEV_SERVER_USER}@${DEV_SERVER_HOST}:${AGENT_DEPLOY_DIR}/" \
            --exclude node_modules \
            --exclude dist \
            --exclude .env
        log "Agent files copied"
    else
        warn "Không tìm thấy ${AGENT_SRC}. Tạo minimal agent..."
        create_minimal_agent
    fi

    # Cài dependencies
    log "Cài npm dependencies trên dev server..."
    ssh_dev "cd ${AGENT_DEPLOY_DIR} && npm install --production 2>&1 | tail -5"
    log "Deploy hoàn thành ✅"
}

# ── Tạo minimal agent nếu chưa có ────────────────────────────────
create_minimal_agent() {
    log "Tạo minimal agent package trên dev server..."
    ssh_dev "mkdir -p ${AGENT_DEPLOY_DIR} && cat > ${AGENT_DEPLOY_DIR}/package.json" << 'EOF'
{
  "name": "orca-dev-agent",
  "version": "1.0.0",
  "description": "Orca Dev Server Agent",
  "main": "agent.js",
  "scripts": {
    "start": "node agent.js",
    "start:direct": "node agent.js --mode direct-ws",
    "start:relay": "node agent.js --mode relay-ws"
  },
  "dependencies": {
    "ws": "^8.18.0"
  }
}
EOF
    # Upload agent.js từ agent/ directory nếu có hoặc dùng placeholder
    if [ -f "${DEPLOY_DIR}/agent/agent.js" ]; then
        scp ${SSH_OPTS} "${DEPLOY_DIR}/agent/agent.js" \
            "${DEV_SERVER_USER}@${DEV_SERVER_HOST}:${AGENT_DEPLOY_DIR}/agent.js"
    fi
}

# ── Start agent ───────────────────────────────────────────────────
start_agent_direct() {
    local AGENT_TOKEN="$1"
    local WORK_DIR="${AGENT_WORK_DIR:-/srv/projects/vnp-blc}"
    log "Khởi động agent (direct-websocket) trên dev server..."
    log "  ORCA_URL:  ${AGENT_ORCA_URL}"
    log "  WORK_DIR:  ${WORK_DIR}"
    log "  TOKEN:     ${AGENT_TOKEN:0:16}... (truncated)"

    ssh_dev "cd ${AGENT_DEPLOY_DIR} && mkdir -p logs && \
        AGENT_TOKEN='${AGENT_TOKEN}' \
        ORCA_URL='${AGENT_ORCA_URL}' \
        MODE='direct-websocket' \
        DEV_SERVER_ID='${DEV_SERVER_LABEL}' \
        AGENT_WORK_DIR='${WORK_DIR}' \
        nohup node agent.js > logs/agent.log 2>&1 &
        echo \$! > logs/agent.pid
        sleep 1
        if kill -0 \$(cat logs/agent.pid) 2>/dev/null; then
            echo 'Agent started (PID: '\$(cat logs/agent.pid)')'
        else
            echo 'WARN: agent exited immediately — check logs/agent.log'
            tail -20 logs/agent.log 2>/dev/null || true
        fi"
}

start_agent_relay() {
    local WORK_DIR="${AGENT_WORK_DIR:-/srv/projects/vnp-blc}"
    log "Khởi động agent (relay-websocket) trên dev server..."
    log "  PORT:     ${AGENT_PORT}"
    log "  WORK_DIR: ${WORK_DIR}"
    log "  TOKEN:    ${AGENT_RELAY_TOKEN:0:8}... (truncated)"

    ssh_dev "cd ${AGENT_DEPLOY_DIR} && mkdir -p logs && \
        AGENT_PORT='${AGENT_PORT}' \
        AGENT_TOKEN='${AGENT_RELAY_TOKEN}' \
        MODE='relay-websocket' \
        DEV_SERVER_ID='${DEV_SERVER_LABEL}' \
        AGENT_WORK_DIR='${WORK_DIR}' \
        nohup node agent.js > logs/agent.log 2>&1 &
        echo \$! > logs/agent.pid
        sleep 1
        if kill -0 \$(cat logs/agent.pid) 2>/dev/null; then
            echo 'Agent started (PID: '\$(cat logs/agent.pid)')'
        else
            echo 'WARN: agent exited immediately — check logs/agent.log'
            tail -20 logs/agent.log 2>/dev/null || true
        fi"
}

# ── Status / Stop / Logs ─────────────────────────────────────────
agent_status() {
    log "Kiểm tra agent status trên ${DEV_SERVER_HOST}..."
    ssh_dev "cd ${AGENT_DEPLOY_DIR} && \
        if [ -f logs/agent.pid ]; then
            PID=\$(cat logs/agent.pid);
            if kill -0 \$PID 2>/dev/null; then
                echo \"✅ Agent running (PID: \$PID)\";
                ps -p \$PID -o pid,etime,cmd 2>/dev/null || true;
            else
                echo \"❌ Agent NOT running (stale PID: \$PID)\";
            fi
        else
            echo \"❌ Agent not started (no PID file)\";
        fi" 2>/dev/null || echo "❌ Không thể kết nối dev server"
}

agent_stop() {
    log "Dừng agent trên ${DEV_SERVER_HOST}..."
    ssh_dev "cd ${AGENT_DEPLOY_DIR} && \
        if [ -f logs/agent.pid ]; then
            PID=\$(cat logs/agent.pid);
            kill \$PID 2>/dev/null && echo \"✅ Agent stopped (PID: \$PID)\" || echo \"Agent was not running\";
            rm -f logs/agent.pid;
        else
            echo \"Agent không đang chạy\";
        fi" 2>/dev/null || err "Không thể kết nối dev server"
}

agent_logs() {
    log "Logs agent (50 dòng cuối) từ ${DEV_SERVER_HOST}..."
    echo ""
    # TASK-DS-010 (BUG-DS-007): ưu tiên file log (agent-direct.log),
    # fallback journald nếu service dùng journal (orca-agent.service cũ)
    ssh_dev "
        LOG_FILE=${AGENT_DEPLOY_DIR}/logs/agent-direct.log
        AGENT_LOG=${AGENT_DEPLOY_DIR}/logs/agent.log
        if [ -f \"\${LOG_FILE}\" ]; then
            tail -50 \"\${LOG_FILE}\"
        elif [ -f \"\${AGENT_LOG}\" ]; then
            tail -50 \"\${AGENT_LOG}\"
        else
            journalctl -u orca-agent-direct -n 50 --no-pager 2>/dev/null \
                || journalctl -u orca-agent -n 50 --no-pager 2>/dev/null \
                || echo 'Chưa có logs. Agent chưa chạy hoặc log file chưa được tạo.'
        fi
    "
}

# ── Print manual instructions ─────────────────────────────────────
print_direct_instructions() {
    local TOKEN="$1"
    local EXPIRES="$2"

    echo ""
    echo -e "${BOLD}${CYAN}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}${CYAN} 🔗 Orca Agent — direct-websocket mode${NC}"
    echo -e "${BOLD}${CYAN}═══════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "${BOLD}Dev Server:${NC} ${DEV_SERVER_HOST}"
    echo -e "${BOLD}Orca URL:${NC}   ${AGENT_ORCA_URL}"
    echo -e "${BOLD}Token:${NC}      ${GREEN}${TOKEN}${NC}"
    echo -e "${BOLD}Expires:${NC}    ${EXPIRES}s"
    echo ""
    echo -e "${BOLD}Chạy lệnh này trên dev server (${DEV_SERVER_HOST}):${NC}"
    echo ""
    echo -e "${BLUE}  ORCA_URL=${AGENT_ORCA_URL} ${NC}\\"
    echo -e "${BLUE}  AGENT_TOKEN=${TOKEN} ${NC}\\"
    echo -e "${BLUE}  node agent.js${NC}"
    echo ""
    echo -e "${YELLOW}─────────────────────────────────────────────────────────────${NC}"
    echo -e "  Mode    : direct-websocket (agent → Orca)"
    echo -e "  Orca    : ${AGENT_ORCA_URL}"
    echo -e "  Server  : ${DEV_SERVER_HOST}"
    echo -e "  Token   : ${TOKEN}"
    echo -e "${YELLOW}─────────────────────────────────────────────────────────────${NC}"
    echo ""
    echo -e "💡 Tips:"
    echo -e "  - Token hết hạn sau ${EXPIRES}s — chạy agent ngay"
    echo -e "  - Dùng ${BOLD}--deploy --mode=direct-ws${NC} để auto-deploy và khởi động"
    echo -e "  - Dùng ${BOLD}--status${NC} để kiểm tra agent đang chạy"
    echo ""
}

print_relay_instructions() {
    echo ""
    echo -e "${BOLD}${CYAN}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}${CYAN} 🔗 Orca Agent — relay-websocket mode${NC}"
    echo -e "${BOLD}${CYAN}═══════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "${BOLD}Dev Server:${NC} ${DEV_SERVER_HOST}:${AGENT_PORT}"
    echo -e "${BOLD}Token:${NC}      ${GREEN}${AGENT_RELAY_TOKEN}${NC}"
    echo ""
    echo -e "${BOLD}1. Chạy agent trên dev server:${NC}"
    echo ""
    echo -e "${BLUE}  AGENT_PORT=${AGENT_PORT} ${NC}\\"
    echo -e "${BLUE}  AGENT_TOKEN=${AGENT_RELAY_TOKEN} ${NC}\\"
    echo -e "${BLUE}  MODE=relay-websocket ${NC}\\"
    echo -e "${BLUE}  node agent.js${NC}"
    echo ""
    echo -e "${BOLD}2. Trong Orca UI, thêm Dev Server:${NC}"
    echo -e "   Connection Type: relay-websocket"
    echo -e "   WebSocket URL:   ${BLUE}ws://${DEV_SERVER_HOST}:${AGENT_PORT}/orca-relay?token=${AGENT_RELAY_TOKEN}${NC}"
    echo ""
    echo -e "${YELLOW}─────────────────────────────────────────────────────────────${NC}"
    echo -e "  Mode    : relay-websocket (Orca → agent)"
    echo -e "  URL     : ws://${DEV_SERVER_HOST}:${AGENT_PORT}/orca-relay"
    echo -e "  Token   : ${AGENT_RELAY_TOKEN}"
    echo -e "${YELLOW}─────────────────────────────────────────────────────────────${NC}"
    echo ""
    echo -e "💡 Tips:"
    echo -e "  - Agent phải chạy TRƯỚC khi Orca test connection"
    echo -e "  - Đổi AGENT_RELAY_TOKEN trong .env để đổi secret"
    echo -e "  - Dùng ${BOLD}--deploy --mode=relay-ws${NC} để auto-deploy và khởi động"
    echo ""
}

# ── Main ──────────────────────────────────────────────────────────
main() {
    # Handle single actions without token generation
    if ${DO_STATUS}; then agent_status; exit 0; fi
    if ${DO_STOP};   then agent_stop;   exit 0; fi
    if ${DO_LOGS};   then agent_logs;   exit 0; fi

    # Deploy nếu được yêu cầu
    if ${DO_DEPLOY}; then
        deploy_agent
    fi

    if [ "${MODE}" = "direct-ws" ]; then
        # Sinh token
        local TOKEN_JSON
        TOKEN_JSON=$(generate_agent_token)
        local TOKEN
        TOKEN=$(echo "${TOKEN_JSON}" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['token'])" 2>/dev/null) \
            || err "Không thể parse token response:\n${TOKEN_JSON}"
        local EXPIRES
        EXPIRES=$(echo "${TOKEN_JSON}" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['expiresIn'])" 2>/dev/null) || EXPIRES="300"

        if ${DO_START}; then
            # Tạo logs dir trước
            ssh_dev "mkdir -p ${AGENT_DEPLOY_DIR}/logs"
            start_agent_direct "${TOKEN}"
        else
            print_direct_instructions "${TOKEN}" "${EXPIRES}"
        fi
    else
        # relay-websocket mode
        if ${DO_START}; then
            # Tạo logs dir trước
            ssh_dev "mkdir -p ${AGENT_DEPLOY_DIR}/logs"
            start_agent_relay
        else
            print_relay_instructions
        fi
    fi
}

main "$@"
