#!/usr/bin/env bash
# ============================================================
# build-local.sh — Build Orca locally (chạy trên máy developer)
# ============================================================
#
# Modes:
#
#   [Default] Server mode — build Node.js server để deploy Docker (~2-3 phút):
#     out/relay/      ← Relay binary (relay.js per platform)
#     out/server/     ← Node.js backend (vite.server.config.ts)
#     out/web/        ← Web SPA (vite.web-spa.config.ts)
#     → KHÔNG cần Electron
#
#   [--desktop] Desktop mode — build server + Electron app (~15-20 phút):
#     (tất cả các bước server mode ở trên)
#     + out/main/     ← Electron main process (electron-vite)
#     + out/preload/  ← Electron preload scripts
#     + dist/linux-unpacked/ ← Packaged Electron app
#
#   [--agent-only] Agent-only — chỉ build agent.js (~3 giây):
#     out/relay/agent.js ← Standalone Node.js bundle (~185KB)
#
# Usage:
#   cd /path/to/orca
#   ./deploy/dev/scripts/build-local.sh              # server mode (no Electron)
#   ./deploy/dev/scripts/build-local.sh --desktop    # server + Electron desktop app
#   ./deploy/dev/scripts/build-local.sh --agent-only # chỉ agent.js
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
DEPLOY_DIR="${SCRIPT_DIR}/.."
DIST_DIR="${DEPLOY_DIR}/dist"

AGENT_ONLY=false
DESKTOP=false
for arg in "$@"; do
    [[ "${arg}" == "--agent-only" ]] && AGENT_ONLY=true
    [[ "${arg}" == "--desktop" ]]    && DESKTOP=true
done

# ── check_cmd helper ─────────────────────────────────────────
check_cmd() {
    if ! command -v "$1" &>/dev/null; then
        echo "❌ ERROR: '$1' not found. Please install it first."
        exit 1
    fi
}

# ────────────────────────────────────────────────────────────
# FAST PATH: Agent-only build (no Electron, ~3 seconds)
# ────────────────────────────────────────────────────────────
if [[ "${AGENT_ONLY}" == "true" ]]; then
    echo "======================================================"
    echo " Orca Agent-Only Build (no Electron)"
    echo "======================================================"
    echo "  Repo:   ${REPO_ROOT}"
    echo "  Output: out/relay/agent.js  (~185KB)"
    echo "  Note:   NO electron-builder, NO Electron download"
    echo "======================================================"
    echo ""

    check_cmd node
    NODE_VERSION=$(node --version | sed 's/v//' | cut -d. -f1)
    if [ "${NODE_VERSION}" -lt 18 ]; then
        echo "❌ ERROR: Node.js 18+ required (found v${NODE_VERSION})"
        exit 1
    fi
    echo "✅ Node.js: $(node --version)"
    echo ""

    cd "${REPO_ROOT}"

    if [ ! -d "node_modules" ]; then
        echo "[1/2] Installing dependencies..."
        if command -v pnpm &>/dev/null; then
            pnpm install --frozen-lockfile 2>/dev/null || npm install
        else
            npm install
        fi
    else
        echo "[1/2] Dependencies already installed. Skipping."
    fi

    echo ""
    echo "[2/2] Building agent.js (esbuild, no Electron)..."
    node config/scripts/build-agent-only.mjs

    echo ""
    echo "  Next step:"
    echo "    bash deploy/dev/scripts/deploy-agents.sh"
    echo ""
    exit 0
fi

# ────────────────────────────────────────────────────────────
# SERVER / DESKTOP: Common prerequisites
# ────────────────────────────────────────────────────────────

check_cmd node
check_cmd pnpm

NODE_VERSION=$(node --version | sed 's/v//' | cut -d. -f1)
if [ "${NODE_VERSION}" -lt 22 ]; then
    echo "❌ ERROR: Node.js 22+ required (found v${NODE_VERSION})"
    exit 1
fi

echo "✅ Node.js: $(node --version)"
echo "✅ pnpm:    $(pnpm --version)"
echo ""

cd "${REPO_ROOT}"

# ────────────────────────────────────────────────────────────
# Print mode header
# ────────────────────────────────────────────────────────────
if [[ "${DESKTOP}" == "true" ]]; then
    TOTAL_STEPS=5
    echo "======================================================"
    echo " Orca Desktop Build (Server + Electron, ~15-20 phút)"
    echo "======================================================"
    echo "  Repo:    ${REPO_ROOT}"
    echo "  Outputs: out/server/  out/web/  out/relay/"
    echo "           out/main/    out/preload/  dist/linux-unpacked/"
    echo "  Note:    Electron-builder sẽ package Linux unpacked"
    echo "======================================================"
