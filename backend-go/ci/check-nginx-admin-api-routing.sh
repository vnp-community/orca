#!/usr/bin/env bash
# ci/check-nginx-admin-api-routing.sh (or fold into an existing CI script —
# check for an existing deploy/dev-focused CI job first)
#
# NOTE: this script lives at backend-go/ci/ (no repo-root ci/ or other
# deploy/dev-focused CI job convention existed at the time this was added —
# see TASK-015). The relative path below is adjusted accordingly
# (../../deploy/dev, not ../deploy/dev as in the task spec, which assumed a
# repo-root ci/ directory). Step 2 (wiring this into actual CI) is left as a
# follow-up — this repo's CI tooling location wasn't identified.
set -euo pipefail

cd "$(dirname "$0")/../../deploy/dev"

# Why: docker-compose.yml maps ${FRONTEND_HTTP_PORT:-8080} on the HOST to
# the frontend container's internal 8080 — the host port is whatever .env
# sets (this repo's checked-in dev/staging .env uses 6769), not always
# literally 8080. Read it the same way compose itself does, defaulting to
# 8080 only when unset — found via live verification (TASK-015), the
# original hardcoded ":8080" would silently probe the wrong port in any
# environment whose .env overrides FRONTEND_HTTP_PORT.
FRONTEND_HTTP_PORT="${FRONTEND_HTTP_PORT:-8080}"
BASE_URL="http://localhost:${FRONTEND_HTTP_PORT}"

# Why: also override SERVER_BIND_IP for this check specifically — a real
# deploy's .env may bind the published port to that deploy target's own
# LAN/public IP (not a loopback-reachable address from wherever this CI
# check runs), which curl-ing "localhost" can never reach regardless of
# port. 127.0.0.1 is always correct for a check running on the same host
# docker compose just bound the port on.
SERVER_BIND_IP=127.0.0.1 docker compose up -d --wait

# Bootstrap admin credentials must be available in this CI environment's
# compose env (BOOTSTRAP_ADMIN_EMAIL/BOOTSTRAP_ADMIN_PASSWORD or the
# auto-generated-password log line) — read from env or grep the
# auth-service container's boot log for the one-time generated password if
# BOOTSTRAP_ADMIN_PASSWORD wasn't set explicitly for this CI run.
ADMIN_EMAIL="${BOOTSTRAP_ADMIN_EMAIL:?set for CI}"
ADMIN_PASSWORD="${BOOTSTRAP_ADMIN_PASSWORD:?set for CI}"

cookie_jar=$(mktemp)
trap 'rm -f "$cookie_jar"' EXIT

curl -sf -c "$cookie_jar" -X POST "${BASE_URL}/auth/local" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}" > /dev/null

content_type=$(curl -s -o /dev/null -w '%{content_type}' -b "$cookie_jar" "${BASE_URL}/admin/api/stats")

if [[ "$content_type" != application/json* ]]; then
  echo "FAIL: GET /admin/api/stats returned Content-Type: $content_type (expected application/json)" >&2
  echo "This means the request hit nginx's SPA fallback instead of api-gateway — see specs/backend-go/bugs/missing-v2/BUG-007" >&2
  docker compose down
  exit 1
fi

echo "OK: /admin/api/stats reaches api-gateway (Content-Type: $content_type)"
docker compose down
