#!/usr/bin/env bash
# ============================================================
# split-up.sh — Orca control plane and dev servers as separate "hosts" in Docker (TDD-WF-AGENT-001 §11.1)
# ============================================================
# Two compose projects joined by one network, the stand-in for the WAN:
#   orca-e2e             deploy/dev stack (+ local Vault). Only api-gateway is on orca-e2e-wan (alias orca-gateway).
#   orca-e2e-devservers  devserver-a (and devserver-b with --multi): agent.js, git, fake or real claude, optional tdd-agent.
# A dev server reaches Orca only through ws://orca-gateway:8081/agent; Orca reaches a dev server only through that
# agent connection. Tokens are issued here, on the operator side, so the server secret never enters a dev server.
#
# Usage (from anywhere):
#   bash tests/stages/e2e/split-topology/split-up.sh                  # binaries already in deploy/dev/bin
#   bash tests/stages/e2e/split-topology/split-up.sh --build          # build Go services, frontend and agent.js first
#   bash tests/stages/e2e/split-topology/split-up.sh --multi          # also devserver-b
#   bash tests/stages/e2e/split-topology/split-up.sh --devservers     # only (re)start the dev servers with fresh tokens
#   TDD_AGENT_SRC=/opt/repos/tdd-agentic-ai bash .../split-up.sh      # bake tdd-agent + tsagent into the dev server image
#   AI_MODE=real INSTALL_CLAUDE_CLI=1 CLAUDE_HOME_A=$HOME/.claude bash .../split-up.sh   # real Claude Code on devserver-a
# Env: SPLIT_BIND_IP (default 127.0.0.1) for the published ports; DEVSERVER_A_ID / DEVSERVER_B_ID.
# Never edits deploy/dev/.env; never touches the compose project orca-go (the regular local stack) or its volumes.
# Down: bash tests/stages/e2e/split-topology/split-down.sh [--volumes]
# ============================================================
set -euo pipefail

SPLIT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SPLIT_DIR}/../../../.." && pwd)"
DEPLOY_DIR="${REPO_ROOT}/deploy/dev"

BUILD=0
MULTI=0
DEVSERVERS_ONLY=0
for arg in "$@"; do
  case "$arg" in
    --build) BUILD=1 ;;
    --multi) MULTI=1 ;;
    --devservers) DEVSERVERS_ONLY=1 ;;
    -h|--help) sed -n 2,22p "${BASH_SOURCE[0]}"; exit 0 ;;
    *) echo "unknown argument: $arg" >&2; exit 2 ;;
  esac
done

[ -f "${DEPLOY_DIR}/.env" ] || { echo "deploy/dev/.env is missing: see deploy/dev/README.md (Local-only)" >&2; exit 1; }
set -a
# shellcheck disable=SC1091
. "${DEPLOY_DIR}/.env" >/dev/null 2>&1
set +a
: "${ORCA_AGENT_API_SECRET:?ORCA_AGENT_API_SECRET missing in deploy/dev/.env}"

# Isolated test stack: local dev-mode Vault, loopback ports. Shell env wins over deploy/dev/.env, which stays untouched.
export VAULT_ADDR="http://vault:8200"
# The shared Vault token of deploy/dev/.env is not a valid dev-mode root token id, and this stack must not use it anyway.
export VAULT_TOKEN="${SPLIT_VAULT_TOKEN:-dev-root-token}"
export SERVER_BIND_IP="${SPLIT_BIND_IP:-127.0.0.1}"
GATEWAY_PORT="${API_GATEWAY_PUBLIC_PORT:-8081}"
GATEWAY_HOST="${SERVER_BIND_IP}"
[ "${GATEWAY_HOST}" = "0.0.0.0" ] && GATEWAY_HOST=127.0.0.1
GATEWAY_URL="http://${GATEWAY_HOST}:${GATEWAY_PORT}"

CP=(docker compose -p orca-e2e --project-directory "${DEPLOY_DIR}"
    -f "${DEPLOY_DIR}/docker-compose.yml" -f "${DEPLOY_DIR}/docker-compose.local.yml" -f "${SPLIT_DIR}/control-plane.override.yml")
