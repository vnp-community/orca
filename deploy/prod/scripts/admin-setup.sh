#!/bin/bash
# ============================================================
# admin-setup.sh — Thiết lập admin user cho Orca Server
# ============================================================
# Dùng khi ORCA_MULTI_USER=1 để tạo hoặc reset admin credentials
#
# Usage:
#   ./admin-setup.sh                          → tạo admin với random password
#   ./admin-setup.sh -e admin@company.com     → custom email
#   ./admin-setup.sh -e admin@company.com -p StrongPass123
#   ./admin-setup.sh --reset                  → reset admin password
#
# Requires: docker container "orca-server" running
# ============================================================

set -euo pipefail

CONTAINER="${ORCA_CONTAINER_NAME:-orca-server}"
ADMIN_EMAIL=""
ADMIN_PASSWORD=""
RESET_ONLY=false

# Parse args
while [[ $# -gt 0 ]]; do
  case "$1" in
    -e|--email)    ADMIN_EMAIL="$2";    shift 2 ;;
    -p|--password) ADMIN_PASSWORD="$2"; shift 2 ;;
    --reset)       RESET_ONLY=true;     shift   ;;
    -h|--help)
      echo "Usage: $0 [-e email] [-p password] [--reset]"
      exit 0 ;;
    *) echo "Unknown arg: $1"; exit 1 ;;
  esac
done

# Check container running
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
  echo "ERROR: Container '${CONTAINER}' not running."
  echo "Start it first: docker compose -f deploy/prod/docker-compose.yml up -d"
  exit 1
fi

# Build node script
if [ "$RESET_ONLY" = true ]; then
  echo "[admin-setup] Resetting admin password..."
  NODE_SCRIPT="
const { ensureFirstAdminUser } = require('./out/server/admin/first-run-setup');
ensureFirstAdminUser({ reset: true, email: '${ADMIN_EMAIL}', password: '${ADMIN_PASSWORD}' })
  .then(() => process.exit(0))
  .catch(e => { console.error(e); process.exit(1); });
"
else
  echo "[admin-setup] Ensuring admin user exists..."
  NODE_SCRIPT="
const { ensureFirstAdminUser } = require('./out/server/admin/first-run-setup');
ensureFirstAdminUser({ email: '${ADMIN_EMAIL}', password: '${ADMIN_PASSWORD}' })
  .then(() => process.exit(0))
  .catch(e => { console.error(e); process.exit(1); });
"
fi

docker exec -i "${CONTAINER}" node -e "${NODE_SCRIPT}"
echo "[admin-setup] Done."
