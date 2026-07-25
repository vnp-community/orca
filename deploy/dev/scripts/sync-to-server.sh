#!/usr/bin/env bash
# ============================================================
# sync-to-server.sh — Sync Orca source code & deploy (Node.js Server Mode)
# ============================================================
# Dùng rsync để đẩy toàn bộ mã nguồn lên server (trừ thư mục rác).
# Trên server, Docker sẽ tự động build từ source thành container.
#
# Usage:
#   ./deploy/dev/scripts/sync-to-server.sh
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="${SCRIPT_DIR}/.."
REPO_ROOT="${DEPLOY_DIR}/../.."

# ── Load .env nếu có ─────────────────────────────────────────
if [ -f "${DEPLOY_DIR}/.env" ]; then
    export $(grep -v '^#' "${DEPLOY_DIR}/.env" | xargs)
fi

SERVER_HOST="${SERVER_HOST:?ERROR: SERVER_HOST not set}"
SERVER_USER="${SERVER_USER:-ubuntu}"
SERVER_KEY="${SERVER_KEY:-${HOME}/.ssh/id_ed25519}"
SERVER_PORT="${SERVER_PORT:-22}"
SERVER_DEPLOY="${SERVER_DEPLOY:-~/orca-deploy}"

echo "======================================================"
echo " Orca Node.js Sync & Deploy"
echo "======================================================"
echo "  Server: ${SERVER_USER}@${SERVER_HOST}:${SERVER_PORT}"
echo "  Deploy: ${SERVER_DEPLOY}"
echo "  Mode:   server (Node.js + Web SPA, no Electron)"
echo "======================================================"
echo ""

SSH_OPTS="-i ${SERVER_KEY} -p ${SERVER_PORT} -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10"

ssh_cmd() {
    ssh ${SSH_OPTS} "${SERVER_USER}@${SERVER_HOST}" "$@"
}

echo "[pre] Testing SSH connection..."
ssh_cmd "echo '✅ Connected to $(hostname)'"
echo ""

echo "[1/3] Syncing source code to server..."
# Exclude node_modules, out, dist, .git to save bandwidth and avoid native binding issues
rsync -avz --progress --delete \
    -e "ssh ${SSH_OPTS}" \
    --exclude 'node_modules' \
    --exclude 'out' \
    --exclude 'dist' \
    --exclude '.git' \
    --exclude '.vscode' \
    --exclude 'build' \
    --exclude '.codegraph' \
    --exclude '.gitnexus' \
    "${REPO_ROOT}/" \
    "${SERVER_USER}@${SERVER_HOST}:${SERVER_DEPLOY}/"

echo "✅ Source synced"
echo ""

echo "[2/3] Building and Deploying via Docker Compose on server..."
ssh_cmd "cd ${SERVER_DEPLOY}/deploy/dev && docker compose -f docker-compose.orca.yml up -d --build --force-recreate"

echo ""
echo "[3/3] Health check (waiting 20s for startup)..."
sleep 20
ORCA_HTTP_PORT="${ORCA_HTTP_PORT:-6769}"
if ssh_cmd "wget -qO- http://127.0.0.1:${ORCA_HTTP_PORT}/health/ready" 2>/dev/null | grep -q .; then
  echo "✅ Health check PASSED (HTTP :${ORCA_HTTP_PORT}/health/ready)"
else
  echo "⚠️  Health check: container may still be starting. Fetching logs..."
  ssh_cmd "docker logs orca-server 2>&1 | tail -n 20"
fi

echo ""
echo "======================================================"
echo " ✅ Deploy complete! Fetching logs..."
echo "======================================================"
sleep 5
ssh_cmd "docker logs orca-server 2>&1 | tail -n 15"
