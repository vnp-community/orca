#!/usr/bin/env bash
# ============================================================
# build-local.sh — Build backend-go binaries + frontend static bundle LOCALLY
# ============================================================
# Step 1 of the deploy flow this directory implements:
#   (1) build binary locally  →  (2) sync to server  →  (3) mount into container
#
# Why build locally instead of server-side (unlike deploy/old's
# docker-compose.yml, which built backend/+frontend/ from source INSIDE the
# container): Go's whole appeal here is a static, cross-compiled binary —
# there's no npm-install-on-server cost to avoid or reproduce, and
# cross-compiling for linux/amd64 works identically from any dev machine
# (CGO_ENABLED=0), so there is no reason to ship source and pay a
# server-side compile cost at all. The server never sees backend-go/ source,
# only the compiled binaries this script produces.
#
# Output layout (gitignored, see ../.gitignore):
#   deploy/dev/bin/<service-name>/orca   — one static binary per service
#   deploy/dev/dist/                     — frontend Vite build output
#
# Usage:
#   ./deploy/dev/scripts/build-local.sh              # build everything
#   ./deploy/dev/scripts/build-local.sh usage-service # build one service only
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${DEPLOY_DIR}/../.." && pwd)"
BACKEND_GO_DIR="${REPO_ROOT}/backend-go"
BIN_DIR="${DEPLOY_DIR}/bin"
DIST_DIR="${DEPLOY_DIR}/dist"

# Every backend-go service, in the order services/00-service-catalog.md lists
# them. Kept as one array so this script is the single source of truth for
# "what services exist" that docker-compose.yml's service list must match.
ALL_SERVICES=(
  ai-provider-service annotation-service api-gateway auth-service
  automation-service credential-broker-service git-gateway-service
  infra-fleet-service issue-tracking-service notification-service
  orchestration-service project-service scm-integration-service
  task-service tenant-service usage-service workflow-service
)

check_cmd() {
  command -v "$1" &>/dev/null || { echo "❌ ERROR: '$1' not found. Please install it first."; exit 1; }
}
check_cmd go
check_cmd pnpm
check_cmd docker

TARGET_SERVICES=("${ALL_SERVICES[@]}")
if [ "$#" -eq 1 ]; then
  TARGET_SERVICES=("$1")
fi

echo "======================================================"
echo " Orca backend-go + frontend — Local Build"
echo "======================================================"
echo "  Repo:     ${REPO_ROOT}"
echo "  Services: ${#TARGET_SERVICES[@]} (${TARGET_SERVICES[*]})"
echo "  Output:   ${BIN_DIR}/<service>/orca  +  ${DIST_DIR}/"
echo "======================================================"
echo ""

mkdir -p "${BIN_DIR}" "${DIST_DIR}"

echo "[1/3] Building backend-go binaries (CGO_ENABLED=0, linux/amd64)..."
FAIL=0
for svc in "${TARGET_SERVICES[@]}"; do
  svc_dir="${BACKEND_GO_DIR}/services/${svc}"
  if [ ! -d "${svc_dir}" ]; then
    echo "  ⚠️  skip ${svc} — directory not found at ${svc_dir}"
    continue
  fi
  out="${BIN_DIR}/${svc}/orca"
  mkdir -p "$(dirname "${out}")"
  echo "  → ${svc}"
  # -trimpath: reproducible builds, no local path leakage into the binary.
  # -ldflags "-s -w": strip debug symbols — smaller binary, matches the
  # "smallest possible" priority carried through to the runtime image too.
  if ! (cd "${svc_dir}" && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
      go build -trimpath -ldflags="-s -w -X main.version=${ORCA_GO_VERSION:-dev}" \
      -o "${out}" ./cmd/server); then
    echo "  ❌ FAILED: ${svc}"
    FAIL=1
    continue
  fi
  # Migrations travel alongside the binary — sync-to-server.sh rsyncs this
  # directory too, and the migrate-<service> one-shot compose service
  # (docker-compose.yml) mounts it read-only.
  if [ -d "${svc_dir}/migrations" ]; then
    rm -rf "${BIN_DIR}/${svc}/migrations"
    cp -r "${svc_dir}/migrations" "${BIN_DIR}/${svc}/migrations"
  fi
  echo "    $(du -h "${out}" | cut -f1)  ${out}"
done

if [ "${FAIL}" -ne 0 ]; then
  echo ""
  echo "❌ One or more services failed to build — aborting before frontend build."
  exit 1
fi

echo ""
echo "[2/3] Building frontend/ (vite build)..."
(cd "${REPO_ROOT}/frontend" && [ -d node_modules ] || pnpm install)
(cd "${REPO_ROOT}/frontend" && pnpm run build)
rm -rf "${DIST_DIR}"
cp -r "${REPO_ROOT}/frontend/out/web" "${DIST_DIR}"

# Why: docker-compose.yml's OPA policy bind mounts must resolve to a real
# path on the SERVER after sync-to-server.sh — that script only ships this
# directory's own tree (bin/dist/config), never the full backend-go/ source
# checkout, so a mount source written as "../../backend-go/policy/orca-authz"
# (relative to the compose file's location once copied to
# ~/orca-go-deploy/) resolves to a nonexistent path there. Docker silently
# bind-mounts an empty directory for a missing host path instead of erroring,
# so every OPA-gated call (auth-service's requireAdminActor, and
# task-service/annotation-service/project-service's own admin checks) failed
# closed for every caller regardless of role — live-verified 2026-08-29
# (admin.listUsers returned AUTH_NOT_ADMIN for the actual bootstrap admin).
# Copying the policy directory into deploy/dev/policy/ here means it's
# already covered by sync-to-server.sh's existing "rsync everything in
# deploy/dev except bin/dist/.env" step — no separate sync step needed, and
# docker-compose.yml's mount source becomes "./policy/orca-authz", correctly
# relative to wherever the compose file itself ends up.
POLICY_DIR="${DEPLOY_DIR}/policy"
rm -rf "${POLICY_DIR}"
mkdir -p "${POLICY_DIR}"
cp -r "${BACKEND_GO_DIR}/policy/orca-authz" "${POLICY_DIR}/orca-authz"

echo ""
echo "[3/3] Building git-gateway-service's own runtime image (needs a real"
echo "  git binary — see docker-compose.yml's git-gateway-service comment)..."
docker build \
  --platform linux/amd64 \
  -t "orca-git-gateway-runtime:${ORCA_GO_VERSION:-dev}" \
  -f "${DEPLOY_DIR}/docker/git-gateway-runtime.Dockerfile" \
  "${DEPLOY_DIR}/docker"
echo "✅ Built orca-git-gateway-runtime:${ORCA_GO_VERSION:-dev}"

echo ""
echo "======================================================"
echo " ✅ Build OK."
echo "   backend-go binaries: $(du -sh "${BIN_DIR}" 2>/dev/null | cut -f1)  (${BIN_DIR}/)"
echo "   frontend static:     $(du -sh "${DIST_DIR}" 2>/dev/null | cut -f1)  (${DIST_DIR}/)"
echo "   OPA policy bundle:   $(du -sh "${POLICY_DIR}" 2>/dev/null | cut -f1)  (${POLICY_DIR}/)"
echo ""
echo "  Next step:"
echo "    ./deploy/dev/scripts/sync-to-server.sh <version>"
echo "======================================================"
