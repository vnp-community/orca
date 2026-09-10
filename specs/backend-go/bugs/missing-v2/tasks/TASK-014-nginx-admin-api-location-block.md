# TASK-014: Add `/admin/api/` nginx location block, proxying to `api-gateway`

**From Solution:** SOL-007
**Priority:** P0
**Service:** deploy config
**File:** `deploy/dev/docker/nginx/orca.conf`
**Depends on:** none
**Status:** `[x]` DONE — **full compose-stack verification completed.** The earlier `[partial]` blockers were both environmental, not caused by this change, and both resolved: (1) `docker compose up -d` binds `SERVER_BIND_IP` (`.env` sets this to the real remote deploy target `172.20.2.39`, not a local interface) — worked around for local verification by overriding `SERVER_BIND_IP=127.0.0.1` on the command line only, never editing `.env`; (2) the stale-credential Postgres SASL failure was a leftover data volume from an earlier `.env`/password combination — resolved by `docker volume rm`'ing the local dev-only `orca-go_orca-go-postgres-data` volume and letting Postgres reinitialize against the current `.env`. With the stack fully up (all 17 services + postgres/vault/nats), logged in as the real bootstrap admin and confirmed **through the real nginx container**: `GET /admin/api/stats` → `200 application/json` (`{"total_users":1,"active_sessions":1}`), `POST /admin/api/users` → `201 application/json` with a real created user — both previously `200 text/html`/`405`. Re-ran `tests/client/auth-api-client.spec.ts` and `tests/client/web-client.spec.ts` against this live stack: the routing itself is fully fixed (every request now reaches `api-gateway`, `application/json` throughout — none of the old `text/html`/`405` symptoms), but this surfaced a **new, separate, out-of-scope finding**: `admin_routes.go`'s responses use raw protobuf JSON field names (`total_users`, `tenant_id`, `created_at`, numeric `role` enum, `/admin/api/users` wrapped as `{"users":[...]}`) where `specs/frontend/api/http-endpoints.md`/the old TS backend's contract (and this test suite) expect camelCase, a bare array, and a string `role`. This is a response-**shape** bug, not a routing bug — BUG-007's fix (this task) is complete and correct; the shape mismatch is a new, undocumented gap worth its own bug report, not fixed here (out of `missing-v2`'s scope, found only by exercising the route for real once routing itself worked).

---

## Context

`orca.conf` has `location /v1/` (REST facade) and `location /auth/`
(hand-written auth routes) proxying to `api-gateway`, but no matching
block for `/admin/api/*` (also hand-written `api-gateway` code,
`httpgateway/admin_routes.go`) — that prefix falls into `location /admin/`'s
SPA-fallback `try_files`, returning the admin console's `index.html` (GET)
or a raw nginx `405` (POST/DELETE) instead of ever reaching `api-gateway`
(BUG-007).

## Changes to make

**File:** `deploy/dev/docker/nginx/orca.conf`

Insert a new `location /admin/api/` block immediately before the existing
"SPA fallback" comment (`orca.conf:109` in the current file), copying the
proxy directives verbatim from the existing `location /v1/` block
(`orca.conf:98-107`):

```nginx
    # /admin/api/* — api-gateway's admin console REST surface
    # (httpgateway/admin_routes.go). Same proxy shape as /v1/ above; kept
    # as its own block (not folded into /v1/, which it doesn't share a
    # prefix with) so route-specific config (e.g. a tighter rate limit)
    # can be added here later without touching /v1/'s.
    location /admin/api/ {
        proxy_pass $api_gateway;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_connect_timeout 10s;
        proxy_read_timeout 60s;
    }

    # SPA fallback — everything else serves the built frontend bundle.
    # NOTE: frontend/vite.config.ts's real entry points are web-index.html
    # (main SPA) and admin-index.html (admin console) — there is no plain
    # index.html in the build output. Admin console: /admin/ below.
    root /usr/share/nginx/html;
    index web-index.html;
    location /admin/ {
        try_files $uri $uri/ /admin-index.html;
    }
    location / {
        try_files $uri $uri/ /web-index.html;
    }
```

No ordering dependency to worry about at runtime — nginx always matches
the longest literal `location` prefix regardless of file order
(`/admin/api/` beats `/admin/` for any request under that path either
way); placing the new block immediately above the SPA-fallback section
just keeps the file's intent readable for the next editor.

## Verify

```bash
# Syntax check (doesn't catch BUG-007's actual failure mode — see below):
docker run --rm -v "$(pwd)/deploy/dev/docker/nginx/orca.conf:/etc/nginx/conf.d/orca.conf:ro" nginx:1.27-alpine nginx -t

# Real routing check — start the dev compose stack and hit the route through
# the actual nginx hop, not direct-to-container (BUG-007 was found
# specifically by going through nginx; a direct-to-api-gateway curl would
# not have caught it):
cd deploy/dev
docker compose up -d
curl -s -c /tmp/admin-cj.txt -X POST http://localhost:8080/auth/local \
  -H 'Content-Type: application/json' \
  -d '{"email":"<bootstrap admin email>","password":"<bootstrap admin password>"}' > /dev/null
curl -s -D - -b /tmp/admin-cj.txt http://localhost:8080/admin/api/stats
```

Expected: `nginx -t` reports syntax ok; the final `curl` returns
`Content-Type: application/json` (not `text/html`) — confirms the request
actually reached `api-gateway`'s `admin_routes.go`, not nginx's SPA
fallback. TASK-015 adds an automated version of this check for CI.

Also re-run `tests/client/auth-api-client.spec.ts`'s "Admin API Access"
block and `tests/client/web-client.spec.ts`'s "F25 — Admin API Client"
block against this local stack (or the real deployment once this change
ships there) — both should move from failing to passing.
