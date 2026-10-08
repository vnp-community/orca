#!/usr/bin/env bash
# Self-test of check-request-service-wiring.sh: the intact tree passes, and removing request from any one
# registration place makes the script fail naming that place. Works on GNU and BSD userland (no sed -i, no grep -P).
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
real_root="$(cd "$here/../.." && pwd)"
script="$here/check-request-service-wiring.sh"
files=(
  backend-go/go.work
  backend-go/Makefile
  backend-go/deploy/postgres-init-databases.sh
  deploy/dev/docker-compose.yml
  deploy/dev/scripts/migrate.sh
  .github/workflows/backend-go-request-service.yml
)

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

fresh_copy() {
  rm -rf "$tmp/tree"
  for f in "${files[@]}"; do
    mkdir -p "$tmp/tree/$(dirname "$f")"
    cp "$real_root/$f" "$tmp/tree/$f"
  done
}

# rewrite FILE CMD...: run CMD with the file on stdin, replace the file with its stdout.
rewrite() {
  local f="$tmp/tree/$1"
  shift
  "$@" < "$f" > "$f.new"
  mv "$f.new" "$f"
}

# drop_in_block NAME: delete REQUEST_SERVICE_ADDR lines inside compose service NAME.
drop_in_block() {
  awk -v name="$1" '
    $0 ~ "^  " name ":[[:space:]]*$" { on = 1 }
    on && $0 ~ "^  [A-Za-z0-9_-]+:" && $0 !~ "^  " name ":" { on = 0 }
    !(on && $0 ~ /REQUEST_SERVICE_ADDR:/) { print }
  '
}

failures=0
run() {
  WIRING_ROOT="$tmp/tree" bash "$script" > "$tmp/out" 2> "$tmp/err" && echo 0 || echo $?
}

expect_pass() {
  fresh_copy
  if [ "$(run)" != "0" ]; then
    echo "FAIL: the intact tree must pass" >&2
    cat "$tmp/err" >&2
    failures=$((failures + 1))
  else
    echo "ok   intact tree passes"
  fi
}

# expect_fail NAME MESSAGE_FRAGMENT: the tree was mutated by the caller.
expect_fail() {
  if [ "$(run)" = "0" ]; then
    echo "FAIL: '$1': the script passed although the registration is missing" >&2
    failures=$((failures + 1))
  elif ! grep -Fq "$2" "$tmp/err"; then
    echo "FAIL: '$1': the script failed but did not say '$2':" >&2
    cat "$tmp/err" >&2
    failures=$((failures + 1))
  else
    echo "ok   $1"
  fi
}

expect_pass

fresh_copy; rewrite backend-go/go.work grep -v 'services/request-service'
expect_fail "go.work without request-service" "go.work does not list"

fresh_copy; rewrite backend-go/Makefile sed -E 's/request-service//g'
expect_fail "Makefile SERVICES without request-service" "Makefile SERVICES does not list request-service"

fresh_copy; rewrite backend-go/deploy/postgres-init-databases.sh sed -E '/^DATABASES=/ s/ request"/"/'
expect_fail "postgres-init-databases.sh without request" "DATABASES does not list request"

fresh_copy; rewrite deploy/dev/docker-compose.yml sed -E 's/^  request-service:/  request-servicex:/'
expect_fail "compose without the request-service service" "does not define the service request-service"

fresh_copy; rewrite deploy/dev/docker-compose.yml sed -E 's/^  migrate-request:/  migrate-requestx:/'
expect_fail "compose without migrate-request" "does not define the service migrate-request"

fresh_copy; rewrite deploy/dev/docker-compose.yml drop_in_block api-gateway
expect_fail "compose without REQUEST_SERVICE_ADDR for api-gateway" "REQUEST_SERVICE_ADDR for api-gateway"

fresh_copy; rewrite deploy/dev/docker-compose.yml drop_in_block issue-status-sync
expect_fail "compose without REQUEST_SERVICE_ADDR for issue-status-sync" "REQUEST_SERVICE_ADDR for issue-status-sync"

fresh_copy; rewrite deploy/dev/scripts/migrate.sh sed -E '/^[[:space:]]*SERVICES=/ s/ request"/"/'
expect_fail "migrate.sh without request" "migrate.sh SERVICES does not list request"

fresh_copy; rewrite .github/workflows/backend-go-request-service.yml sed -E 's/\[postgres, mysql\]/[postgres]/'
expect_fail "workflow without the mysql matrix" "no matrix with both postgres and mysql"

fresh_copy; rm "$tmp/tree/.github/workflows/backend-go-request-service.yml"
expect_fail "workflow missing" "backend-go-request-service.yml does not exist"

if [ "$failures" -ne 0 ]; then
  echo "$failures wiring-check self-test(s) failed" >&2
  exit 1
fi
echo "all wiring-check self-tests passed"
