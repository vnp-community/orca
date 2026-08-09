#!/usr/bin/env bash
# ============================================================
# build.sh — Build & package Orca Desktop từ desktop/
# ============================================================
# desktop/ là package Electron tự chứa (đã tách khỏi monorepo gốc,
# xem deploy/desktop/README.md). Script này:
#   1. pnpm install (nếu chưa có node_modules)
#   2. Build Dev Server Agent binaries (out/relay/) — cần cho tính năng
#      "Deploy Relay" của SSH Targets trong app desktop
#   3. electron-vite build (main + preload + renderer)
#   4. electron-builder package theo platform hiện tại (hoặc --mac/--win/--linux)
#
# Usage:
#   ./deploy/desktop/scripts/build.sh                # platform hiện tại
#   ./deploy/desktop/scripts/build.sh --mac
#   ./deploy/desktop/scripts/build.sh --win
#   ./deploy/desktop/scripts/build.sh --linux
#   ./deploy/desktop/scripts/build.sh --dir           # unpacked dir only (nhanh, để test)
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
DESKTOP_DIR="${REPO_ROOT}/desktop"

PLATFORM_FLAG=""
DIR_ONLY=false
for arg in "$@"; do
    case "$arg" in
        --mac|--win|--linux) PLATFORM_FLAG="$arg" ;;
        --dir) DIR_ONLY=true ;;
    esac
done

check_cmd() {
    command -v "$1" &>/dev/null || { echo "❌ ERROR: '$1' not found."; exit 1; }
}
check_cmd node
check_cmd pnpm

cd "${DESKTOP_DIR}"

echo "======================================================"
echo " Orca Desktop Build"
echo "======================================================"
echo "  Package: ${DESKTOP_DIR}"
echo "  Platform: ${PLATFORM_FLAG:-host}"
echo "======================================================"
echo ""

if [ ! -d "node_modules" ]; then
    echo "[1/4] Installing dependencies..."
    pnpm install
else
    echo "[1/4] Dependencies already installed. Skipping."
fi

echo ""
echo "[2/4] Building Dev Server Agent + Relay binaries (out/relay/)..."
pnpm run build:relay

echo ""
echo "[3/4] Building Electron main + preload + renderer (electron-vite)..."
pnpm run build

echo ""
if [ "${DIR_ONLY}" = "true" ]; then
    echo "[4/4] Packaging (--dir, unpacked only — fast, for local testing)..."
    npx electron-builder --config config/electron-builder.config.cjs ${PLATFORM_FLAG} --dir
else
    echo "[4/4] Packaging (${PLATFORM_FLAG:-host platform installer})..."
    npx electron-builder --config config/electron-builder.config.cjs ${PLATFORM_FLAG}
fi

echo ""
echo "======================================================"
echo " ✅ Build done."
echo "   dist/  — installers (.dmg/.exe/.AppImage/.deb) or dist/*-unpacked/"
echo "======================================================"
