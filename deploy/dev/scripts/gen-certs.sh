#!/usr/bin/env bash
# ============================================================
# gen-certs.sh — Tạo self-signed TLS certificate cho Orca Server
# ============================================================
# Dùng cho môi trường dev/internal.
# Với production, thay bằng Let's Encrypt hoặc internal CA.
#
# Output:
#   docker/nginx/certs/server.crt
#   docker/nginx/certs/server.key
#
# Usage:
#   ORCA_DOMAIN=b15.openledger.vn ./deploy/dev/scripts/gen-certs.sh
# Hoặc để dùng IP của Orca server:
#   ORCA_DOMAIN=172.20.2.39 ./deploy/dev/scripts/gen-certs.sh
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="${SCRIPT_DIR}/.."
CERTS_DIR="${DEPLOY_DIR}/docker/nginx/certs"

# Load .env
if [ -f "${DEPLOY_DIR}/.env" ]; then
    export $(grep -v '^#' "${DEPLOY_DIR}/.env" | xargs) 2>/dev/null || true
fi

DOMAIN="${ORCA_DOMAIN:-b15.openledger.vn}"
# IP của Orca Server (container host) — 172.20.2.39
SERVER_IP="172.20.2.39"
# Dev machine (fleet member) — thêm vào SAN để developer truy cập trực tiếp
DEV_IP="172.20.2.31"

echo "======================================================"
echo " Generating self-signed TLS cert"
echo "======================================================"
echo "  Domain: ${DOMAIN}"
echo "  Output: ${CERTS_DIR}/"
echo "======================================================"
echo ""

mkdir -p "${CERTS_DIR}"

# Tạo cert với SAN (Subject Alternative Name) cho domain và localhost
openssl req -x509 -nodes -newkey rsa:4096 \
    -days 3650 \
    -keyout "${CERTS_DIR}/server.key" \
    -out    "${CERTS_DIR}/server.crt" \
    -subj   "/C=VN/ST=HCM/L=HoChiMinh/O=VNPBlc/CN=${DOMAIN}" \
    -addext "subjectAltName=DNS:${DOMAIN},DNS:localhost,IP:127.0.0.1,IP:${SERVER_IP},IP:${DEV_IP}"

echo ""
echo "✅ Certificate generated:"
echo "   ${CERTS_DIR}/server.crt"
echo "   ${CERTS_DIR}/server.key"
echo ""

# Hiển thị thông tin cert
openssl x509 -in "${CERTS_DIR}/server.crt" -noout -text | grep -A3 "Validity\|Subject:"

echo ""
echo "────────────────────────────────────────────────────────"
echo "  ⚠️  Self-signed cert — Developer cần trust cert này:"
echo ""
echo "  macOS:"
echo "    sudo security add-trusted-cert -d -r trustRoot \\"
echo "      -k /Library/Keychains/System.keychain \\"
echo "      ${CERTS_DIR}/server.crt"
echo ""
echo "  Ubuntu/Debian:"
echo "    sudo cp ${CERTS_DIR}/server.crt /usr/local/share/ca-certificates/${DOMAIN}.crt"
echo "    sudo update-ca-certificates"
echo ""
echo "  Chrome (temporary bypass):"
echo "    Mở URL → Advanced → Proceed anyway"
echo "────────────────────────────────────────────────────────"
