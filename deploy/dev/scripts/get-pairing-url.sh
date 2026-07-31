#!/usr/bin/env bash
# =================================================================
# get-pairing-url.sh — Lấy Pairing URL / Code để kết nối Orca Web UI
# =================================================================
#
# Script này kết nối vào Orca Server, tạo device token mới và in ra:
#   - Web Client URL: mở trực tiếp trong browser (auto-connect)
#   - Pairing Code:   paste vào field "Pairing URL or code" trên web UI
#
# Usage:
#   bash deploy/dev/scripts/get-pairing-url.sh           # default: URL + code
#   bash deploy/dev/scripts/get-pairing-url.sh --url     # chỉ in URL (cho pipe/script)
#   bash deploy/dev/scripts/get-pairing-url.sh --code    # chỉ in base64 code
#   bash deploy/dev/scripts/get-pairing-url.sh --json    # JSON output đầy đủ
#   bash deploy/dev/scripts/get-pairing-url.sh --open    # mở browser ngay (macOS/Linux)
#   bash deploy/dev/scripts/get-pairing-url.sh --rotate  # tạo token mới (revoke cũ)
# =================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="${SCRIPT_DIR}/.."

# ── Colours ──────────────────────────────────────────────────────
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'
BOLD='\033[1m'; CYAN='\033[0;36m'; NC='\033[0m'

log()  { echo -e "${GREEN}[get-pairing]${NC} $*" >&2; }
warn() { echo -e "${YELLOW}[get-pairing]${NC} $*" >&2; }
err()  { echo -e "\033[0;31m[get-pairing] ERROR:${NC} $*" >&2; exit 1; }

# ── Load .env ────────────────────────────────────────────────────
if [ -f "${DEPLOY_DIR}/.env" ]; then
    set -a
    # shellcheck disable=SC1091
    source "${DEPLOY_DIR}/.env"
    set +a
fi

# ── Config từ .env / defaults ─────────────────────────────────────
SERVER_HOST="${SERVER_HOST:-172.20.2.39}"
SERVER_USER="${SERVER_USER:-ubuntu}"
SERVER_KEY="${SERVER_KEY:-${HOME}/.ssh/id_ed25519}"
SERVER_PORT="${SERVER_PORT:-22}"
ORCA_DOMAIN="${ORCA_DOMAIN:-b15.openledger.vn}"
CONTAINER="${ORCA_CONTAINER_NAME:-orca-server}"
DATA_PATH="${ORCA_DATA_PATH:-/data/orca}"

SSH_OPTS="-i ${SERVER_KEY} -p ${SERVER_PORT} -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10 -o BatchMode=yes"

# ── Parse args ───────────────────────────────────────────────────
OUTPUT_MODE="default"   # default | url | code | json
DO_ROTATE=false
DO_OPEN=false

for arg in "$@"; do
    case "${arg}" in
        --url)    OUTPUT_MODE="url"  ;;
        --code)   OUTPUT_MODE="code" ;;
        --json)   OUTPUT_MODE="json" ;;
        --open)   DO_OPEN=true       ;;
        --rotate) DO_ROTATE=true     ;;
        --help|-h)
            echo ""
            echo -e "${BOLD}Usage:${NC}"
            echo "  bash deploy/dev/scripts/get-pairing-url.sh [options]"
            echo ""
            echo -e "${BOLD}Options:${NC}"
            echo "  (none)     In URL + code đầy đủ"
            echo "  --url      Chỉ in web client URL (dùng trong pipe/script)"
            echo "  --code     Chỉ in base64 pairing code"
            echo "  --json     JSON output: {url, code, deviceId, endpoint}"
            echo "  --open     Mở browser ngay sau khi tạo URL"
            echo "  --rotate   Tạo token mới (revoke token cũ chưa dùng)"
            echo "  --help     Hiển thị help này"
            echo ""
            exit 0
            ;;
    esac
done

# ── Node.js script chạy trong container ──────────────────────────
# Dùng heredoc để tránh escape hell
NODE_SCRIPT=$(cat << 'NODESCRIPT'
const crypto = require('crypto');
const fs = require('fs');

const dataPath = process.env.ORCA_DATA_PATH || '/data/orca';
const domain   = process.env.ORCA_DOMAIN    || 'localhost';
const rotate   = process.env.ROTATE         === 'true';

// 1. Đọc E2EE public key
const keypairPath = `${dataPath}/orca-e2ee-keypair.json`;
if (!fs.existsSync(keypairPath)) {
  console.error('ERROR: E2EE keypair not found at ' + keypairPath);
  process.exit(1);
}
const keypair = JSON.parse(fs.readFileSync(keypairPath, 'utf8'));
const publicKeyB64 = keypair.publicKeyB64;
if (!publicKeyB64) {
  console.error('ERROR: publicKeyB64 missing in keypair file');
  process.exit(1);
}

// 2. Đọc device registry
const regPath = `${dataPath}/orca-devices.json`;
let registry = { devices: [] };
if (fs.existsSync(regPath)) {
  try { registry = JSON.parse(fs.readFileSync(regPath, 'utf8')); } catch(e) {}
}
if (!Array.isArray(registry.devices)) registry.devices = [];

// 3. Rotate hoặc reuse pending device
const scope = 'runtime';
if (rotate) {
  // Revoke tất cả pending runtime tokens chưa dùng
  registry.devices = registry.devices.filter(d => !(d.lastSeenAt === 0 && d.scope === scope));
}

// Tìm pending device chưa dùng
let device = registry.devices.find(d => d.lastSeenAt === 0 && d.scope === scope);
if (!device) {
  // Tạo mới
  device = {
    deviceId:  crypto.randomUUID(),
    name:      'Web Browser',
    token:     crypto.randomBytes(24).toString('hex'),
    scope,
    pairedAt:  Date.now(),
    lastSeenAt: 0
  };
  registry.devices.push(device);
  // Lưu lại
  fs.writeFileSync(regPath, JSON.stringify(registry, null, 2), { mode: 0o600 });
}

// 4. Build pairing offer
// Why: ORCA_DOMAIN có thể có hoặc không có scheme prefix (wss:// hoặc ws://)
// Nếu đã có prefix thì dùng nguyên, nếu chưa thì thêm wss:// mặc định.
const rawDomain = domain;
let wsEndpoint;
if (/^wss?:\/\//i.test(rawDomain)) {
  wsEndpoint = rawDomain.replace(/\/$/, '');  // strip trailing slash
} else {
  wsEndpoint = `wss://${rawDomain}`;
}
const offer = {
  v: 2,
  endpoint:    wsEndpoint,
  deviceToken: device.token,
  publicKeyB64,
  scope
};
const pairingCode  = Buffer.from(JSON.stringify(offer)).toString('base64url');
const webClientUrl = `https://${rawDomain.replace(/^wss?:\/\//i, '').split(':')[0]}/#pairing=${encodeURIComponent(pairingCode)}`;

// 5. Output JSON
console.log(JSON.stringify({
  url:      webClientUrl,
  code:     pairingCode,
  deviceId: device.deviceId,
  endpoint: offer.endpoint,
  domain,
  isNew:    device.pairedAt === device.pairedAt  // always true; for schema compat
}));
NODESCRIPT
)

