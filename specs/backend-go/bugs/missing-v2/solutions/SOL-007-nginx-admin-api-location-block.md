# SOL-007: Fix BUG-007 — add an explicit `/admin/api/` nginx location proxying to `api-gateway`

**Resolves:** BUG-007
**Service:** deploy config (`deploy/dev/docker/nginx/orca.conf`)
**Affected files:** `deploy/dev/docker/nginx/orca.conf`
**Priority:** High
**Status:** 🟡 Proposed — not yet implemented

---

## Grounding in `specs/backend-go/tdd/`

`orca.conf`'s own header comment already cites
`specs/backend-go/architecture/08-inter-service-communication.md` as its
design source, and that doc is unambiguous about the routing rule this
file is meant to implement:

> **API Gateway responsibilities** ... 1. Terminates TLS. ... 4. Routes
> REST requests to the appropriate service via `grpc-gateway`... `api-gateway`
> is the only service exposed outside the cluster network.

Every REST path — `/admin/api/*` included, since
`httpgateway/admin_routes.go` is real, in-process `api-gateway` code, not
a separate service — belongs behind this same routing rule. `orca.conf`
already gets this right for `/v1/*` (the generated `grpc-gateway` REST
facade) and `/auth/*` (hand-written `api-gateway` routes, per that
location block's own comment referencing `specs/frontend/api/http-endpoints.md`);
`/admin/api/*` is the one hand-written REST route group `orca.conf` never
added a matching block for — an omission in this file specifically, not a
gap in the design doc's routing rule itself.

## Design

Add a `location /admin/api/` block, positioned so nginx's longest-prefix
matching picks it over the existing `location /admin/` SPA-fallback block
(nginx doesn't require ordering for this — longest literal prefix always
wins — but placing it visually next to `/admin/` keeps intent readable for
the next person editing this file):

```nginx
# orca.conf — insert before the "SPA fallback" comment block

# /admin/api/* — api-gateway's admin console REST surface
# (httpgateway/admin_routes.go). Same proxy shape as /v1/ below; kept as
# its own block (not folded into /v1/) because it doesn't share that
# prefix and because admin-console auth semantics are distinct enough to
# want its own location if that ever needs route-specific config (e.g. a
# tighter rate limit) without touching /v1/'s.
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
root /usr/share/nginx/html;
index web-index.html;
location /admin/ {
    try_files $uri $uri/ /admin-index.html;
}
location / {
    try_files $uri $uri/ /web-index.html;
}
```

The proxy directives are copied verbatim from the existing `location
/v1/` block (`orca.conf:98-107`) rather than introduced as a new pattern —
same backend, same timeout posture, no reason for this route group to
behave differently at the transport level.

## Reconciling with `missing-v1/BUG-001`'s "Resolved" status

Per BUG-007's own report, this contradicts a status already marked
`✅ Resolved` elsewhere. This solution doesn't relitigate whether
`admin_routes.go` itself was correctly implemented and verified at the
time (it plausibly was, in isolation) — it only closes the gap that makes
it unreachable through the real deployed edge. Whoever applies this fix
should also:
- Check whether `172.20.2.39`'s nginx config is deployed from this exact
  repo file or has drifted (a separate ops question this solution doesn't
  answer) — if it's supposed to auto-deploy from `orca.conf`, that
  deployment pipeline itself may have a gap worth a follow-up bug.
- Update `missing-v1/BUG-001-admin-console-rest-surface-missing.md`'s
  status once this is verified live, noting the nginx-routing regression
  found here, so that report's history stays accurate rather than
  silently re-flipping to "Resolved" with no trace of this having broken
  again.

## Testing Plan

- **This bug is unreachable from `project-service`-style Go unit
  tests** — it's pure nginx config, so the regression test has to exercise
  the config itself. Add an nginx-config test (e.g. `docker run` the real
  nginx image with this config bind-mounted, against a stub `api-gateway`
  backend, and assert `GET /admin/api/stats` reaches the stub — not just
  that the config file parses) to CI, since `nginx -t` alone (syntax
  validation) would not have caught BUG-007 — the config was syntactically
  valid, just incomplete.
- Manual/CI smoke check post-deploy: `curl` (through the real edge, not
  direct-to-container — BUG-007's own investigation found the bug
  specifically by going through the real `nginx` hop) `GET /admin/api/stats`
  with a valid admin session cookie → expect `200 application/json`, not
  `200 text/html`.
- Re-run `tests/client/auth-api-client.spec.ts`'s "Admin API Access"
  describe block and `tests/client/web-client.spec.ts`'s "F25 — Admin API
  Client" describe block against the fixed deployment — both should move
  from JSON-parse failures / HTML responses to their intended assertions
  passing.
