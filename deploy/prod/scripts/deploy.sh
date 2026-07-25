#!/bin/bash
# Deploy Orca Server to production server
#
# Usage:
#   bash deploy/prod/scripts/deploy.sh [version]
#   ORCA_REMOTE_HOST=user@myserver.com bash deploy/prod/scripts/deploy.sh 1.2.3
#
# Required environment:
#   ORCA_REMOTE_HOST  - SSH target (default: ubuntu@172.20.2.39)
#   ORCA_DEPLOY_DIR   - Deploy directory on remote (default: ~/orca-deploy)
#
# Optional:
#   ORCA_REGISTRY     - Docker registry prefix (default: local transfer via ssh)

set -euo pipefail

# ── Configuration ─────────────────────────────────────────────────────────────
ORCA_VERSION=${1:-$(PATH="$HOME/.nvm/versions/node/v20.20.2/bin:$PATH" node -p "require('./package.json').version" 2>/dev/null || echo "0.0.0")}
REMOTE_HOST=${ORCA_REMOTE_HOST:-ubuntu@172.20.2.39}
DEPLOY_DIR=${ORCA_DEPLOY_DIR:-~/orca-deploy}
IMAGE_NAME="vnpblc/orca-server"
IMAGE_TAG="${IMAGE_NAME}:${ORCA_VERSION}"
IMAGE_LATEST="${IMAGE_NAME}:latest"

echo ""
echo "╔══════════════════════════════════════════╗"
echo "║   Orca Server Deploy: v${ORCA_VERSION}   ║"
echo "╚══════════════════════════════════════════╝"
echo ""
echo "  Target:  ${REMOTE_HOST}:${DEPLOY_DIR}"
echo "  Image:   ${IMAGE_TAG}"
echo ""

# ── Step 1: Build Docker image ────────────────────────────────────────────────
echo "▶ [1/4] Building Docker image..."
docker buildx build \
    --platform linux/amd64 \
    --tag "${IMAGE_TAG}" \
    --tag "${IMAGE_LATEST}" \
    --file deploy/prod/Dockerfile \
    .
echo "  ✅ Image built: ${IMAGE_TAG}"

# ── Step 2: Transfer image to remote ─────────────────────────────────────────
echo ""
echo "▶ [2/4] Transferring image to ${REMOTE_HOST}..."
docker save "${IMAGE_TAG}" \
    | gzip \
    | ssh "${REMOTE_HOST}" "mkdir -p ${DEPLOY_DIR} && gunzip | docker load"
echo "  ✅ Image transferred"

# ── Step 3: Deploy on remote ──────────────────────────────────────────────────
echo ""
echo "▶ [3/4] Deploying on remote..."

# Transfer compose file and .env.example
scp deploy/prod/docker-compose.yml "${REMOTE_HOST}:${DEPLOY_DIR}/docker-compose.yml"

# Write .env on remote
ssh "${REMOTE_HOST}" bash <<REMOTE_SCRIPT
cd ${DEPLOY_DIR}

# Create .env if not exists
if [ ! -f .env ]; then
  cat > .env <<EOF
ORCA_VERSION=${ORCA_VERSION}
ORCA_IMAGE=${IMAGE_NAME}
ORCA_RPC_PORT=6768
ORCA_WEB_PORT=6769
EOF
  echo "[remote] Created .env"
else
  # Update version in existing .env
  sed -i "s/^ORCA_VERSION=.*/ORCA_VERSION=${ORCA_VERSION}/" .env
  echo "[remote] Updated ORCA_VERSION in .env"
fi

# Restart container (no-build — image already loaded)
ORCA_VERSION=${ORCA_VERSION} docker compose up -d --no-build
echo "[remote] Container restarted"
REMOTE_SCRIPT

echo "  ✅ Deployed"

# ── Step 4: Health check ──────────────────────────────────────────────────────
echo ""
echo "▶ [4/4] Health check (waiting 15s for startup)..."
sleep 15

HTTP_PORT=${ORCA_WEB_PORT:-6769}

if ssh "${REMOTE_HOST}" "curl -sf http://localhost:${HTTP_PORT}/ > /dev/null 2>&1"; then
    echo "  ✅ Health check PASSED (HTTP :${HTTP_PORT} responding)"
else
    echo "  ❌ Health check FAILED — printing last 50 log lines:"
    ssh "${REMOTE_HOST}" "docker logs orca-server --tail 50" || true
    exit 1
fi

# ── Done ──────────────────────────────────────────────────────────────────────
echo ""
echo "╔══════════════════════════════════════════════╗"
echo "║   ✅ Deploy Complete: v${ORCA_VERSION}        ║"
echo "╚══════════════════════════════════════════════╝"
echo ""
echo "  Web UI:  http://$(ssh ${REMOTE_HOST} hostname -I 2>/dev/null | awk '{print $1}' || echo '<remote>'):${HTTP_PORT}"
echo "  RPC:     ws://$(ssh ${REMOTE_HOST} hostname -I 2>/dev/null | awk '{print $1}' || echo '<remote>'):${ORCA_RPC_PORT:-6768}"
echo ""
