# BUG-007: `/admin/api/*` is implemented in `api-gateway` but nginx never proxies to it — regression vs. `missing-v1/BUG-001`'s "Resolved" status

**Service:** deploy config (`deploy/dev/docker/nginx/orca.conf`), not `api-gateway` itself
**File:** `deploy/dev/docker/nginx/orca.conf`
**Severity:** High — the entire admin console REST surface (11 routes per `missing-v1/BUG-001`) is unreachable on the live deployment despite the backend Go code existing
**Symptom:** `GET /admin/api/stats` (and `/admin/api/users`, `/admin/api/audit`, …) returns `200 text/html` (the admin SPA's `index.html`, via nginx's SPA-fallback `try_files`) instead of JSON. `POST /admin/api/users` returns a raw nginx `405 Not Allowed` page (static-file serving doesn't permit POST).
**Status:** 🔴 Open — found live 2026-08-27 via `tests/client/auth-api-client.spec.ts`/`web-client.spec.ts`, **contradicts `../missing-v1/BUG-001-admin-console-rest-surface-missing.md`'s "✅ Resolved" status**.

---

## Description

`api-gateway` genuinely implements the admin REST surface:

```go
// backend-go/services/api-gateway/internal/adapter/httpgateway/admin_routes.go:28
r.Route("/admin/api", func(sub chi.Router) {
	// ... GET /admin/api/stats, /admin/api/users, /admin/api/audit,
	//     DELETE /admin/api/users/:id, GET /admin/api/sessions, ...
})
```

But `deploy/dev/docker/nginx/orca.conf` — the only nginx config actually
mounted on this deployment (per BUG-005's own investigation:
`/ws` proxies to `api-gateway`, confirmed real) — has no location block
that proxies `/admin/api/*` anywhere:

```nginx
# orca.conf:98-120 (every location block in the file, in order)
location /v1/ {
	proxy_pass $api_gateway;
	# ...
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

`/admin/api/stats` matches `location /admin/` (the longest matching
prefix — there's no more specific `/admin/api/` block), which only serves
static files with an SPA fallback to `/admin-index.html`. It never reaches
`location /v1/`'s `proxy_pass`, and `api-gateway`'s real
`/admin/api/*` routes are never proxied to at all — nginx doesn't even
know `api-gateway` exists for this path prefix.

## Confirmed

- `backend-go/services/api-gateway/internal/adapter/httpgateway/admin_routes.go:17-28` —
  the route group genuinely exists (`specs/frontend/api/http-endpoints.md`
  referenced directly in its own doc comment).
- `deploy/dev/docker/nginx/orca.conf:96-120` — full, exhaustive list of
  every `location` block in the file; none proxies `/admin/api/*` to
  `api-gateway`.
- Live-verified 2026-08-27 against `172.20.2.39:6769` (through the real
  deployed nginx, `Server: nginx/1.27.5` in the response headers — not a
  shortcut/direct-to-container test):
  - `GET /admin/api/stats` (authenticated, valid admin cookie) →
    `200 text/html`, body starts with `<!DOCTYPE html>...<title>Orca Admin</title>`
    (the admin SPA shell).
  - `POST /admin/api/users` (same session) → `405 Not Allowed`, nginx's own
    default error page (`<html><head><title>405 Not Allowed</title>...`,
    not a JSON error body — proves this never reaches Go code at all).

## Why This Contradicts `missing-v1/BUG-001`

`missing-v1/README.md` and `BUG-001-admin-console-rest-surface-missing.md`
both currently claim this is `✅ Resolved`. Two possibilities, not
distinguished by this report:
1. The `admin_routes.go` handler was built and verified some other way
   (unit test, or a `curl` direct to the `api-gateway` container bypassing
   nginx) but the nginx config change to actually expose it was never made
   or never deployed to `172.20.2.39`.
2. A nginx config regression after BUG-001 was marked resolved — e.g. a
   redeploy that reverted `orca.conf` to an earlier version.

Either way, **from a real client's point of view through the actual public
edge, this is unresolved** — worth reconciling with whoever marked BUG-001
done before trusting that status again.

## Suggested Fix

Add a `location /admin/api/` block to `orca.conf`, proxying to `$api_gateway`
the same way `location /v1/` does, positioned so it takes priority over
`location /admin/`'s SPA-fallback block (nginx matches the longest literal
prefix, so `/admin/api/` already wins over `/admin/` — just needs to exist).
