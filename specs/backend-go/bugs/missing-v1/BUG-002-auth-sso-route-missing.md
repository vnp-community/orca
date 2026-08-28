# BUG-002: `GET /auth/sso/:provider` route not registered (404 instead of a stub 501)

**Service:** `api-gateway`
**File:** `internal/adapter/httpgateway/auth_routes.go` — `mountAuthRoutes`
**Severity:** Low — SSO is a stub everywhere (old backend included), but the *behavior* on a hit still regresses
**Symptom:** `SsoButton` (`LoginPage.tsx`) hitting `GET /auth/sso/:provider` gets chi's default `404 Not Found` page instead of the documented, structured `501`
**Status:** ✅ Resolved — see TASK-007–008 (2 task(s), all DONE) for implementation evidence.

---

## Description

`specs/frontend/api/http-endpoints.md` documents:

> `GET /auth/sso/:provider` — SSO login kick-off — **stub**: backend returns
> `501` unconditionally today.

`mountAuthRoutes` (`auth_routes.go:56`) registers `POST /auth/local`,
`GET /auth/me`, `POST /auth/logout`, and `GET /auth/config` — matching the
rest of the spec exactly — but has no `mux.Get("/auth/sso/{provider}", ...)`
route at all:

```
$ grep -n "sso" backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go
(no matches)
```

`auth_routes.go`'s own `/auth/config` handler already reports the honest
state (`"providers": []`), so `SsoButton` should never render in practice —
but if it ever does (stale frontend build, provider added to `/auth/config`
without the route following), the request 404s through chi's default
handler instead of the documented `501`. This is a minor drift from the
spec's contract, not a functional blocker, since `providers: []` means no
SSO button is ever shown today.

---

## Fix

Add a route matching the old backend's stub behavior:

```go
mux.Get("/auth/sso/{provider}", func(w http.ResponseWriter, r *http.Request) {
    writeJSONError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "SSO login is not yet supported")
})
```

## References

- `specs/frontend/api/http-endpoints.md` — `## Auth (/auth/*)`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go` — `mountAuthRoutes`
