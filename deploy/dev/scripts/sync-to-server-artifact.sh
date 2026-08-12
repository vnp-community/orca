#!/usr/bin/env bash
# ============================================================
# sync-to-server-artifact.sh — Build LOCALLY, ship ARTIFACTS only
# ============================================================
# Alternative to sync-to-server.sh: that script rsyncs the whole app source
# tree and lets Docker build backend/+frontend/ FROM SOURCE on the server
# (see docker-compose.orca.yml + docker/backend/Dockerfile). This script
# builds backend/out/server + frontend/out/web LOCALLY first, then rsyncs
# only the build output + the manifests `pnpm install --prod` needs —
# never the source tree — and has the server run a single-stage image
# (docker-compose.orca.artifact.yml + Dockerfile.artifact) that just
# installs prod deps and copies the pre-built artifacts in.
#
# Usage:
#   ./deploy/dev/scripts/sync-to-server-artifact.sh <version>
# Example:
#   ./deploy/dev/scripts/sync-to-server-artifact.sh 1.4.138-rc.6
# ============================================================

set -euo pipefail

if [ "$#" -ne 1 ]; then
    echo "ERROR: Version argument is required to prevent accidental untracked deployments."
    echo "Usage: $0 <version>"
    echo "Example: $0 1.4.138-rc.6"
    exit 1
fi

ORCA_VERSION="$1"

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
echo " Orca Node.js Sync & Deploy — ARTIFACT MODE"
echo "======================================================"
echo "  Server: ${SERVER_USER}@${SERVER_HOST}:${SERVER_PORT}"
echo "  Deploy: ${SERVER_DEPLOY}"
echo "  Mode:   artifact (built locally, no server-side compile)"
echo "  Version: ${ORCA_VERSION}"
echo "======================================================"
echo ""

SSH_OPTS="-i ${SERVER_KEY} -p ${SERVER_PORT} -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10"

ssh_cmd() {
    ssh ${SSH_OPTS} "${SERVER_USER}@${SERVER_HOST}" "$@"
}

echo "[pre] Testing SSH connection..."
ssh_cmd "echo '✅ Connected to $(hostname)'"
echo ""

echo "[1/4] Building backend/ + frontend/ LOCALLY..."
bash "${SCRIPT_DIR}/build-local.sh"
echo ""

if [ ! -f "${REPO_ROOT}/backend/out/server/index.js" ]; then
    echo "❌ ERROR: backend/out/server/index.js not found after build-local.sh — aborting."
    exit 1
fi
if [ ! -d "${REPO_ROOT}/frontend/out/web" ]; then
    echo "❌ ERROR: frontend/out/web/ not found after build-local.sh — aborting."
    exit 1
fi

echo "[2/4] Syncing build artifacts + manifests to server (NO source tree)..."
ssh_cmd "mkdir -p ${SERVER_DEPLOY}/backend/out ${SERVER_DEPLOY}/frontend/out ${SERVER_DEPLOY}/desktop/config"

# Manifests pnpm install --prod needs (small, not "source")
rsync -avz --progress \
    -e "ssh ${SSH_OPTS}" \
    "${REPO_ROOT}/package.json" \
    "${REPO_ROOT}/pnpm-lock.yaml" \
    "${REPO_ROOT}/pnpm-workspace.yaml" \
    "${REPO_ROOT}/.npmrc" \
    "${SERVER_USER}@${SERVER_HOST}:${SERVER_DEPLOY}/"

rsync -avz --progress \
    -e "ssh ${SSH_OPTS}" \
    "${REPO_ROOT}/backend/package.json" \
    "${SERVER_USER}@${SERVER_HOST}:${SERVER_DEPLOY}/backend/package.json"

rsync -avz --progress --delete \
    -e "ssh ${SSH_OPTS}" \
    "${REPO_ROOT}/desktop/config/patches/" \
    "${SERVER_USER}@${SERVER_HOST}:${SERVER_DEPLOY}/desktop/config/patches/"

# Build artifacts
rsync -avz --progress --delete \
    -e "ssh ${SSH_OPTS}" \
    "${REPO_ROOT}/backend/out/server/" \
    "${SERVER_USER}@${SERVER_HOST}:${SERVER_DEPLOY}/backend/out/server/"

rsync -avz --progress --delete \
    -e "ssh ${SSH_OPTS}" \
    "${REPO_ROOT}/frontend/out/web/" \
    "${SERVER_USER}@${SERVER_HOST}:${SERVER_DEPLOY}/frontend/out/web/"

# Deploy config itself (compose files, Dockerfile.artifact, entrypoint,
# nginx, .env) — small, versioned deploy/dev/ folder, not app source.
rsync -avz --progress --delete \
    --exclude 'docker/backend/ssh' \
    --exclude 'docker/nginx/certs' \
    -e "ssh ${SSH_OPTS}" \
    "${REPO_ROOT}/deploy/dev/" \
    "${SERVER_USER}@${SERVER_HOST}:${SERVER_DEPLOY}/deploy/dev/"

echo "✅ Artifacts + manifests synced"
echo ""

echo "[3/4] Building and deploying via Docker Compose (artifact mode) on server..."
ssh_cmd "cd ${SERVER_DEPLOY}/deploy/dev && ORCA_VERSION=${ORCA_VERSION} docker compose -f docker-compose.orca.artifact.yml build orca && ORCA_VERSION=${ORCA_VERSION} docker compose -f docker-compose.orca.artifact.yml up -d --force-recreate"

echo ""
echo "[4/4] Health check (waiting 20s for startup)..."
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
