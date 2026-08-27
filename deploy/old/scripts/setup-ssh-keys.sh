#!/usr/bin/env bash
# =================================================================
# setup-ssh-keys.sh — Chuẩn bị SSH key cho Orca Server → Dev Machine
# =================================================================
#
# Script này chạy trên máy LOCAL (developer), thực hiện:
#   1. Sinh SSH keypair ed25519 tại deploy/dev/docker/orca/ssh/
#   2. Tạo SSH config cho các dev server
#   3. Copy public key lên dev server (authorize)
#   4. Test kết nối từ Orca container → dev server
#
# Usage:
#   # Lần đầu: sinh key + authorize tất cả dev servers trong .env
#   bash deploy/dev/scripts/setup-ssh-keys.sh
#
#   # Chỉ authorize một server cụ thể:
#   bash deploy/dev/scripts/setup-ssh-keys.sh --authorize 172.20.2.31
#
#   # Xem public key (để thêm thủ công vào server):
#   bash deploy/dev/scripts/setup-ssh-keys.sh --print-pubkey
#
#   # Test SSH từ Orca container → dev server:
#   bash deploy/dev/scripts/setup-ssh-keys.sh --test
# =================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="${SCRIPT_DIR}/.."

# ── Colours ──────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; BOLD='\033[1m'; NC='\033[0m'

log()  { echo -e "${GREEN}[setup-ssh]${NC} $*"; }
warn() { echo -e "${YELLOW}[setup-ssh]${NC} $*"; }
err()  { echo -e "${RED}[setup-ssh] ERROR:${NC} $*" >&2; }
info() { echo -e "${BLUE}[setup-ssh]${NC} $*"; }

# ── Load .env ────────────────────────────────────────────────────
if [ -f "${DEPLOY_DIR}/.env" ]; then
    set -a
    # shellcheck disable=SC1091
    source "${DEPLOY_DIR}/.env"
    set +a
fi

# ── Config ───────────────────────────────────────────────────────
SSH_DIR="${DEPLOY_DIR}/docker/orca/ssh"
KEY_FILE="${SSH_DIR}/id_ed25519"
CONFIG_FILE="${SSH_DIR}/config"
KNOWN_HOSTS_FILE="${SSH_DIR}/known_hosts"

# Orca Server (nơi container chạy)
ORCA_SERVER_HOST="${SERVER_HOST:-172.20.2.39}"
ORCA_SERVER_USER="${SERVER_USER:-ubuntu}"
ORCA_SERVER_KEY="${SERVER_KEY:-${HOME}/.ssh/id_ed25519}"
ORCA_SERVER_PORT="${SERVER_PORT:-22}"

# Dev Server(s) (nơi Orca SSH vào để relay code)
DEV_SERVER_HOST="${DEV_SERVER_HOST:-172.20.2.31}"
DEV_SERVER_USER="${DEV_SERVER_USER:-ubuntu}"
DEV_SERVER_PORT="${DEV_SERVER_PORT:-22}"
DEV_SERVER_KEY="${SERVER_KEY:-${HOME}/.ssh/id_ed25519}"   # key dùng để authorize

# Tên hostname trong SSH config
DEV_SERVER_LABEL="${DEV_SERVER_LABEL:-dev-local}"

SSH_OPTS_ORCA="-i ${ORCA_SERVER_KEY} -p ${ORCA_SERVER_PORT} -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10"
SSH_OPTS_DEV="-i ${DEV_SERVER_KEY} -p ${DEV_SERVER_PORT} -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10"

# ── Parse args ───────────────────────────────────────────────────
ACTION="all"
AUTHORIZE_HOST=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --authorize)
            ACTION="authorize"
            AUTHORIZE_HOST="${2:-${DEV_SERVER_HOST}}"
            shift 2
            ;;
        --print-pubkey)
            ACTION="print-pubkey"
            shift
            ;;
        --test)
            ACTION="test"
            shift
            ;;
        --help|-h)
            ACTION="help"
            shift
            ;;
        *)
            shift
            ;;
    esac
done

# ── Functions ────────────────────────────────────────────────────

print_usage() {
    echo ""
    echo -e "${BOLD}Usage:${NC}"
    echo "  bash deploy/dev/scripts/setup-ssh-keys.sh [options]"
    echo ""
    echo -e "${BOLD}Options:${NC}"
    echo "  (none)               Sinh key + tạo config + authorize dev server"
    echo "  --authorize <host>   Chỉ authorize public key vào host"
    echo "  --print-pubkey       In ra public key để copy thủ công"
    echo "  --test               Test SSH từ Orca container → dev server"
    echo "  --help               Hiển thị help này"
    echo ""
}