else
    TOTAL_STEPS=3
    echo "======================================================"
    echo " Orca Server Build (Node.js, no Electron, ~2-3 phút)"
    echo "======================================================"
    echo "  Repo:    ${REPO_ROOT}"
    echo "  Outputs: out/relay/  out/server/  out/web/"
    echo "  Note:    Dùng pnpm run build:server"
    echo "           Không cần Electron runtime"
    echo "======================================================"
fi
echo ""

# ────────────────────────────────────────────────────────────
# Step 1: Install dependencies
# ────────────────────────────────────────────────────────────
if [ ! -d "node_modules" ]; then
    echo "[1/${TOTAL_STEPS}] Installing dependencies..."
    pnpm install
else
    echo "[1/${TOTAL_STEPS}] Dependencies already installed. Skipping."
fi

# ────────────────────────────────────────────────────────────
# Step 2: Server build (relay + backend + web SPA) — luôn chạy
# ────────────────────────────────────────────────────────────
echo ""
echo "[2/${TOTAL_STEPS}] Building server artifacts (relay + backend + web)..."
echo "  → Building Dev Server Agent (relay binaries, all platforms)..."
pnpm run build:dev-server

echo "  → Building Node.js backend (vite.server.config.ts → out/server/)..."
pnpm run build:backend

echo "  → Building Web SPA (vite.web-spa.config.ts → out/web/)..."
pnpm run build:frontend:web

echo ""
echo "✅ Server build complete."
echo "  out/relay/   — relay binaries"
echo "  out/server/  — Node.js backend"
echo "  out/web/     — Web SPA"

# ────────────────────────────────────────────────────────────
# Step 3 (Server mode only): Done — next step sync
# ────────────────────────────────────────────────────────────
if [[ "${DESKTOP}" == "false" ]]; then
    echo ""
    echo "[3/${TOTAL_STEPS}] Verifying artifacts..."
    echo "  relay:   $(ls out/relay/*.js out/relay/**/*.js 2>/dev/null | wc -l | tr -d ' ') files"
    echo "  server:  $(du -sh out/server/ 2>/dev/null | cut -f1)"
    echo "  web:     $(du -sh out/web/ 2>/dev/null | cut -f1)"
    echo ""
    echo "======================================================"
    echo " ✅ Build done! (Server mode — no Electron)"
    echo "======================================================"
    echo ""
    echo "  Next step:"
    echo "    ./deploy/dev/scripts/sync-to-server.sh"
    echo ""
    exit 0
fi

# ────────────────────────────────────────────────────────────
# Step 3 (Desktop mode): Build Electron main + renderer
# ────────────────────────────────────────────────────────────
echo ""
echo "[3/${TOTAL_STEPS}] Building Electron main + renderer (electron-vite → out/main/, out/preload/)..."
pnpm run build:electron-vite

echo ""
echo "✅ Electron main built."

# ────────────────────────────────────────────────────────────
# Step 4 (Desktop mode): Package Linux unpacked via electron-builder
# ────────────────────────────────────────────────────────────
echo ""
echo "[4/${TOTAL_STEPS}] Packaging Linux unpacked (electron-builder --linux --dir)..."

# Đảm bảo Electron runtime có sẵn
pnpm run ensure:electron-runtime

# Build Linux unpacked directory (KHÔNG tạo AppImage/deb, chỉ dir)
npx electron-builder \
    --config config/electron-builder.config.cjs \
    --linux \
    --dir

echo ""
echo "✅ Packaged to: dist/linux-unpacked/"

# ────────────────────────────────────────────────────────────
# Step 5 (Desktop mode): Copy dist vào deploy dir
# ────────────────────────────────────────────────────────────
echo ""
echo "[5/${TOTAL_STEPS}] Copying dist to deploy directory..."

mkdir -p "${DIST_DIR}"
rm -rf "${DIST_DIR}/linux-unpacked"
cp -r "${REPO_ROOT}/dist/linux-unpacked" "${DIST_DIR}/linux-unpacked"

echo ""
echo "======================================================"
echo " ✅ Build done! (Desktop mode — Server + Electron)"
echo "======================================================"
echo ""
echo "  Server artifacts:"
echo "    out/relay/   out/server/   out/web/"
echo ""
echo "  Desktop artifacts:"
echo "    out/main/    out/preload/"
echo "    dist/linux-unpacked/ ($(du -sh "${DIST_DIR}/linux-unpacked" | cut -f1))"
echo ""
echo "  Next step:"
echo "    ./deploy/dev/scripts/sync-to-server.sh"
echo ""
