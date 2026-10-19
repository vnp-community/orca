#!/usr/bin/env bash
# ============================================================
# redeploy-local.sh — rebuild and restart named services on the local docker stack, then smoke-test it
# ============================================================
# Why this exists: binaries are bind-mounted into stock images, so a rebuilt binary is only picked up when the
# container restarts; forgetting that, or recreating Vault (which is in-memory in the local overlay), cost real time.
#
# Usage:
#   ./deploy/dev/scripts/redeploy-local.sh request-service project-service     # rebuild + restart these
#   BUILD_FRONTEND=1 ./deploy/dev/scripts/redeploy-local.sh frontend api-gateway
#   SKIP_SMOKE=1 ./deploy/dev/scripts/redeploy-local.sh task-service
#   SKIP_MIGRATE=1 skips applying the service's new migrations (they are applied by default)
# Service names are the docker compose service names (the Go services are named <name>-service).
# Vault and its init job are never touched.
# ============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${DEPLOY_DIR}/../.." && pwd)"

[ "$#" -gt 0 ] || { echo "usage: $0 <compose-service>..." >&2; exit 2; }

# The containers run natively on this host's architecture; a wrong GOARCH only shows up as "exec format error".
case "$(docker info --format '{{.Architecture}}' 2>/dev/null || uname -m)" in
  aarch64|arm64) export GOARCH=arm64 ;;
  *) export GOARCH=amd64 ;;
esac

COMPOSE=(docker compose -f "${DEPLOY_DIR}/docker-compose.yml")
[ -f "${DEPLOY_DIR}/docker-compose.local.yml" ] && COMPOSE+=(-f "${DEPLOY_DIR}/docker-compose.local.yml")

go_services=()
restart=()
for svc in "$@"; do
  case "$svc" in
    vault|vault-init|postgres|nats) echo "refusing to restart ${svc} (stateful)" >&2; exit 2 ;;
    frontend) restart+=("$svc") ;;
    *) go_services+=("$svc"); restart+=("$svc") ;;
  esac
done

if [ "${#go_services[@]}" -gt 0 ]; then
  for svc in "${go_services[@]}"; do
    bash "${SCRIPT_DIR}/build-local.sh" "$svc"
  done
fi
if printf '%s\n' "$@" | grep -qx frontend; then
  BUILD_FRONTEND=1 bash "${SCRIPT_DIR}/build-local.sh" "${go_services[0]:-api-gateway}"
fi

# A rebuilt service may ship new migrations; applying them is idempotent and must happen before it restarts.
# Compose service name -> database name used by the migrate-<db> one-shots.
db_for_service() {
  case "$1" in
    infra-fleet-service) echo infra ;;
    ai-provider-service) echo aiprovider ;;
    issue-tracking-service) echo issuetracking ;;
    scm-integration-service) echo scm ;;
    credential-broker-service) echo credential ;;
    issue-status-sync) echo issuestatussync ;;
    *-service) echo "${1%-service}" ;;
    *) echo "" ;;
  esac
}
if [ "${SKIP_MIGRATE:-0}" != "1" ] && [ "${#go_services[@]}" -gt 0 ]; then
  # Read the list into a variable first: grep -q closing the pipe early would fail the pipeline under pipefail.
  migrators="$(cd "${DEPLOY_DIR}" && "${COMPOSE[@]}" --profile migrate config --services)"
  for svc in "${go_services[@]}"; do
    db="$(db_for_service "$svc")"
    if [ -n "$db" ] && printf '%s\n' "${migrators}" | grep -qx "migrate-${db}"; then
      bash "${SCRIPT_DIR}/migrate.sh" "$db"
    fi
  done
fi

up_out="$(cd "${DEPLOY_DIR}" && "${COMPOSE[@]}" up -d "${restart[@]}" 2>&1 | tee /dev/stderr)"

# A recreated container gets a new IP on the bridge network. Other services keep the address they resolved earlier,
# and that address may now belong to a different container, so they connect to the wrong service ("unknown service ...").
# Restarting every stateless service makes them resolve again. Stateful containers are never touched.
if printf '%s' "${up_out}" | grep -q "Recreate"; then
  echo "==> containers were recreated: restarting all Go services so none keeps a stale address"
  all_services=()
  while IFS= read -r name; do
    case "$name" in
      postgres|nats|vault|vault-init|vault-token-renewer|migrate-*) ;;
      *) all_services+=("$name") ;;
    esac
  done < <(cd "${DEPLOY_DIR}" && "${COMPOSE[@]}" config --services)
  ( cd "${DEPLOY_DIR}" && "${COMPOSE[@]}" restart "${all_services[@]}" )
else
  ( cd "${DEPLOY_DIR}" && "${COMPOSE[@]}" restart "${restart[@]}" )
fi

if [ "${SKIP_SMOKE:-0}" != "1" ] && command -v python3 >/dev/null 2>&1; then
  echo "==> smoke test (orca-flows platform suite)"
  sleep 8
  ( cd "${REPO_ROOT}/tests/stages/e2e/orca-flows" && python3 run_acceptance.py platform --no-report )
fi
