#!/usr/bin/env bash
# ============================================================
# run-orca-flows-ci.sh — build the local docker stack on a CI runner and run the orca-flows e2e against it
# ============================================================
# TASK-E2E-029. The stack is the same one a developer runs (deploy/dev + docker-compose.local.yml, a throwaway
# in-memory Vault), the dev server is the Dev Server Agent started on the runner itself, and AI answers come from the
# deterministic fake claude (`--ai-stub`): CI has no Claude account. A green run proves wiring and contracts, not AI quality.
#
#   bash tests/stages/e2e/orca-flows/ci/run-orca-flows-ci.sh nightly
#   bash tests/stages/e2e/orca-flows/ci/run-orca-flows-ci.sh weekly
#   bash tests/stages/e2e/orca-flows/ci/run-orca-flows-ci.sh custom -- --ai-stub request ai
#   bash tests/stages/e2e/orca-flows/ci/run-orca-flows-ci.sh --prepare-env-only [path]   # write the .env and stop (no docker)
#
# Modes: nightly = platform workflow request matrix exits ai plan; weekly = every suite with --strict
# (MCP stays BLOCKED because MCP needs a Vault policy and TLS front proxy the throwaway Vault does not provide).
#
# DESTRUCTIVE: the script ends with `docker compose down -v` on the whole orca-go project, so it refuses to run unless
# CI=true (GitHub sets it) or ORCA_FLOWS_CI_ALLOW_DESTRUCTIVE=1. POSIX (Linux runner); never prints a secret value.
# Exit codes: 0 green, 1 a test failed, 2 a precondition is missing (nothing was started), 3 the stack did not come up.
# ============================================================
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$here/../../../../.." && pwd)"
dev="$repo/deploy/dev"
flows="$repo/tests/stages/e2e/orca-flows"
out_dir="${ORCA_FLOWS_ARTIFACT_DIR:-$flows/reports}"
compose=(docker compose -f "$dev/docker-compose.yml" -f "$dev/docker-compose.local.yml" --project-directory "$dev")

# ── env file ────────────────────────────────────────────────

# Sets KEY=VALUE in the env file (replacing an existing line); python avoids sed -i differences between GNU and BSD.
set_key() { # file key value
  python3 - "$1" "$2" "$3" <<'PY'
import re, sys
path, key, val = sys.argv[1:4]
lines = open(path).read().splitlines() if __import__("os").path.exists(path) else []
pat = re.compile(r"^" + re.escape(key) + r"=")
out, done = [], False
for line in lines:
    if pat.match(line):
        if not done:
            out.append(f"{key}={val}")
        done = True
    else:
        out.append(line)
if not done:
    out.append(f"{key}={val}")
open(path, "w").write("\n".join(out) + "\n")
PY
}

secret() { openssl rand -hex 16; }

write_env() { # path
  local f="$1"
  [ -f "$f" ] || cp "$dev/.env.example" "$f"
  set_key "$f" POSTGRES_PASSWORD "$(secret)"
  set_key "$f" VAULT_TOKEN dev-root-token                     # the throwaway Vault's root token; it only lives in this job
  set_key "$f" VAULT_ADDR http://vault:8200                   # the overlay's Vault, not the shared vnp-domain one
  set_key "$f" ORCA_AGENT_API_SECRET "$(secret)"
  set_key "$f" BOOTSTRAP_COMPANY_NAME orca-ci
  set_key "$f" BOOTSTRAP_ADMIN_EMAIL ci-admin@example.test
  set_key "$f" BOOTSTRAP_ADMIN_PASSWORD "$(secret)Aa1"
  set_key "$f" FRONTEND_HTTP_PORT 8080
  set_key "$f" API_GATEWAY_PUBLIC_PORT 8081
  set_key "$f" REQUEST_FLOW_ENABLED true                      # the tenant switch (request.flowSet) is turned on by the suite itself
  set_key "$f" REQUEST_APPROVAL_ENABLED true
  set_key "$f" MCP_ENABLED false
  chmod 600 "$f"
  # The internal tokens request-service refuses to start without (and its readyz needs); fills only what is empty.
  python3 - "$f" <<'PY'
import os, subprocess, sys
path = sys.argv[1]
text = open(path).read()
for key in ("REQUEST_INTERNAL_CALLER_TOKEN", "REQUEST_SERVICE_INTERNAL_TOKEN", "PROJECT_INTERNAL_CALLER_TOKEN", "AUTH_DIRECTORY_CALLER_TOKEN"):
    val = subprocess.run(["openssl", "rand", "-hex", "32"], capture_output=True, text=True, check=True).stdout.strip()
    lines = [l for l in text.splitlines() if not l.startswith(key + "=")]
    lines.append(f"{key}={val}")
    text = "\n".join(lines) + "\n"
open(path, "w").write(text)
PY
}

if [ "${1:-}" = "--prepare-env-only" ]; then
  target="${2:-$dev/.env}"
  write_env "$target"
  echo "orca-flows-ci: wrote $target ($(grep -c '=' "$target") keys, values not printed)"
  exit 0
fi

mode="${1:-}"
[ -n "$mode" ] || { echo "usage: $0 nightly|weekly|custom [-- run_acceptance.py args]" >&2; exit 2; }
shift
[ "${1:-}" = "--" ] && shift

if [ "${CI:-}" != "true" ] && [ "${ORCA_FLOWS_CI_ALLOW_DESTRUCTIVE:-}" != "1" ]; then
  echo "orca-flows-ci: refusing to run outside CI: it ends with 'docker compose down -v' on the orca-go project." >&2
  echo "               set ORCA_FLOWS_CI_ALLOW_DESTRUCTIVE=1 only on a machine whose orca-go stack is disposable." >&2
  exit 2