# 1. Sinh SSH keypair
generate_key() {
    log "Chuẩn bị thư mục SSH: ${SSH_DIR}"
    mkdir -p "${SSH_DIR}"
    chmod 700 "${SSH_DIR}"

    if [ -f "${KEY_FILE}" ]; then
        warn "Key đã tồn tại: ${KEY_FILE}"
        warn "Dùng key hiện có (không sinh lại). Xoá để sinh key mới."
    else
        log "Sinh SSH keypair ed25519..."
        ssh-keygen -t ed25519 \
            -f "${KEY_FILE}" \
            -N "" \
            -C "orca-server@${ORCA_SERVER_HOST}"
        log "✅ Key đã tạo:"
        log "   Private: ${KEY_FILE}"
        log "   Public:  ${KEY_FILE}.pub"
    fi

    chmod 600 "${KEY_FILE}"
    chmod 644 "${KEY_FILE}.pub"
}

# 2. Tạo SSH config
generate_ssh_config() {
    log "Tạo SSH config: ${CONFIG_FILE}"
    cat > "${CONFIG_FILE}" << EOF
# SSH Config cho Orca Server container
# Quản lý bởi: deploy/dev/scripts/setup-ssh-keys.sh
# KHÔNG sửa thủ công — tái tạo bằng script

# ── Dev Machine (172.20.2.31) ─────────────────────────────────
Host ${DEV_SERVER_LABEL}
    HostName ${DEV_SERVER_HOST}
    Port     ${DEV_SERVER_PORT}
    User     ${DEV_SERVER_USER}
    IdentityFile /home/orca/.ssh/id_ed25519
    UserKnownHostsFile /home/orca/.ssh/known_hosts
    StrictHostKeyChecking accept-new
    ServerAliveInterval 30
    ServerAliveCountMax 3

# ── Shortcut: địa chỉ IP trực tiếp ──────────────────────────
Host ${DEV_SERVER_HOST}
    Port     ${DEV_SERVER_PORT}
    User     ${DEV_SERVER_USER}
    IdentityFile /home/orca/.ssh/id_ed25519
    UserKnownHostsFile /home/orca/.ssh/known_hosts
    StrictHostKeyChecking accept-new
    ServerAliveInterval 30
    ServerAliveCountMax 3
EOF
    chmod 600 "${CONFIG_FILE}"
    log "✅ SSH config tạo tại: ${CONFIG_FILE}"
}

# 3. Lấy fingerprint của dev server vào known_hosts
scan_known_hosts() {
    local target_host="$1"
    local target_port="${2:-22}"

    log "Lấy host fingerprint: ${target_host}:${target_port}"
    touch "${KNOWN_HOSTS_FILE}"
    chmod 600 "${KNOWN_HOSTS_FILE}"

    # Xoá entry cũ nếu có
    ssh-keygen -R "[${target_host}]:${target_port}" -f "${KNOWN_HOSTS_FILE}" 2>/dev/null || true
    ssh-keygen -R "${target_host}" -f "${KNOWN_HOSTS_FILE}" 2>/dev/null || true

    # Thêm fingerprint mới
    if [ "${target_port}" = "22" ]; then
        ssh-keyscan -H "${target_host}" >> "${KNOWN_HOSTS_FILE}" 2>/dev/null
    else
        ssh-keyscan -H -p "${target_port}" "${target_host}" >> "${KNOWN_HOSTS_FILE}" 2>/dev/null
    fi
    log "✅ Fingerprint đã lưu vào: ${KNOWN_HOSTS_FILE}"
}

# 4. Authorize public key vào dev server
authorize_key() {
    local target_host="$1"
    local target_user="${2:-ubuntu}"
    local target_port="${3:-22}"

    if [ ! -f "${KEY_FILE}.pub" ]; then
        err "Public key không tồn tại: ${KEY_FILE}.pub"
        err "Chạy script không có --authorize để sinh key trước."
        exit 1
    fi

    local pubkey
    pubkey=$(cat "${KEY_FILE}.pub")

    log "Authorize Orca public key vào ${target_user}@${target_host}:${target_port}"
    info "  Public key: ${pubkey}"
    echo ""

    ssh ${SSH_OPTS_DEV} -p "${target_port}" "${target_user}@${target_host}" bash << REMOTE
set -e
mkdir -p ~/.ssh
chmod 700 ~/.ssh

# Xoá entry cũ nếu có (tránh duplicate)
if [ -f ~/.ssh/authorized_keys ]; then
    grep -v "orca-server@${ORCA_SERVER_HOST}" ~/.ssh/authorized_keys > /tmp/ak.tmp 2>/dev/null || true
    mv /tmp/ak.tmp ~/.ssh/authorized_keys
fi

# Thêm key mới
echo "${pubkey}" >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys

echo "✅ Key đã authorize trên \$(hostname)"
echo "   authorized_keys:"
tail -1 ~/.ssh/authorized_keys
REMOTE

    log "✅ Authorized thành công"
}

# 5. Copy known_hosts lên Orca Server container
update_known_hosts_on_server() {
    if [ ! -f "${KNOWN_HOSTS_FILE}" ]; then
        warn "known_hosts local không có. Bỏ qua sync lên server."
        return
    fi

    log "Copy known_hosts lên Orca Server (${ORCA_SERVER_HOST})..."
    # File sẽ được mount vào container qua docker volume ./docker/orca/ssh:/home/orca/.ssh:ro
    # Cần upload lên thư mục ssh trên server trước, sau đó mount sẽ tự lấy
    rsync -az \
        -e "ssh ${SSH_OPTS_ORCA}" \
        "${SSH_DIR}/" \
        "${ORCA_SERVER_USER}@${ORCA_SERVER_HOST}:~/orca-deploy/docker/orca/ssh/"
    log "✅ SSH dir synced lên server"
}

