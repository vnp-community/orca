#!/usr/bin/env bash
# T2 end-to-end of the Request flow on the dev stack with the stub agent (CR-REQ-025).
#
#   bash backend-go/ci/request-e2e/run-request-e2e.sh
#
# Needs Docker, Go, python3 with tests/backend/requirements.txt, and the variables the dev stack itself needs
# (VAULT_TOKEN, POSTGRES_PASSWORD, bootstrap admin) plus ORCA_REQUEST_PROJECT_ID, a project the stub agent serves.
# Exit codes: 0 all checks passed, 1 a check failed, 2 a precondition is missing (nothing was started).
# Not verified on a CI runner or against the shared Vault yet: see IMPLEMENTATION-NOTES.md of request-quality-rollout.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../../.." && pwd)"
dev="$repo/deploy/dev"
compose=(docker compose -f "$dev/docker-compose.yml" -f "$here/docker-compose.e2e.yml" --project-directory "$dev")

# Services the Request flow touches; infra-fleet-service stays in the list because other services still dial it.
services=(auth-service tenant-service project-service infra-fleet-service issue-tracking-service
  issue-status-sync task-service request-service api-gateway)

need() {
  if ! command -v "$1" > /dev/null 2>&1; then
    echo "request-e2e: '$1' is required" >&2
    exit 2
  fi
}
need docker
need go
need python3

for var in VAULT_TOKEN POSTGRES_PASSWORD ORCA_REQUEST_PROJECT_ID; do
  if [ -z "${!var:-}" ]; then
    echo "request-e2e: $var must be set" >&2
    exit 2
  fi
done

echo "==> building binaries"
for svc in "${services[@]}"; do
  out="$dev/bin/$svc/orca"
  mkdir -p "$(dirname "$out")"
  (cd "$repo/backend-go/services/$svc" && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o "$out" ./cmd/server)
  if [ -d "$repo/backend-go/services/$svc/migrations/postgres" ]; then
    rm -rf "$dev/bin/$svc/migrations"
    cp -r "$repo/backend-go/services/$svc/migrations/postgres" "$dev/bin/$svc/migrations"
  fi
done
mkdir -p "$dev/bin/agent-stub"
(cd "$repo/backend-go/services/request-service" && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o "$dev/bin/agent-stub/orca" ./e2e/cmd/agent-stub)

# Only the project name's containers are touched: never a prune, other stacks live on this machine.
cleanup() {
  status=$?
  if [ "$status" -ne 0 ]; then
    "${compose[@]}" logs --no-color --tail 200 request-service agent-stub > "${REQUEST_E2E_LOG_DIR:-$here}/request-e2e-logs.txt" 2>&1 || true
  fi
  "${compose[@]}" down --timeout 20 > /dev/null 2>&1 || true
  exit "$status"
}
trap cleanup EXIT

echo "==> starting the stack"
"${compose[@]}" up -d postgres nats
"${compose[@]}" up -d --wait postgres nats
bash "$dev/scripts/migrate.sh" > /dev/null
"${compose[@]}" up -d "${services[@]}" agent-stub

echo "==> waiting for the gateway"
base="${ORCA_API_BASE_URL:-http://localhost:6768}"
for _ in $(seq 1 60); do
  if curl -fsS "$base/healthz" > /dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 2
done
if [ "${ready:-0}" -ne 1 ]; then
  echo "request-e2e: the gateway never became healthy at $base" >&2
  exit 1
fi

echo "==> running checks"
export ORCA_API_BASE_URL="$base"
(cd "$repo/tests/request" && python3 check_request_flow_types.py)
