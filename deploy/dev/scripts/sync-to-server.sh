#!/usr/bin/env bash
# ============================================================
# sync-to-server.sh — rsync built binaries + run migrations + start stack
# ============================================================
# Step 2+3 of the deploy flow:
#   (1) build binary locally  →  (2) sync to server  →  (3) mount into container
#
# Unlike deploy/old/scripts/sync-to-server-artifact.sh, there is NO
# `docker compose build` step here at all — every image
# (gcr.io/distroless/static-debian12, nginx:1.27-alpine, postgres:16-alpine,
# hashicorp/vault, nats, migrate/migrate) is public and unmodified; the
# server only needs `docker compose pull` (cached after the first run) and
# the rsynced binaries/static assets bind-mounted in. This is what makes
# redeploys fast: no server-side compile, no server-side image build.
#
# Usage:
#   ./deploy/dev/scripts/sync-to-server.sh <version>
# Example:
#   ./deploy/dev/scripts/sync-to-server.sh 0.1.0
# ============================================================

set -euo pipefail

if [ "$#" -ne 1 ]; then
    echo "ERROR: Version argument is required to prevent accidental untracked deployments."
    echo "Usage: $0 <version>"
    exit 1
fi
ORCA_GO_VERSION="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${DEPLOY_DIR}/../.." && pwd)"

if [ -f "${DEPLOY_DIR}/.env" ]; then
    export $(grep -v '^#' "${DEPLOY_DIR}/.env" | xargs)
fi

SERVER_HOST="${SERVER_HOST:?ERROR: SERVER_HOST not set — see .env.example}"
SERVER_USER="${SERVER_USER:-ubuntu}"
SERVER_KEY="${SERVER_KEY:-${HOME}/.ssh/id_ed25519}"
SERVER_PORT="${SERVER_PORT:-22}"
SERVER_DEPLOY="${SERVER_DEPLOY:-~/orca-go-deploy}"

echo "======================================================"
echo " Orca backend-go — Sync & Deploy (mount-based, artifact mode)"
echo "======================================================"
echo "  Server:  ${SERVER_USER}@${SERVER_HOST}:${SERVER_PORT}"
echo "  Deploy:  ${SERVER_DEPLOY}"
echo "  Version: ${ORCA_GO_VERSION}"
echo "======================================================"
echo ""

SSH_OPTS="-i ${SERVER_KEY} -p ${SERVER_PORT} -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10"
ssh_cmd() { ssh ${SSH_OPTS} "${SERVER_USER}@${SERVER_HOST}" "$@"; }
rsync_cmd() { rsync -avz --progress -e "ssh ${SSH_OPTS}" "$@"; }

echo "[pre] Testing SSH connection..."
ssh_cmd "echo '✅ Connected to \$(hostname)'"
echo ""

echo "[1/5] Building backend-go binaries + frontend LOCALLY..."
ORCA_GO_VERSION="${ORCA_GO_VERSION}" bash "${SCRIPT_DIR}/build-local.sh"
echo ""

if [ ! -f "${DEPLOY_DIR}/bin/usage-service/orca" ]; then
    echo "❌ ERROR: build output missing (deploy/dev/bin/usage-service/orca) — aborting."
    exit 1
fi
if [ ! -d "${DEPLOY_DIR}/dist" ]; then
    echo "❌ ERROR: frontend build output missing (deploy/dev/dist/) — aborting."
    exit 1
fi

echo "[2/5] Syncing binaries + migrations + frontend + deploy config to server..."
ssh_cmd "mkdir -p ${SERVER_DEPLOY}"

# Binaries + per-service migrations (small — SQL files + a handful of ~10MB
# static Go binaries, nowhere near "the source tree").
rsync_cmd --delete "${DEPLOY_DIR}/bin/" "${SERVER_USER}@${SERVER_HOST}:${SERVER_DEPLOY}/bin/"

# Frontend static build.
rsync_cmd --delete "${DEPLOY_DIR}/dist/" "${SERVER_USER}@${SERVER_HOST}:${SERVER_DEPLOY}/dist/"

# Deploy config itself (compose file, nginx conf, scripts, postgres init) —
# excludes .env (synced separately below, never overwritten blindly) and
# bin/dist (already synced above with --delete, don't re-walk them here).
rsync_cmd --delete \
    --exclude '.env' \
    --exclude 'bin/' \
    --exclude 'dist/' \
    "${DEPLOY_DIR}/" \
    "${SERVER_USER}@${SERVER_HOST}:${SERVER_DEPLOY}/"

# .env: only push if the server doesn't have one yet — never silently
# overwrite server-side secrets (VAULT_TOKEN, POSTGRES_PASSWORD, ...) with
# whatever's in the local working copy.
if ! ssh_cmd "test -f ${SERVER_DEPLOY}/.env"; then
    echo "  (no .env on server yet — pushing local one; edit it on the server after this)"
    rsync_cmd "${DEPLOY_DIR}/.env" "${SERVER_USER}@${SERVER_HOST}:${SERVER_DEPLOY}/.env"
else
    echo "  (server already has .env — leaving it untouched)"
fi

echo "✅ Synced"
echo ""

echo "[3/5] Pulling public images on server (cached after first run)..."
ssh_cmd "cd ${SERVER_DEPLOY} && docker compose pull postgres vault nats frontend"
echo ""

echo "[4/5] Running migrations (profile: migrate) for every DB-owning service..."
ssh_cmd "cd ${SERVER_DEPLOY} && docker compose up -d postgres vault nats && sleep 5"
bash "${SCRIPT_DIR}/migrate.sh" --remote
echo ""

echo "[5/5] Starting/recreating the full stack..."
ssh_cmd "cd ${SERVER_DEPLOY} && docker compose up -d --force-recreate --remove-orphans"
echo ""

echo "Health check (waiting 15s for startup)..."
sleep 15
if ssh_cmd "docker exec orca-go-frontend wget -qO- http://127.0.0.1:8080/healthz" 2>/dev/null | grep -q "ok"; then
    echo "✅ Frontend health check PASSED"
else
    echo "⚠️  Frontend health check inconclusive — fetching logs..."
fi
ssh_cmd "docker compose -f ${SERVER_DEPLOY}/docker-compose.yml ps"

echo ""
echo "======================================================"
echo " ✅ Deploy complete."
echo "======================================================"