# 6. Test SSH từ Orca container → dev server
test_ssh_from_container() {
    log "Test SSH từ Orca container (${ORCA_SERVER_HOST}) → Dev (${DEV_SERVER_HOST})"

    local result
    result=$(ssh ${SSH_OPTS_ORCA} "${ORCA_SERVER_USER}@${ORCA_SERVER_HOST}" \
        "docker exec orca-server sh -c '
            ssh -i /home/orca/.ssh/id_ed25519 \
                -o UserKnownHostsFile=/home/orca/.ssh/known_hosts \
                -o ConnectTimeout=10 \
                ${DEV_SERVER_USER}@${DEV_SERVER_HOST} \
                \"echo SSH_OK && node --version && hostname\"
        '" 2>&1)

    if echo "${result}" | grep -q "SSH_OK"; then
        log "✅ SSH từ container → dev server THÀNH CÔNG:"
        echo "${result}" | grep -E "SSH_OK|v[0-9]+\.|Linux|for-dev"
    else
        err "❌ SSH thất bại:"
        echo "${result}"
        exit 1
    fi
}

# ── Main ─────────────────────────────────────────────────────────

echo ""
echo -e "${BOLD}═══════════════════════════════════════════════════════${NC}"
echo -e "${BOLD} Orca SSH Key Setup${NC}"
echo -e "${BOLD}═══════════════════════════════════════════════════════${NC}"
echo -e "  Orca Server : ${ORCA_SERVER_HOST} (container chạy tại đây)"
echo -e "  Dev Machine : ${DEV_SERVER_HOST}  (Orca SSH vào đây)"
echo -e "  SSH Key Dir : ${SSH_DIR}"
echo -e "${BOLD}═══════════════════════════════════════════════════════${NC}"
echo ""

case "${ACTION}" in
    "help")
        print_usage
        ;;

    "print-pubkey")
        if [ ! -f "${KEY_FILE}.pub" ]; then
            err "Key chưa được sinh. Chạy script không có options trước."
            exit 1
        fi
        echo ""
        echo -e "${BOLD}Public key (thêm vào ~/.ssh/authorized_keys trên dev server):${NC}"
        echo ""
        cat "${KEY_FILE}.pub"
        echo ""
        ;;

    "authorize")
        TARGET="${AUTHORIZE_HOST:-${DEV_SERVER_HOST}}"
        generate_key
        generate_ssh_config
        scan_known_hosts "${TARGET}" "${DEV_SERVER_PORT}"
        authorize_key "${TARGET}" "${DEV_SERVER_USER}" "${DEV_SERVER_PORT}"
        update_known_hosts_on_server
        log "✅ Authorize hoàn tất cho: ${TARGET}"
        ;;

    "test")
        test_ssh_from_container
        ;;

    "all"|*)
        log "=== Bước 1/5: Sinh SSH keypair ==="
        generate_key

        log "=== Bước 2/5: Tạo SSH config ==="
        generate_ssh_config

        log "=== Bước 3/5: Lấy fingerprint dev server ==="
        scan_known_hosts "${DEV_SERVER_HOST}" "${DEV_SERVER_PORT}"

        log "=== Bước 4/5: Authorize key vào dev server ==="
        authorize_key "${DEV_SERVER_HOST}" "${DEV_SERVER_USER}" "${DEV_SERVER_PORT}"

        log "=== Bước 5/5: Sync SSH dir lên Orca Server ==="
        update_known_hosts_on_server

        echo ""
        echo -e "${GREEN}${BOLD}═══════════════════════════════════════════════════════${NC}"
        echo -e "${GREEN}${BOLD} ✅ Setup SSH hoàn tất!${NC}"
        echo -e "${GREEN}${BOLD}═══════════════════════════════════════════════════════${NC}"
        echo ""
        echo -e "Bước tiếp theo:"
        echo -e "  1. Restart Orca container để mount SSH dir mới:"
        echo -e "     ${BLUE}ssh ${ORCA_SERVER_USER}@${ORCA_SERVER_HOST}${NC}"
        echo -e "     ${BLUE}cd ~/orca-deploy && docker compose -f docker-compose.orca.yml restart orca${NC}"
        echo ""
        echo -e "  2. Test SSH từ container:"
        echo -e "     ${BLUE}bash deploy/dev/scripts/setup-ssh-keys.sh --test${NC}"
        echo ""
        echo -e "  3. Thêm dev server vào Orca qua Web UI:"
        echo -e "     ${BLUE}https://b15.openledger.vn → Add Remote Host → ${DEV_SERVER_HOST}${NC}"
        echo ""
        ;;
esac