fi
for tool in docker go node pnpm python3 openssl curl rsync; do
  command -v "$tool" >/dev/null 2>&1 || { echo "orca-flows-ci: '$tool' is required" >&2; exit 2; }
done
docker compose version >/dev/null 2>&1 || { echo "orca-flows-ci: 'docker compose' (v2) is required" >&2; exit 2; }
[ -f "$dev/.env" ] && { echo "orca-flows-ci: $dev/.env already exists; a CI run must start from a clean checkout" >&2; exit 2; }

case "$mode" in
  nightly) args=(--ai-stub --keep-stub-agent platform workflow request matrix exits ai plan) ;;
  weekly) args=(--ai-stub --keep-stub-agent --strict --allow-blocked MCP-01,MCP-02) ;;
  custom) args=("$@"); args+=(--keep-stub-agent) ;;
  *) echo "orca-flows-ci: unknown mode '$mode'" >&2; exit 2 ;;
esac

mkdir -p "$out_dir"

collect_diagnostics() {
  local status=$?
  if [ "$status" -ne 0 ]; then
    echo "==> collecting diagnostics (exit $status)"
    "${compose[@]}" logs --no-color --tail 400 > "$out_dir/stack-logs.txt" 2>&1 || true
    for home in "${TMPDIR:-/tmp}"/orca-fake-claude-home-*; do
      [ -d "$home" ] || continue
      cp "$home/agent.log" "$out_dir/agent-$(basename "$home").log" 2>/dev/null || true
      cp "$home/.fake-claude/calls.jsonl" "$out_dir/fake-claude-calls-$(basename "$home").jsonl" 2>/dev/null || true
    done
  fi
  pkill -f "agent/out/agent.js" 2>/dev/null || true
  "${compose[@]}" down -v --timeout 20 >/dev/null 2>&1 || true
  exit "$status"
}
trap collect_diagnostics EXIT

echo "==> writing deploy/dev/.env for the throwaway stack"
write_env "$dev/.env"

echo "==> building the Go services and the frontend (deploy/dev/scripts/build-local.sh)"
bash "$dev/scripts/build-local.sh"

echo "==> starting postgres, nats and the throwaway vault"
"${compose[@]}" up -d postgres nats vault
"${compose[@]}" up -d --wait postgres nats vault
"${compose[@]}" up -d vault-init
docker wait orca-go-vault-init >/dev/null

echo "==> migrating every database"
bash "$dev/scripts/migrate.sh" >/dev/null

echo "==> starting the services"
"${compose[@]}" up -d

gateway="http://localhost:8081"
admin_email="ci-admin@example.test"
echo "==> waiting for the gateway and the bootstrap admin"
ready=0
for _ in $(seq 1 90); do
  if curl -fsS -m 5 "$gateway/healthz" >/dev/null 2>&1; then ready=1; break; fi
  sleep 2
done
[ "$ready" -eq 1 ] || { echo "orca-flows-ci: the gateway never became healthy at $gateway" >&2; exit 3; }
admin_password="$(sed -n 's/^BOOTSTRAP_ADMIN_PASSWORD=//p' "$dev/.env" | tail -n 1)"
logged_in=0
for _ in $(seq 1 60); do
  code="$(curl -s -m 8 -o /dev/null -w '%{http_code}' -X POST "$gateway/auth/local" -H 'Content-Type: application/json' \
    -d "{\"email\":\"$admin_email\",\"password\":\"$admin_password\"}" || true)"
  if [ "$code" = "200" ]; then logged_in=1; break; fi
  sleep 2
done
[ "$logged_in" -eq 1 ] || { echo "orca-flows-ci: the bootstrap admin could not log in" >&2; exit 3; }

# A direct-websocket agent registers under ORCA_AGENT_DEFAULT_TENANT_ID; the bootstrap company's id is generated, so
# point infra-fleet at it, then restart the Go services because a recreated container gets a new address on the bridge.
echo "==> pointing infra-fleet-service at the bootstrap tenant"
tenant_id="$("${compose[@]}" exec -T postgres psql -U orca -d tenant -At -c "select id from tenant.companies order by created_at limit 1" | tr -d '[:space:]')"
case "$tenant_id" in
  [0-9a-f]*-*-*-*-*) ;;
  *) echo "orca-flows-ci: could not read the bootstrap tenant id" >&2; exit 3 ;;
esac
set_key "$dev/.env" ORCA_AGENT_DEFAULT_TENANT_ID "$tenant_id"
"${compose[@]}" up -d infra-fleet-service
restartable=()
while IFS= read -r svc; do restartable+=("$svc"); done < <("${compose[@]}" config --services | grep -Ev '^(postgres|nats|vault|vault-init|vault-token-renewer|migrate-.*)$')
"${compose[@]}" restart "${restartable[@]}" >/dev/null
sleep 10

echo "==> installing dependencies and building the agent"
(cd "$repo" && pnpm install --no-frozen-lockfile --prefer-frozen-lockfile=false >/dev/null)
(cd "$repo" && node agent/build.mjs >/dev/null)

echo "==> running orca-flows ($mode) with the fake claude"
set +e
(cd "$flows" && python3 run_acceptance.py "${args[@]}")
status=$?
set -e
for f in last-run.json last-run.md; do
  [ "$out_dir" = "$flows/reports" ] || cp "$flows/reports/$f" "$out_dir/$f" 2>/dev/null || true
done
exit "$status"
