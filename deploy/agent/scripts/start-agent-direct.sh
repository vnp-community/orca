#!/usr/bin/env bash
set -euo pipefail

# ── Script hỗ trợ deploy agent ở direct-websocket mode trên Dev Server ──
# Agent sẽ tự lấy token mới từ Orca server mỗi lần start/restart.

# Biến kết nối (từ .env nếu có)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/../.env"
[ -f "${ENV_FILE}" ] && source "${ENV_FILE}" || true

DEV_SERVER_HOST="${DEV_SERVER_HOST:-172.20.2.31}"
DEV_SERVER_USER="${DEV_SERVER_USER:-ubuntu}"
SSH_KEY="${DEV_SERVER_KEY:-~/.ssh/id_ed25519}"
ORCA_URL="${AGENT_ORCA_URL:-wss://b15.openledger.vn/agent}"
ORCA_HTTP_HOST="${SERVER_HOST:-172.20.2.39}"
# 6768 = backend-go api-gateway's public port (deploy/dev/.env
# API_GATEWAY_PUBLIC_PORT) — NOT 6769, which is the frontend/nginx port.
ORCA_HTTP_PORT_VAL="${ORCA_HTTP_PORT:-6768}"
AUTH_HDR="${ORCA_AGENT_API_SECRET:+Authorization: Bearer ${ORCA_AGENT_API_SECRET}}"
AUTH_HDR="${AUTH_HDR:-X-Orca-Admin: 1}"
DEV_NAME="${DEV_SERVER_LABEL:-Dev Server (dev-local)}"
DEV_SERVER_ID="${DEV_SERVER_LABEL:-dev-local}"

echo -e "\033[0;34m[run-direct]\033[0m Deploy agent daemon lên ${DEV_SERVER_HOST}..."
echo -e "  ORCA_URL:  ${ORCA_URL}"
echo -e "  Wrapper:   /home/${DEV_SERVER_USER}/orca-agent/start.sh (auto-token on every restart)"

WORK_DIR="/home/${DEV_SERVER_USER}"

# ── Wrapper script: fetch fresh token rồi start agent ──────────────────────────
# Wrapper được chạy bởi systemd ExecStart thay vì agent.js trực tiếp.
# Mỗi lần service start/restart → wrapper gọi API → lấy token mới → chạy agent.
# Token không bao giờ hết hạn vì luôn được làm mới trước khi agent connect.
cat << WRAPPER > /tmp/orca-agent-start.sh
#!/usr/bin/env bash
# Orca Agent Starter — auto-fetches fresh token on every start
set -euo pipefail

ORCA_HTTP_HOST="${ORCA_HTTP_HOST}"
ORCA_HTTP_PORT="${ORCA_HTTP_PORT_VAL}"
ORCA_URL="${ORCA_URL}"
DEV_SERVER_ID="${DEV_SERVER_LABEL:-dev-local}"
DEV_SERVER_NAME="${DEV_NAME}"
AUTH_HEADER="${AUTH_HDR}"
AGENT_DIR="/home/${DEV_SERVER_USER}/orca-agent"
LOG_DIR="\${AGENT_DIR}/logs"
mkdir -p "\${LOG_DIR}"

echo "[\$(date -u +%FT%TZ)] Fetching fresh agent token from \${ORCA_HTTP_HOST}:\${ORCA_HTTP_PORT}..." >> "\${LOG_DIR}/agent-direct.log"

# Gọi API để đăng ký token + tạo DevServer record
# TASK-DS-009: --max-time 8 < TimeoutStopSec=10s → curl fails cleanly before systemd SIGKILL
API_RESP=\$(curl -sf --max-time 8 --retry 2 --retry-delay 2 -X POST \
  -H "Content-Type: application/json" \
  -H "\${AUTH_HEADER}" \
  -d "{\"devServerId\":\"\${DEV_SERVER_ID}\",\"name\":\"\${DEV_SERVER_NAME}\",\"ttl\":600}" \
  "http://\${ORCA_HTTP_HOST}:\${ORCA_HTTP_PORT}/api/agent-token" 2>/dev/null) || {
    echo "[\$(date -u +%FT%TZ)] ERROR: Token request failed/timed out (max 8s). Retrying in 10s..." >> "\${LOG_DIR}/agent-direct.log"
    sleep 10
    exit 1
}

