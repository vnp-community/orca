#!/usr/bin/env bash
# ============================================================
# build.sh — Build Orca Mobile (Expo/React Native) từ mobile/
# ============================================================
# mobile/ đã là package độc lập từ trước (pnpm-lock.yaml, tsconfig,
# metro.config.js riêng) — script này chỉ chuẩn hoá lệnh build local.
# Release thật (sign + upload TestFlight) đi qua fastlane, xem
# deploy/mobile/README.md.
#
# Usage:
#   ./deploy/mobile/scripts/build.sh --ios       # expo prebuild + run:ios (local/simulator)
#   ./deploy/mobile/scripts/build.sh --android    # expo prebuild + run:android (local/emulator)
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
MOBILE_DIR="${REPO_ROOT}/mobile"

TARGET=""
for arg in "$@"; do
    case "$arg" in
        --ios)     TARGET=ios ;;
        --android) TARGET=android ;;
    esac
done

[ -z "${TARGET}" ] && { echo "Usage: $0 --ios|--android"; exit 1; }

check_cmd() {
    command -v "$1" &>/dev/null || { echo "❌ ERROR: '$1' not found."; exit 1; }
}
check_cmd node
check_cmd pnpm

cd "${MOBILE_DIR}"

echo "======================================================"
echo " Orca Mobile Build — ${TARGET}"
echo "======================================================"
echo "  Package: ${MOBILE_DIR}"
echo "======================================================"
echo ""

if [ ! -d "node_modules" ]; then
    echo "[1/2] Installing dependencies (pnpm install trong mobile/, lockfile riêng)..."
    pnpm install
else
    echo "[1/2] Dependencies already installed. Skipping."
fi

echo ""
echo "[2/2] Running expo run:${TARGET} (prebuild native project nếu chưa có)..."
pnpm run "${TARGET}"

echo ""
echo "======================================================"
echo " ✅ ${TARGET} build launched."
echo "   Release (TestFlight/Play Store) → xem deploy/mobile/README.md"
echo "======================================================"
