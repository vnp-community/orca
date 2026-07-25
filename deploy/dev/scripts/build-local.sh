#!/usr/bin/env bash
# ============================================================
# build-local.sh — Build Orca locally (chạy trên máy developer)
# ============================================================
# Output:
#   dist/linux-unpacked/   ← Electron app compiled for Linux
#
# Requirements trên máy local:
#   - Node.js 22+
#   - pnpm
#   - Docker cross-platform build tools (nếu local là macOS/Windows)
#     → Dùng electron-builder cross-compile target linux
#
# Usage:
#   cd /path/to/orca          # root của repo Orca
#   ./deploy/dev/scripts/build-local.sh
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
DEPLOY_DIR="${SCRIPT_DIR}/.."
DIST_DIR="${DEPLOY_DIR}/dist"

echo "======================================================"
echo " Orca Local Build Script"
echo "======================================================"
echo "  Repo:   ${REPO_ROOT}"
echo "  Output: ${DIST_DIR}/linux-unpacked/"
echo "======================================================"
echo ""

# ── Kiểm tra prerequisites ───────────────────────────────────
check_cmd() {
    if ! command -v "$1" &>/dev/null; then
        echo "❌ ERROR: '$1' not found. Please install it first."
        exit 1
    fi
}

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

# ── Step 1: Install dependencies (nếu chưa có) ──────────────
cd "${REPO_ROOT}"

if [ ! -d "node_modules" ]; then
    echo "[1/4] Installing dependencies..."
    pnpm install
else
    echo "[1/4] Dependencies already installed. Skipping."
fi

# ── Step 2: Build artifacts ──────────────────────────────────
echo ""
echo "[2/4] Building Orca..."

# Build relay binary
echo "  → Building relay..."
pnpm run build:relay

# Build CLI (TypeScript → JS)
echo "  → Building CLI..."
pnpm run build:cli

# Build main process + renderer (electron-vite)
echo "  → Building Electron main + renderer..."
pnpm run build:electron-vite

# Build web UI bundle (React SPA)
echo "  → Building Web UI..."
pnpm run build:web

echo ""
echo "✅ Build complete. Artifacts in: out/"

# ── Step 3: Package Linux unpacked (không cần AppImage wrapper) ─
echo ""
echo "[3/4] Packaging Linux unpacked..."

# Đảm bảo electron runtime có
pnpm run ensure:electron-runtime

# Build Linux unpacked directory (KHÔNG tạo AppImage/deb, chỉ dir)
# --dir = unpacked directory only (nhỏ hơn, đủ để chạy trong container)
npx electron-builder \
    --config config/electron-builder.config.cjs \
    --linux \
    --dir

echo ""
echo "✅ Packaged to: dist/linux-unpacked/"

# ── Step 4: Copy dist vào deploy dir ────────────────────────
echo ""
echo "[4/4] Copying dist to deploy directory..."

mkdir -p "${DIST_DIR}"

# Xoá dist cũ
rm -rf "${DIST_DIR}/linux-unpacked"

# Copy mới
cp -r "${REPO_ROOT}/dist/linux-unpacked" "${DIST_DIR}/linux-unpacked"

echo ""
echo "======================================================"
echo " ✅ Build done!"
echo "======================================================"
echo ""
echo "  Artifacts:"
ls -lh "${DIST_DIR}/linux-unpacked" | head -10
echo "  ..."
echo ""
echo "  Size: $(du -sh "${DIST_DIR}/linux-unpacked" | cut -f1)"
echo ""
echo "  Next step:"
echo "    ./deploy/dev/scripts/sync-to-server.sh"
echo ""