NEW_TOKEN=\$(echo "\${API_RESP}" | python3 -c "import json,sys; print(json.load(sys.stdin)['token'])" 2>/dev/null) || \
NEW_TOKEN=\$(echo "\${API_RESP}" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

if [ -z "\${NEW_TOKEN}" ]; then
    echo "[\$(date -u +%FT%TZ)] ERROR: Invalid token response: \${API_RESP}" >> "\${LOG_DIR}/agent-direct.log"
    sleep 10
    exit 1
fi

echo "[\$(date -u +%FT%TZ)] Token acquired: \${NEW_TOKEN}" >> "\${LOG_DIR}/agent-direct.log"

# Start agent với token mới
exec env ORCA_URL="\${ORCA_URL}" AGENT_TOKEN="\${NEW_TOKEN}" MODE="direct-websocket" \
  AGENT_WORK_DIR="/home/${DEV_SERVER_USER}" HOME="/home/${DEV_SERVER_USER}" \
  node "\${AGENT_DIR}/agent.js"
WRAPPER

# Tạo systemd service file — dùng wrapper thay vì agent.js trực tiếp
cat << EOF > /tmp/orca-agent-direct.service
[Unit]
Description=Orca Dev Server Agent (direct-websocket, auto-token)
After=network.target

[Service]
Type=simple
User=${DEV_SERVER_USER}
WorkingDirectory=/home/${DEV_SERVER_USER}/orca-agent
Environment=NODE_ENV=production
ExecStart=/bin/bash /home/${DEV_SERVER_USER}/orca-agent/start.sh
# Restart=always: systemd restarts wrapper → wrapper fetches new token → agent starts
Restart=always
RestartSec=5
StandardOutput=append:/home/${DEV_SERVER_USER}/orca-agent/logs/agent-direct.log
StandardError=append:/home/${DEV_SERVER_USER}/orca-agent/logs/agent-direct.log
MemoryMax=512M

[Install]
WantedBy=multi-user.target
EOF

# Copy wrapper script và service file lên dev server
scp -i ${SSH_KEY} -o StrictHostKeyChecking=no \
    /tmp/orca-agent-start.sh \
    ${DEV_SERVER_USER}@${DEV_SERVER_HOST}:/tmp/orca-agent-start.sh 2>/dev/null
scp -i ${SSH_KEY} -o StrictHostKeyChecking=no \
    /tmp/orca-agent-direct.service \
    ${DEV_SERVER_USER}@${DEV_SERVER_HOST}:/tmp/orca-agent-direct.service 2>/dev/null

# Setup systemd trên dev server
ssh -i ${SSH_KEY} -o StrictHostKeyChecking=no ${DEV_SERVER_USER}@${DEV_SERVER_HOST} "bash -s" << EOF
  # Stop relay service nếu đang chạy
  sudo systemctl stop orca-agent 2>/dev/null || true

  # Setup logs dir + install wrapper script
  mkdir -p /home/${DEV_SERVER_USER}/orca-agent/logs
  cp /tmp/orca-agent-start.sh /home/${DEV_SERVER_USER}/orca-agent/start.sh
  chmod +x /home/${DEV_SERVER_USER}/orca-agent/start.sh

  # Cài đặt và start service
  sudo mv /tmp/orca-agent-direct.service /etc/systemd/system/orca-agent-direct.service
  sudo systemctl daemon-reload
  sudo systemctl enable orca-agent-direct
  sudo systemctl restart orca-agent-direct

  echo -e "\n✅ \033[0;32mService orca-agent-direct đã được khởi động!\033[0m"
  echo "Mỗi lần restart, agent tự lấy token mới từ Orca server."
  echo "Để kiểm tra trạng thái:"
  echo -e "\033[0;34m  ssh ${DEV_SERVER_USER}@${DEV_SERVER_HOST} 'sudo systemctl status orca-agent-direct'\033[0m"
  echo "Để xem logs:"
  echo -e "\033[0;34m  ssh ${DEV_SERVER_USER}@${DEV_SERVER_HOST} 'tail -f /home/${DEV_SERVER_USER}/orca-agent/logs/agent-direct.log'\033[0m"
EOF

# Dọn dẹp file tạm
rm -f /tmp/orca-agent-direct.service /tmp/orca-agent-start.sh