# ── Chạy script trên container ───────────────────────────────────
if [ "${OUTPUT_MODE}" = "default" ]; then
    log "Kết nối đến ${SERVER_USER}@${SERVER_HOST}:${SERVER_PORT}..."
fi

RESULT=$(ssh ${SSH_OPTS} "${SERVER_USER}@${SERVER_HOST}" \
    "ORCA_DATA_PATH='${DATA_PATH}' ORCA_DOMAIN='${ORCA_DOMAIN}' ROTATE='${DO_ROTATE}' \
     docker exec -e ORCA_DATA_PATH='${DATA_PATH}' -e ORCA_DOMAIN='${ORCA_DOMAIN}' -e ROTATE='${DO_ROTATE}' \
     ${CONTAINER} node -e $(printf '%q' "${NODE_SCRIPT}")" 2>/dev/null \
) || err "Không thể kết nối đến server hoặc container không chạy.
  Kiểm tra: ssh ${SERVER_USER}@${SERVER_HOST} 'docker ps | grep ${CONTAINER}'"

# Parse JSON result
WEB_URL=$(echo "${RESULT}" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['url'])" 2>/dev/null) \
    || err "Server trả về kết quả không hợp lệ:\n${RESULT}"
PAIR_CODE=$(echo "${RESULT}" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['code'])" 2>/dev/null)
DEVICE_ID=$(echo "${RESULT}" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['deviceId'])" 2>/dev/null)
ENDPOINT=$(echo "${RESULT}"  | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['endpoint'])" 2>/dev/null)

# ── Output ───────────────────────────────────────────────────────
case "${OUTPUT_MODE}" in
    "url")
        echo "${WEB_URL}"
        ;;

    "code")
        echo "${PAIR_CODE}"
        ;;

    "json")
        echo "${RESULT}" | python3 -m json.tool
        ;;

    "default"|*)
        echo ""
        echo -e "${BOLD}${CYAN}═══════════════════════════════════════════════════════════${NC}"
        echo -e "${BOLD}${CYAN} 🔗 Orca Pairing Info${NC}"
        echo -e "${BOLD}${CYAN}═══════════════════════════════════════════════════════════${NC}"
        echo ""
        echo -e "${BOLD}Cách 1 — Mở URL trực tiếp (auto-connect):${NC}"
        echo -e "${BLUE}${WEB_URL}${NC}"
        echo ""
        echo -e "${BOLD}Cách 2 — Paste vào field \"Pairing URL or code\":${NC}"
        echo -e "${GREEN}${PAIR_CODE}${NC}"
        echo ""
        echo -e "${YELLOW}─────────────────────────────────────────────────────────────${NC}"
        echo -e "  Endpoint : ${ENDPOINT}"
        echo -e "  Device   : ${DEVICE_ID}"
        echo -e "  Server   : ${SERVER_HOST}"
        echo -e "  Rotate   : ${DO_ROTATE}"
        echo -e "${YELLOW}─────────────────────────────────────────────────────────────${NC}"
        echo ""
        echo -e "💡 Tips:"
        echo -e "  - Token sẽ bị xoá sau khi browser đã connect thành công"
        echo -e "  - Dùng ${BOLD}--rotate${NC} để revoke token cũ và tạo mới"
        echo -e "  - Dùng ${BOLD}--open${NC} để mở browser tự động"
        echo -e "  - Dùng ${BOLD}--url${NC} để lấy URL dùng trong pipe: \$(script --url)"
        echo ""
        ;;
esac

# ── Mở browser ───────────────────────────────────────────────────
if [ "${DO_OPEN}" = "true" ]; then
    if command -v open &>/dev/null; then
        log "Mở browser (macOS)..."
        open "${WEB_URL}"
    elif command -v xdg-open &>/dev/null; then
        log "Mở browser (Linux)..."
        xdg-open "${WEB_URL}"
    else
        warn "Không tìm thấy lệnh mở browser. Copy URL trên để mở thủ công."
    fi
fi
