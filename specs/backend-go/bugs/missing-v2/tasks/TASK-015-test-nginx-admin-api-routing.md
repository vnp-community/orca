# TASK-015: CI check that `/admin/api/*` actually reaches `api-gateway` through the real nginx config

**From Solution:** SOL-007
**Priority:** P1
**Service:** deploy config, CI
**File:** a new CI script (location depends on this repo's CI tooling — see Step 1), `deploy/dev/README.md` (doc update, optional)
**Depends on:** TASK-014
**Status:** `[ ]` TODO

---

## Context

`nginx -t` (syntax validation) would not have caught BUG-007 — the config
was syntactically valid, just missing a `location` block entirely. The
only test that actually catches this class of bug is one that starts real
nginx with the real config and makes a real HTTP request through it,
matching how BUG-007 was originally found (going through the real deployed
edge, not direct-to-container).

## Changes to make

### Step 1 — routing-check script

```bash
#!/usr/bin/env bash
# ci/check-nginx-admin-api-routing.sh (or fold into an existing CI script —
# check for an existing deploy/dev-focused CI job first)
set -euo pipefail

cd "$(dirname "$0")/../deploy/dev"

docker compose up -d --wait

# Bootstrap admin credentials must be available in this CI environment's
# compose env (BOOTSTRAP_ADMIN_EMAIL/BOOTSTRAP_ADMIN_PASSWORD or the
# auto-generated-password log line) — read from env or grep the
# auth-service container's boot log for the one-time generated password if
# BOOTSTRAP_ADMIN_PASSWORD wasn't set explicitly for this CI run.
ADMIN_EMAIL="${BOOTSTRAP_ADMIN_EMAIL:?set for CI}"
ADMIN_PASSWORD="${BOOTSTRAP_ADMIN_PASSWORD:?set for CI}"

cookie_jar=$(mktemp)
trap 'rm -f "$cookie_jar"' EXIT

curl -sf -c "$cookie_jar" -X POST http://localhost:8080/auth/local \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}" > /dev/null

content_type=$(curl -s -o /dev/null -w '%{content_type}' -b "$cookie_jar" http://localhost:8080/admin/api/stats)

if [[ "$content_type" != application/json* ]]; then
  echo "FAIL: GET /admin/api/stats returned Content-Type: $content_type (expected application/json)" >&2
  echo "This means the request hit nginx's SPA fallback instead of api-gateway — see specs/backend-go/bugs/missing-v2/BUG-007" >&2
  docker compose down
  exit 1
fi

echo "OK: /admin/api/stats reaches api-gateway (Content-Type: $content_type)"
docker compose down
```

### Step 2 — wire into CI

Same placement question as TASK-007's Docker-image check — find this
repo's actual CI config location before assuming a path; the script above
is self-contained and runnable from any CI runner with Docker Compose
available, wire it to run on any PR touching
`deploy/dev/docker/nginx/orca.conf` or
`backend-go/services/api-gateway/internal/adapter/httpgateway/admin_routes.go`.

### Step 3 — regression-proof the check itself

Before committing, temporarily remove the `location /admin/api/` block
TASK-014 added and re-run this script — confirm it actually fails (not
just "the compose stack didn't come up" for an unrelated reason) before
trusting it as a real regression guard, then restore the block.

## Verify

```bash
bash ci/check-nginx-admin-api-routing.sh   # or wherever Step 2 wires this in
```

Expected: prints `OK: /admin/api/stats reaches api-gateway...` and exits
0. Re-running after deliberately reverting TASK-014's nginx change (Step 3
above) should print the `FAIL` message and exit 1 — confirm this manually
once, then restore the fix.