DS=(docker compose -f "${SPLIT_DIR}/docker-compose.devserver.yml")
[ "${MULTI}" -eq 1 ] && DS+=(--profile multi)

# deploy/dev pins container_name (orca-go-*): a second stack under another project would collide with ours.
foreign="$(docker ps -a --filter name='^orca-go-' --format '{{.Names}} {{.Label "com.docker.compose.project"}}' | awk '$2!="orca-e2e"{print $1}')"
if [ -n "${foreign}" ]; then
  echo "containers of another Orca stack exist (${foreign//$'\n'/, }); stop it first, this stack reuses the same container names" >&2
  exit 1
fi

if [ "${BUILD}" -eq 1 ]; then
  arch="$(docker info --format '{{.Architecture}}')"
  case "${arch}" in aarch64|arm64) export GOARCH=arm64 ;; *) export GOARCH=amd64 ;; esac
  echo "==> build backend-go services + frontend (GOARCH=${GOARCH})"
  bash "${DEPLOY_DIR}/scripts/build-local.sh"
  echo "==> build agent.js"
  (cd "${REPO_ROOT}/agent" && node build.mjs)
fi
[ -x "${DEPLOY_DIR}/bin/api-gateway/orca" ] || { echo "deploy/dev/bin is empty: rerun with --build" >&2; exit 1; }
[ -f "${REPO_ROOT}/agent/out/agent.js" ] || { echo "agent/out/agent.js is missing: rerun with --build" >&2; exit 1; }

if [ "${DEVSERVERS_ONLY}" -eq 0 ]; then
  echo "==> control plane: infra"
  "${CP[@]}" up -d postgres nats vault
  "${CP[@]}" up -d vault-init

  echo "==> control plane: migrations (same steps as deploy/dev/scripts/migrate.sh, on project orca-e2e)"
  SERVICES="auth tenant project infra aiprovider workflow task orchestration automation annotation notification usage credential issuetracking scm mcp issuestatussync request"
  ENSURE_DB_SQL='SELECT format($f$CREATE DATABASE %I OWNER orca$f$, d) FROM unnest(string_to_array(:'"'"'dbs'"'"', '"'"' '"'"')) AS d WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = d)\gexec'
  for _ in $(seq 1 30); do "${CP[@]}" exec -T postgres pg_isready -U orca >/dev/null 2>&1 && break; sleep 2; done
  printf '%s\n' "${ENSURE_DB_SQL}" | "${CP[@]}" exec -T postgres psql -q -v ON_ERROR_STOP=1 -U orca -d postgres -v dbs="${SERVICES}"
  for svc in ${SERVICES}; do
    if ! "${CP[@]}" --profile migrate run --rm "migrate-${svc}" >/dev/null 2>&1; then
      echo "migrate-${svc} failed:" >&2
      "${CP[@]}" --profile migrate run --rm "migrate-${svc}" >&2 || true
      exit 1
    fi
  done

  echo "==> control plane: services"
  "${CP[@]}" up -d
fi

# A direct-websocket agent registers under ORCA_AGENT_DEFAULT_TENANT_ID; the bootstrap company's id is generated, so point
# infra-fleet at it (as tests/stages/e2e/orca-flows/ci does), via the shell env rather than deploy/dev/.env.
if [ -z "${ORCA_AGENT_DEFAULT_TENANT_ID:-}" ]; then
  tenant_id=""
  for _ in $(seq 1 60); do
    tenant_id="$("${CP[@]}" exec -T postgres psql -U orca -d tenant -At -c "select id from tenant.companies order by created_at limit 1" 2>/dev/null | tr -d '[:space:]')"
    case "${tenant_id}" in [0-9a-f]*-*-*-*-*) break ;; *) tenant_id=""; sleep 2 ;; esac
  done
  [ -n "${tenant_id}" ] || { echo "could not read the bootstrap tenant id" >&2; exit 1; }
  export ORCA_AGENT_DEFAULT_TENANT_ID="${tenant_id}"
  current="$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' orca-go-infra-fleet | sed -n 's/^ORCA_AGENT_DEFAULT_TENANT_ID=//p')"
  if [ "${current}" != "${tenant_id}" ]; then
    echo "==> pointing infra-fleet-service at the bootstrap tenant"
    "${CP[@]}" up -d infra-fleet-service >/dev/null
    # a recreated container gets a new bridge address; the other Go services must reconnect
    mapfile -t restartable < <("${CP[@]}" config --services | grep -Ev '^(postgres|nats|vault|vault-init|vault-token-renewer|frontend|migrate-.*)$')
    "${CP[@]}" restart "${restartable[@]}" >/dev/null
  fi
