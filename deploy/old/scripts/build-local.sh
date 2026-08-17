#!/usr/bin/env bash
# ============================================================
# build-local.sh — Pre-flight local build of backend/ + frontend/
# ============================================================
# Kể từ khi tách monorepo thành 4 package độc lập, việc build/deploy
# đã tách theo từng thư mục deploy/:
#
#   backend/ + frontend/  → build server-side qua Docker
#                            (docker-compose.yml Dockerfile tự build
#                             từ backend/ và frontend/ — xem docker/backend/Dockerfile).
#                            Script này chỉ chạy build LOCAL để kiểm tra
#                            lỗi sớm trước khi sync-to-server.sh + docker build.
#   agent/                → deploy/agent/scripts/deploy-agents.sh --build
#   desktop/               → deploy/desktop/scripts/build.sh
#
# Usage:
#   ./deploy/dev/scripts/build-local.sh     # build thử backend/ + frontend/ (pre-flight)
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

check_cmd() {
    command -v "$1" &>/dev/null || { echo "❌ ERROR: '$1' not found. Please install it first."; exit 1; }
}
check_cmd node
check_cmd pnpm

NODE_VERSION=$(node --version | sed 's/v//' | cut -d. -f1)
if [ "${NODE_VERSION}" -lt 22 ]; then
    echo "❌ ERROR: Node.js 22+ required (found v${NODE_VERSION})"
    exit 1
fi

cd "${REPO_ROOT}"

echo "======================================================"
echo " Orca Backend + Frontend — Local Pre-flight Build"
echo "======================================================"
echo "  Repo:    ${REPO_ROOT}"
echo "  Outputs: backend/out/server/   frontend/out/web/"
echo "  Note:    Deploy thật build lại server-side qua Docker"
echo "           (xem deploy/dev/docker/backend/Dockerfile)."
echo "           Script này chỉ để bắt lỗi sớm."
echo "======================================================"
echo ""

echo "[1/2] Building backend/ (vite build)..."
(cd "${REPO_ROOT}/backend" && [ -d node_modules ] || pnpm install)
(cd "${REPO_ROOT}/backend" && pnpm run build)

echo ""
echo "[2/2] Building frontend/ (vite build)..."
(cd "${REPO_ROOT}/frontend" && [ -d node_modules ] || pnpm install)
(cd "${REPO_ROOT}/frontend" && pnpm run build)

echo ""
echo "======================================================"
echo " ✅ Pre-flight build OK."
echo "   backend/out/server/:  $(du -sh "${REPO_ROOT}/backend/out/server" 2>/dev/null | cut -f1)"
echo "   frontend/out/web/:    $(du -sh "${REPO_ROOT}/frontend/out/web" 2>/dev/null | cut -f1)"
echo ""
echo "  Next step:"
echo "    ./deploy/dev/scripts/sync-to-server.sh <version>"
echo ""
echo "  Muốn build Agent hoặc Desktop?"
echo "    Agent:   pnpm run build:agent   (hoặc bash deploy/agent/scripts/deploy-agents.sh --build)"
echo "    Desktop: bash deploy/desktop/scripts/build.sh"
echo "======================================================"
