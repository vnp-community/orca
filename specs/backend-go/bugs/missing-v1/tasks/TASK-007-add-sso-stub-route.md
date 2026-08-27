# TASK-007: Add `GET /auth/sso/:provider` 501 stub route

**From Solution:** SOL-002
**Priority:** P2
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/httpgateway/auth_routes.go`
**Depends on:** none
**Status:** `[x]` DONE — `GET /auth/sso/{provider}` returns the documented 501 `NOT_IMPLEMENTED` stub in `auth_routes.go`; build/vet clean.

---

## Changes to make

In `mountAuthRoutes`, add next to the existing `/auth/config` handler:

```go
// SSO is not implemented anywhere in the target architecture yet
// (auth-service.md's RPC surface has no OAuth/provider-login concept) —
// /auth/config already reports providers: [] honestly. This route exists
// only so a client that somehow reaches it gets the same documented 501
// the old TS backend returned, instead of a bare 404.
mux.Get("/auth/sso/{provider}", func(w http.ResponseWriter, r *http.Request) {
    writeJSONError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "SSO login is not yet supported")
})
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```
