#!/usr/bin/env bash
# Stops both "hosts" of the split topology. --volumes also drops their data (Postgres of orca-e2e, dev server repos).
# Never touches the regular deploy/dev stack (compose project orca-go) or its volumes.
set -euo pipefail

SPLIT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SPLIT_DIR}/../../../.." && pwd)"
DEPLOY_DIR="${REPO_ROOT}/deploy/dev"

flags=(--remove-orphans)
[ "${1:-}" = "--volumes" ] && flags+=(--volumes)

set -a
# shellcheck disable=SC1091
[ -f "${DEPLOY_DIR}/.env" ] && . "${DEPLOY_DIR}/.env" >/dev/null 2>&1
set +a
export VAULT_ADDR="http://vault:8200" VAULT_TOKEN="${SPLIT_VAULT_TOKEN:-dev-root-token}" DEVSERVER_A_TOKEN=unused
docker compose -f "${SPLIT_DIR}/docker-compose.devserver.yml" --profile multi down "${flags[@]}"
docker compose -p orca-e2e --project-directory "${DEPLOY_DIR}" \
  -f "${DEPLOY_DIR}/docker-compose.yml" -f "${DEPLOY_DIR}/docker-compose.local.yml" -f "${SPLIT_DIR}/control-plane.override.yml" \
  --profile migrate down "${flags[@]}"