fi

for _ in $(seq 1 90); do
  curl -fsS -m 5 "${GATEWAY_URL}/healthz" >/dev/null 2>&1 && break
  sleep 2
done
echo "    gateway answering at ${GATEWAY_URL}"

# tdd-agent wheel for the dev server image (the image copies whatever wheels/ holds).
WHEELS="${SPLIT_DIR}/devserver/wheels"
find "${WHEELS}" -name '*.whl' -delete
if [ -n "${TDD_AGENT_SRC:-}" ]; then
  echo "==> build tdd-agent wheel from ${TDD_AGENT_SRC}"
  # fixed zip timestamps: an unchanged tdd-agent gives a byte-identical wheel, so the image layer stays cached
  SOURCE_DATE_EPOCH=315532800 python3 -m pip wheel --quiet --no-deps --no-build-isolation --wheel-dir "${WHEELS}" "${TDD_AGENT_SRC}" 2>/dev/null \
    || SOURCE_DATE_EPOCH=315532800 python3 -m pip wheel --quiet --no-deps --wheel-dir "${WHEELS}" "${TDD_AGENT_SRC}"
fi

issue_token() { # $1 = dev server id; the token goes to the container's env, never to the terminal
  curl -sf -m 10 -X POST "${GATEWAY_URL}/api/agent-token" -H 'Content-Type: application/json' \
    -H "Authorization: Bearer ${ORCA_AGENT_API_SECRET}" \
    -d "{\"devServerId\":\"$1\",\"name\":\"$1\",\"ttl\":3600}" \
    | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])'
}

export DEVSERVER_A_ID="${DEVSERVER_A_ID:-e2e-devserver-a}"
# Build first: infra-fleet drops a token nobody connected with within 60 s (agentwsserver DefaultConnectTimeout), and an
# image build can take minutes.
echo "==> dev server image"
DEVSERVER_A_TOKEN=unused "${DS[@]}" build
export DEVSERVER_B_ID="${DEVSERVER_B_ID:-e2e-devserver-b}"
echo "==> agent tokens (operator side)"
DEVSERVER_A_TOKEN="$(issue_token "${DEVSERVER_A_ID}")"; export DEVSERVER_A_TOKEN
if [ "${MULTI}" -eq 1 ]; then DEVSERVER_B_TOKEN="$(issue_token "${DEVSERVER_B_ID}")"; export DEVSERVER_B_TOKEN; fi

echo "==> dev servers"
"${DS[@]}" up -d --no-build --force-recreate

echo "==> wait until connected, approve as admin"
names=("${DEVSERVER_A_ID}")
[ "${MULTI}" -eq 1 ] && names+=("${DEVSERVER_B_ID}")
ORCA_E2E_GATEWAY_URL="${GATEWAY_URL}" python3 "${SPLIT_DIR}/devserver_ready.py" "${names[@]}"

cat <<EOF

Split topology is up.
  Orca (control plane)  ${GATEWAY_URL}   frontend: http://${GATEWAY_HOST}:${FRONTEND_HTTP_PORT:-8080}
  Dev servers           ${names[*]}  (containers orca-e2e-devserver-*, repos under /srv/projects)

Verify the topology:
  ORCA_E2E_GATEWAY_URL=${GATEWAY_URL} python3 tests/stages/e2e/split-topology/verify_topology.py $([ "${MULTI}" -eq 1 ] && echo --multi)
Workflow + tdd-agent suite (needs TDD_AGENT_SRC at build time):
  ORCA_E2E_GATEWAY_URL=${GATEWAY_URL} python3 tests/stages/e2e/split-topology/suite_workflow_tdd_agent.py
EOF
