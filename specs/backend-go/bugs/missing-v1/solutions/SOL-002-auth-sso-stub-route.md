# SOL-002: Add `GET /auth/sso/:provider` stub route

**Resolves:** [BUG-002](../BUG-002-auth-sso-route-missing.md)
**Service:** `api-gateway`
**Affected files (proposed):**
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes_test.go` (new)
**Status:** ✅ Implemented — all 2 task(s) (TASK-007–008) DONE; see each task file's own Status/Verify section for evidence.

---

## Design

No TDD architecture doc treats SSO as anything but a documented stub for
now — `auth-service.md` describes password-based `Login`/session issuance
only; there is no OAuth/SSO provider flow in its RPC surface, and
`GET /auth/config` already reports `providers: []` honestly. This is
therefore a one-route, low-risk fix that closes the response-shape gap
without inventing SSO support that isn't scoped anywhere.

Add to `mountAuthRoutes` (`auth_routes.go`), next to the existing
`/auth/config` handler:

```go
// SSO is not implemented anywhere in the target architecture yet
// (auth-service.md's RPC surface has no OAuth/provider-login concept) —
// /auth/config already reports providers: [] honestly. This route exists
// only so a client that somehow reaches it (stale build, provider added
// to /auth/config without this route following) gets the same documented
// 501 the old TS backend returned, instead of a bare 404.
mux.Get("/auth/sso/{provider}", func(w http.ResponseWriter, r *http.Request) {
    writeJSONError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "SSO login is not yet supported")
})
```

No proto/usecase change needed — this is purely a transport-layer
completeness fix, consistent with `mountTraceRoutes`'s existing precedent
of serving an honest placeholder rather than a bare 404 for a
documented-but-unbuilt surface.

## Test plan

```go
func TestAuthSSORoute_Returns501(t *testing.T) {
    r := chi.NewRouter()
    mountAuthRoutes(r, fakeAuthClient{}, fakeCookieValidator{})
    req := httptest.NewRequest(http.MethodGet, "/auth/sso/google", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != http.StatusNotImplemented {
        t.Errorf("want 501, got %d", w.Code)
    }
}
```

## References

- `specs/backend-go/tdd/services/auth-service.md` — confirms no OAuth/SSO RPC exists in the target design
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go` — `mountAuthRoutes`
