#!/usr/bin/env bash
# Fails when request-service is missing from any place a new service must be registered.
# migrate.sh once forgot issuetracking (BUG-014) and that DB was never migrated anywhere, so this
# check exists for request-service; it is deliberately not generalised (see TASK-REQ-025-06).
#
# WIRING_ROOT overrides the repository root (the self-test points it at a mutated copy).
set -euo pipefail

root="${WIRING_ROOT:-$(cd "$(dirname "$0")/../.." && pwd)}"
fail=0

ok() { echo "OK: $1"; }
bad() { echo "FAIL: $1" >&2; fail=1; }

# word NAME FILE: NAME as a whole word in FILE (not a substring of another service name).
has_word() { grep -Eq "(^|[^[:alnum:]_-])$1([^[:alnum:]_-]|$)" "$2"; }

# block FILE NAME: the compose service block starting at "  NAME:" up to the next two-space-indented key.
block() {
  awk -v name="$2" '
    $0 ~ "^  " name ":[[:space:]]*$" { on = 1; next }
    on && $0 ~ "^  [A-Za-z0-9_-]+:" { on = 0 }
    on { print }
  ' "$1"
}

check_file() {
  if [ ! -f "$root/$1" ]; then
    bad "$1 does not exist"
    return 1
  fi
}

# (a) go.work
if check_file backend-go/go.work; then
  if grep -Eq '^[[:space:]]*\./services/request-service[[:space:]]*$' "$root/backend-go/go.work"; then
    ok "go.work uses ./services/request-service"
  else
    bad "backend-go/go.work does not list ./services/request-service"
  fi
fi

# (b) Makefile SERVICES
if check_file backend-go/Makefile; then
  services=$(awk '/^SERVICES[[:space:]]*:?=/ { on = 1 } on { print } on && !/\\[[:space:]]*$/ { exit }' "$root/backend-go/Makefile")
  if printf '%s\n' "$services" | grep -Eq '(^|[[:space:]])request-service([[:space:]]|\\|$)'; then
    ok "Makefile SERVICES has request-service"
  else
    bad "backend-go/Makefile SERVICES does not list request-service"
  fi
fi

# (c) postgres-init-databases.sh DATABASES
if check_file backend-go/deploy/postgres-init-databases.sh; then
  dbs=$(grep -E '^DATABASES=' "$root/backend-go/deploy/postgres-init-databases.sh" || true)
  if printf '%s\n' "$dbs" | grep -Eq '[" ]request[" ]'; then
    ok "postgres-init-databases.sh DATABASES has request"
  else
    bad "backend-go/deploy/postgres-init-databases.sh DATABASES does not list request"
  fi
fi

# (d) compose services, (e) REQUEST_SERVICE_ADDR for the two clients
compose=deploy/dev/docker-compose.yml
if check_file "$compose"; then
  for svc in request-service migrate-request; do
    if [ -n "$(block "$root/$compose" "$svc")" ]; then
      ok "$compose defines $svc"
    else
      bad "$compose does not define the service $svc"
    fi
  done
  for svc in api-gateway issue-status-sync; do
    if block "$root/$compose" "$svc" | grep -Eq '^[[:space:]]+REQUEST_SERVICE_ADDR:'; then
      ok "$compose sets REQUEST_SERVICE_ADDR for $svc"
    else
      bad "$compose does not set REQUEST_SERVICE_ADDR for $svc"
    fi
  done
fi

# (f) migrate.sh SERVICES
if check_file deploy/dev/scripts/migrate.sh; then
  line=$(grep -E '^[[:space:]]*SERVICES=' "$root/deploy/dev/scripts/migrate.sh" | head -n 1 || true)
  if printf '%s\n' "$line" | grep -Eq '[" ]request[" ]'; then
    ok "migrate.sh SERVICES has request"
  else
    bad "deploy/dev/scripts/migrate.sh SERVICES does not list request"
  fi
fi

# (g) workflow with the dialect matrix
wf=.github/workflows/backend-go-request-service.yml
if check_file "$wf"; then
  if grep -Eq 'dialect:[[:space:]]*\[[^]]*postgres[^]]*\]' "$root/$wf" && grep -Eq 'dialect:[[:space:]]*\[[^]]*mysql[^]]*\]' "$root/$wf"; then
    ok "$wf runs a postgres and mysql matrix"
  else
    bad "$wf has no matrix with both postgres and mysql"
  fi
fi

if [ "$fail" -ne 0 ]; then
  echo "request-service is not fully registered" >&2
  exit 1
fi
echo "request-service wiring complete"
