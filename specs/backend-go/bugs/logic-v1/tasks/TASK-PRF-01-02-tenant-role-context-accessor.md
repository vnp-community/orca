# TASK-PRF-01-02: Add `Role(ctx)` accessor to `common/tenant`

**From Solution:** SOL-PRF-01
**Priority:** P0 — the authorization usecase (TASK-PRF-01-03) reads this on every gated call
**Service:** `tenant-service` (edits a `common/` package shared by every service)
**File:** `backend-go/common/tenant/tenant.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`tenant-service`'s new RBAC checks (company-admin, department-lead) need the
caller's role, but no role claim propagates from api-gateway into a
service's request context yet — the same pre-existing gap
`project-service`'s `callerGlobalRole` stub already documents. This task adds
the accessor parallel to the existing `TenantID`/`UserID` ones so
`tenant-service`'s authorization code has somewhere real to read from once
the upstream propagation gap closes; until then it returns `"", false` and
callers fail closed (deny).

## Changes to make

In `backend-go/common/tenant/tenant.go`, add a `roleKey` alongside the
existing keys and a `WithRole`/`Role` pair mirroring `WithUserID`/`UserID`:

```go
var (
	tenantIDKey = &contextKey{"tenant_id"}
	userIDKey   = &contextKey{"user_id"}
	roleKey     = &contextKey{"role"} // NEW
)
```

```go
// WithRole attaches the caller's role claim to ctx — populated by the
// inbound gRPC interceptor once JWT-role-claim propagation from api-gateway
// lands (tracked at project-service/internal/usecase/authorization.go's
// callerGlobalRole stub). Not called anywhere in this codebase yet — see
// Role's doc comment.
func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, roleKey, role)
}

// Role returns the caller's role and whether one was present. Returns
// ("", false) until the upstream role-claim-propagation gap closes (see
// WithRole's doc comment) — every caller of this function must treat that
// as "unknown role" and fail closed (deny), never as an implicit grant.
func Role(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(roleKey).(string)
	return v, ok && v != ""
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./common/tenant/...
go test ./common/tenant/... -v
```

Add a `tenant_test.go` case (or extend the existing test file) asserting
`Role` returns `("", false)` on a bare `context.Background()` and the set
value round-trips through `WithRole`/`Role`.
