#!/bin/bash
# ============================================================
# Orca Container Entrypoint (Pure Node.js)
# ============================================================
# ENV vars:
#   ORCA_DOMAIN          : domain mà client dùng để kết nối (wss://DOMAIN)
#   ORCA_PORT            : WebSocket/RPC port (default: 6768)
#   ORCA_HTTP_PORT       : HTTP static files + /health/* (default: 6769)
#   ORCA_USER_DATA_PATH  : data directory (default: /data/orca)
#   ORCA_VERSION         : image version tag
#   NODE_ENV             : node environment (default: production)
#   ORCA_MULTI_USER      : '1' = enable multi-user mode (default: 0)
#   ORCA_AUTH_MODE       : 'none' | 'local' | 'sso' (default: none)
#   ORCA_ADMIN_EMAIL     : first admin email (optional — auto-generated if empty)
#   ORCA_ADMIN_PASSWORD  : first admin password (optional — auto-generated if empty)
#   ORCA_FLEET_METRICS_ENABLED : 'true' = expose /metrics (default: false)
# ============================================================

set -euo pipefail

export ORCA_PORT="${ORCA_PORT:-6768}"
export ORCA_HTTP_PORT="${ORCA_HTTP_PORT:-6769}"
export ORCA_DOMAIN="${ORCA_DOMAIN:-localhost}"
export ORCA_USER_DATA_PATH="${ORCA_USER_DATA_PATH:-/data/orca}"
export NODE_ENV="${NODE_ENV:-production}"
export ORCA_MULTI_USER="${ORCA_MULTI_USER:-0}"
export ORCA_AUTH_MODE="${ORCA_AUTH_MODE:-none}"

# ── Paths ────────────────────────────────────────────────────
ORCA_APP_DIR="/opt/orca/app"

echo "╔══════════════════════════════════╗"
echo "║        Orca Server               ║"
echo "╚══════════════════════════════════╝"
echo ""
echo "  Version:      ${ORCA_VERSION:-unknown}"
echo "  RPC Port:     ${ORCA_PORT}"
echo "  HTTP Port:    ${ORCA_HTTP_PORT}"
echo "  Data Dir:     ${ORCA_USER_DATA_PATH}"
echo "  Domain:       ${ORCA_DOMAIN}"
echo "  Node Env:     ${NODE_ENV}"
echo "  Multi-User:   ${ORCA_MULTI_USER}"
echo "  Auth Mode:    ${ORCA_AUTH_MODE}"
if [ "${ORCA_MULTI_USER}" = "1" ]; then
  echo "  Admin URL:    http://localhost:${ORCA_HTTP_PORT}/admin/"
fi
echo ""

# Create data directory if not exists
if [ -n "${ORCA_USER_DATA_PATH}" ]; then
  mkdir -p "${ORCA_USER_DATA_PATH}"
  # Pre-create users/ subdirectory for multi-user process isolation
  if [ "${ORCA_MULTI_USER}" = "1" ]; then
    mkdir -p "${ORCA_USER_DATA_PATH}/users"
    echo "[entrypoint] Multi-user data dir: ${ORCA_USER_DATA_PATH}/users/"
  fi
  echo "[entrypoint] Data directory: ${ORCA_USER_DATA_PATH}"
fi

# ── SSH key setup ──────────────────────────────────────────────────────────
# Why: /home/orca/.ssh is a read-only bind-mount owned by ubuntu (uid=1000).
# SSH refuses to read config/key files not owned by the current user (orca, uid=999).
# Fix: copy SSH files to a writable directory and update HOME so SSH uses them.
ORCA_SSH_SRC="/home/orca/.ssh"
ORCA_SSH_DEST="/data/orca/.ssh"
if [ -d "${ORCA_SSH_SRC}" ] && [ "$(ls -A ${ORCA_SSH_SRC} 2>/dev/null)" ]; then
  mkdir -p "${ORCA_SSH_DEST}"
  cp -n "${ORCA_SSH_SRC}/id_ed25519"     "${ORCA_SSH_DEST}/id_ed25519"     2>/dev/null || true
  cp    "${ORCA_SSH_SRC}/id_ed25519.pub" "${ORCA_SSH_DEST}/id_ed25519.pub" 2>/dev/null || true
  cp    "${ORCA_SSH_SRC}/known_hosts"    "${ORCA_SSH_DEST}/known_hosts"    2>/dev/null || true
  # Rewrite config to point to /data/orca/.ssh paths (owned by orca)
  if [ -f "${ORCA_SSH_SRC}/config" ] || [ -f "${ORCA_SSH_SRC}/config.bak" ]; then
    SRC_CFG="${ORCA_SSH_SRC}/config"
    [ ! -f "${SRC_CFG}" ] && SRC_CFG="${ORCA_SSH_SRC}/config.bak"
    sed "s|/home/orca/\.ssh/|${ORCA_SSH_DEST}/|g" "${SRC_CFG}" > "${ORCA_SSH_DEST}/config"
  fi
  chmod 700 "${ORCA_SSH_DEST}"
  chmod 600 "${ORCA_SSH_DEST}/id_ed25519" 2>/dev/null || true
  chmod 644 "${ORCA_SSH_DEST}/id_ed25519.pub" "${ORCA_SSH_DEST}/known_hosts" 2>/dev/null || true
  chmod 600 "${ORCA_SSH_DEST}/config" 2>/dev/null || true
  export HOME="/data/orca"
  echo "[entrypoint] SSH keys copied to ${ORCA_SSH_DEST} (HOME=${HOME})"
else
  echo "[entrypoint] No SSH keys found at ${ORCA_SSH_SRC} — skipping SSH setup"
fi

# Validate required files
if [ ! -f "${ORCA_APP_DIR}/out/server/index.js" ]; then
  echo "[entrypoint] ERROR: ${ORCA_APP_DIR}/out/server/index.js not found!"
  echo "[entrypoint] Build server-side qua docker compose build (xem deploy/dev/docker/backend/Dockerfile), hoặc chạy deploy/dev/scripts/build-local.sh để test local."
  exit 1
fi

echo "[entrypoint] Starting Orca server..."
echo ""

cd "${ORCA_APP_DIR}"
exec node out/server/index.js

