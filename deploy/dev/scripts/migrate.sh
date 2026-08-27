#!/usr/bin/env bash
# ============================================================
# migrate.sh — run golang-migrate for one or all DB-owning services
# ============================================================
# Uses the `migrate-*` one-shot compose services (profile "migrate",
# docker-compose.yml) — the official migrate/migrate image, not a shell
# inside the distroless service containers (which have no shell).
#
# Usage:
#   ./deploy/dev/scripts/migrate.sh                 # all 14 services, local compose context
#   ./deploy/dev/scripts/migrate.sh usage            # one service (name after "migrate-")
#   ./deploy/dev/scripts/migrate.sh --remote         # run on the deploy server via SSH
#   ./deploy/dev/scripts/migrate.sh --remote usage   # one service, on the server
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

SERVICES="auth tenant project infra aiprovider workflow task orchestration automation annotation notification usage credential scm"

REMOTE=0
TARGET=""
for arg in "$@"; do
  case "$arg" in
    --remote) REMOTE=1 ;;
    *) TARGET="$arg" ;;
  esac
done

if [ -n "${TARGET}" ]; then
  SERVICES="${TARGET}"
fi

run_local() {
  cd "${DEPLOY_DIR}"
  for svc in ${SERVICES}; do
    echo "==> migrate-${svc}"
    docker compose run --rm "migrate-${svc}"
  done
}

run_remote() {
  if [ -f "${DEPLOY_DIR}/.env" ]; then
    export $(grep -v '^#' "${DEPLOY_DIR}/.env" | xargs)
  fi
  SERVER_HOST="${SERVER_HOST:?SERVER_HOST not set}"
  SERVER_USER="${SERVER_USER:-ubuntu}"
  SERVER_KEY="${SERVER_KEY:-${HOME}/.ssh/id_ed25519}"
  SERVER_PORT="${SERVER_PORT:-22}"
  SERVER_DEPLOY="${SERVER_DEPLOY:-~/orca-go-deploy}"
  SSH_OPTS="-i ${SERVER_KEY} -p ${SERVER_PORT} -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10"

  for svc in ${SERVICES}; do
    echo "==> migrate-${svc} (remote)"
    ssh ${SSH_OPTS} "${SERVER_USER}@${SERVER_HOST}" \
      "cd ${SERVER_DEPLOY} && docker compose run --rm migrate-${svc}"
  done
}

if [ "${REMOTE}" -eq 1 ]; then
  run_remote
else
  run_local
fi

echo "✅ Migrations applied for: ${SERVICES}"
