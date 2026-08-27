# TASK-AUTH-04-06: `handleCreateUser` request body gains `password`

**From Solution:** SOL-AUTH-04
**Priority:** P0
**Service:** `api-gateway` (httpgateway)
**File:** `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_admin_routes.go`
**Depends on:** TASK-AUTH-04-01, TASK-AUTH-04-02
**Status:** `[ ]` TODO

---

## Context

`POST /v1/auth/users` (and the `/admin/api/users` mount that reuses this same handler) currently never sends a password to `auth-service`, even though `docs/logic/auth/BL-AUTH-04-admin-user-crud.md`'s spec requires `{email, name, role, password}`. This is the REST-side half of TASK-AUTH-04-02.

## Changes to make

In `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_admin_routes.go`, change:

```go
type createUserRequestBody struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}
```

to:

```go
type createUserRequestBody struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Password string `json:"password"`
}
```

In `handleCreateUser`, change the `CreateUserRequest` construction:

```go
resp, err := client.CreateUser(ctx, &authv1.CreateUserRequest{
	Email:    body.Email,
	Name:     body.Name,
	TenantId: identity.TenantID,
	Role:     parseRole(body.Role),
	Password: body.Password,
})
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestAuthAdminRoutes -v
```

Expected: `POST /admin/api/users` (or `/v1/auth/users`) with `password` in the body results in a fake `AuthServiceClient.CreateUser` call receiving that exact plaintext in `req.Password` — end-to-end regression guard for the bug's headline symptom ("that account is permanently unusable"), together with TASK-AUTH-04-02's login round-trip test.
