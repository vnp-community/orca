# TASK-005: Mount `/admin/api/*` REST routes in `api-gateway`

**From Solution:** SOL-001
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/httpgateway/admin_routes.go` (new), `router.go`
**Depends on:** TASK-001, TASK-002, TASK-003, TASK-004
**Status:** `[x]` DONE — `admin_routes.go` mounts all 12 `/admin/api/*` routes (including the documented role-only narrowing for `PATCH /users/:id`) and `router.go` wires `mountAdminRoutes` alongside `mountAuthAdminRoutes`; build/vet clean.

---

## Context

Mirrors `auth_admin_routes.go`'s existing pattern exactly (identity from
context, `gatewaygrpc.AttachIdentity`, `writeGRPCError` on failure) but at
`/admin/api` instead of `/v1/auth`, matching `http-endpoints.md`'s literal
contract.

## Changes to make

### New file `admin_routes.go`

```go
func mountAdminRoutes(r chi.Router, client authv1.AuthServiceClient) {
    r.Route("/admin/api", func(sub chi.Router) {
        sub.Get("/stats", handleAdminStats(client))
        sub.Get("/users", handleListUsers(client))          // reuse existing handler from auth_admin_routes.go
        sub.Post("/users", handleCreateUser(client))         // reuse existing handler
        sub.Patch("/users/{id}", handleUpdateUserRole(client)) // reuse; see open question below
        sub.Delete("/users/{id}", handleDeactivateUser(client))
        sub.Get("/sessions", handleListAllSessions(client))
        sub.Delete("/sessions/{sessionId}", handleRevokeSessionByID(client)) // reuse handleRevokeSession
        sub.Delete("/users/{userId}/sessions", handleForceRevokeAllSessions(client))
        sub.Get("/policies", handleListPolicies(client))
        sub.Post("/policies", handleCreatePolicy(client))
        sub.Put("/policies/{id}", handleUpdatePolicy(client))
        sub.Delete("/policies/{id}", handleDeletePolicy(client))
        sub.Get("/audit", handleQueryAuditLog(client))        // reuse existing handler
    })
}

func handleAdminStats(client authv1.AuthServiceClient) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        identity, _ := identityFromContext(r.Context())
        ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
        resp, err := client.GetAdminStats(ctx, &authv1.GetAdminStatsRequest{})
        if err != nil {
            writeGRPCError(w, err)
            return
        }
        writeJSON(w, http.StatusOK, resp)
    }
}

func handleDeactivateUser(client authv1.AuthServiceClient) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        identity, _ := identityFromContext(r.Context())
        id := chi.URLParam(r, "id")
        ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
        resp, err := client.DeactivateUser(ctx, &authv1.DeactivateUserRequest{UserId: id})
        if err != nil {
            writeGRPCError(w, err)
            return
        }
        writeJSON(w, http.StatusOK, resp.GetUser())
    }
}

// handleListAllSessions, handleForceRevokeAllSessions, handleListPolicies,
// handleCreatePolicy, handleUpdatePolicy, handleDeletePolicy follow the
// same shape — decode path/query params, AttachIdentity, call the gRPC
// client, writeGRPCError or writeJSON. Write each following
// handleCreateUser's exact structure in auth_admin_routes.go.
```

### `router.go`

```go
if deps.AuthClient != nil {
    mountAuthAdminRoutes(authed, deps.AuthClient) // existing — leave as-is
    mountAdminRoutes(authed, deps.AuthClient)      // NEW
}
```

### Open question to resolve before/during this task

`PATCH /admin/api/users/:id` in `http-endpoints.md` implies a full user
edit (email/name), but the only RPC available is `UpdateUserRole`
(role-only). Either: (a) narrow this route to role-only for now and file a
follow-up for a real `UpdateUser` RPC, or (b) add `UpdateUser` to TASK-001's
proto scope before starting this task. Default to (a) — ship the
role-only version now, note the limitation in the handler's doc comment.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```
