#!/bin/sh
set -e

ORCA_MULTI_USER="${ORCA_MULTI_USER:-0}"
ORCA_AUTH_MODE="${ORCA_AUTH_MODE:-none}"

echo "╔══════════════════════════════════╗"
echo "║        Orca Server               ║"
echo "╚══════════════════════════════════╝"
echo ""
echo "  Version:      ${ORCA_VERSION:-unknown}"
echo "  RPC Port:     ${ORCA_PORT:-6768}"
echo "  HTTP Port:    ${ORCA_HTTP_PORT:-6769}"
echo "  Data Dir:     ${ORCA_USER_DATA_PATH:-~/.orca}"
echo "  Node Env:     ${NODE_ENV:-production}"
echo "  Multi-User:   ${ORCA_MULTI_USER}"
echo "  Auth Mode:    ${ORCA_AUTH_MODE}"
if [ "${ORCA_MULTI_USER}" = "1" ]; then
  echo "  Admin Panel:  http://localhost:${ORCA_HTTP_PORT:-6769}/admin/"
fi
echo ""

# Create data directory if configured
if [ -n "$ORCA_USER_DATA_PATH" ]; then
  mkdir -p "$ORCA_USER_DATA_PATH"
  # Pre-create per-user data directory for multi-user isolation
  if [ "${ORCA_MULTI_USER}" = "1" ]; then
    mkdir -p "$ORCA_USER_DATA_PATH/users"
    echo "[entrypoint] Multi-user data dir: $ORCA_USER_DATA_PATH/users/"
  fi
  echo "[entrypoint] Data directory: $ORCA_USER_DATA_PATH"
fi

# Validate required files
if [ ! -f "out/server/index.js" ]; then
  echo "[entrypoint] ERROR: out/server/index.js not found!"
  echo "[entrypoint] Build server-side qua docker build (xem deploy/prod/Dockerfile)."
  exit 1
fi

echo "[entrypoint] Starting Orca server..."
echo ""

# Execute the command (default: node out/server/index.js)
exec "$@"

