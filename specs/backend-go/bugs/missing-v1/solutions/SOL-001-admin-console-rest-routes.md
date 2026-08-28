# SOL-001: Implement `/admin/api/*` per `auth-service.md`'s already-specified admin design

**Resolves:** [BUG-001](../BUG-001-admin-console-rest-surface-missing.md)
**Service:** `auth-service` (new RPCs) + `api-gateway` (new REST routes)
**Affected files (proposed):**
- `backend-go/proto/orca/auth/v1/auth.proto`
- `backend-go/services/auth-service/internal/usecase/*.go` (new use cases)
- `backend-go/services/auth-service/internal/adapter/postgres/*.go` (new repository methods)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/admin_routes.go` (new file)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go` (mount call)
**Status:** ✅ Implemented — all 6 task(s) (TASK-001–006) DONE; see each task file's own Status/Verify section for evidence.

---

## The design already exists — this is a gap-closing task, not a new design

`specs/backend-go/tdd/services/auth-service.md` already specifies the exact
admin-console backend this bug needs, under its "Admin console backend"
bullet (line 30) and detailed RPC list (lines 96-124):

> **Admin console backend** — user CRUD, session force-revoke,
> access-policy CRUD, audit-log query, first-run setup. Folded into
> `auth-service` because admin operations are RBAC operations on data this
> service already owns — a separate `admin-service` would just be a second
> front door onto the same tables.

The TDD's own RPC list (verbatim, this is the target contract):

| TDD-specified RPC | Backs which `/admin/api/*` route |
|---|---|
| `DeactivateUser`, `ReactivateUser` (auth-service.md:99) | `DELETE /admin/api/users/:id` |
| `ListSessionsForUser`, `ForceRevokeSession`, `ForceRevokeAllSessionsForUser` (auth-service.md:106-108) | `GET /admin/api/sessions`, `DELETE /admin/api/sessions/:sessionId`, `DELETE /admin/api/users/:userId/sessions` |
| `CreateAccessPolicy`, `GetAccessPolicy`, `ListAccessPolicies`, `UpdateAccessPolicy`, `DeleteAccessPolicy` (auth-service.md:111-113) | `POST`/`GET`/`PUT /:id`/`DELETE /:id` on `/admin/api/policies` |
| `QueryAuditLog` (auth-service.md:117) | `GET /admin/api/audit` (already REST-wired at the wrong path, `/v1/auth/audit-log` — reuse the same RPC, add the route) |

None of these RPCs exist in `backend-go/proto/orca/auth/v1/` today except
`QueryAuditLog`/`CreateUser`/`ListUsers`/`UpdateUserRole` (already wired at
`/v1/auth/*`, see BUG-001). This solution adds the rest and mounts all of
them a second time under `/admin/api/*` matching `http-endpoints.md`'s
exact contract, rather than replacing the existing `/v1/auth/*` surface
(some other REST-first consumer may already depend on it).

`GET /admin/api/stats` has **no** TDD-specified RPC anywhere — propose
adding a small `GetAdminStats` RPC to `auth-service` (counts are the
cheapest read: total users, active sessions, policies) rather than
fabricating a route with no backing data. Flag as a scope addition beyond
the TDD, not something to skip.

---

## Design — Proto additions (`auth.proto`)

```protobuf
// Deactivate / reactivate — per auth-service.md:99's explicit note that
// Go adds ReactivateUser deliberately (the TS admin console had no path
// back from deactivated).
rpc DeactivateUser(DeactivateUserRequest) returns (DeactivateUserResponse);
rpc ReactivateUser(ReactivateUserRequest) returns (ReactivateUserResponse);

// Session admin — auth-service.md:106-108
rpc ListSessionsForUser(ListSessionsForUserRequest) returns (ListSessionsForUserResponse);
rpc ForceRevokeAllSessionsForUser(ForceRevokeAllSessionsForUserRequest) returns (ForceRevokeAllSessionsForUserResponse);
// ForceRevokeSession already exists as RevokeSession — reuse, don't duplicate.

// Access policy CRUD — auth-service.md:111-113, backed by the
// access_policies table auth-service.md:172 already specifies
// (id, name, kind, document JSONB, version, updated_by, updated_at —
// an UPDATE creates a new version row, never an in-place mutation, per
// auth-service.md:150's "OPA bundle sync and audit both need a stable
// history" rule).
rpc CreateAccessPolicy(CreateAccessPolicyRequest) returns (AccessPolicy);
rpc GetAccessPolicy(GetAccessPolicyRequest) returns (AccessPolicy);
rpc ListAccessPolicies(ListAccessPoliciesRequest) returns (ListAccessPoliciesResponse);
rpc UpdateAccessPolicy(UpdateAccessPolicyRequest) returns (AccessPolicy);
rpc DeleteAccessPolicy(DeleteAccessPolicyRequest) returns (google.protobuf.Empty);

// Scope addition beyond the TDD — cheapest useful read for the admin
// dashboard landing view.
rpc GetAdminStats(GetAdminStatsRequest) returns (GetAdminStatsResponse);

message AccessPolicy {
  string id = 1;
  string name = 2;
  string kind = 3;         // "role-definition" | "rate-tier" | ... per auth-service.md:147
  string document_json = 4; // JSONB document, serialized
  int32 version = 5;
  string updated_by = 6;
  google.protobuf.Timestamp updated_at = 7;
}

message GetAdminStatsResponse {
  int32 total_users = 1;
  int32 active_sessions = 2;
  int32 total_policies = 3;
}
```

`buf breaking` must pass in CI per `08-inter-service-communication.md`'s
gRPC conventions — these are all additive (new RPCs/messages), so no
breaking change.

---

## Design — `usecase/` layer

Follow `03-clean-architecture-guidelines.md`'s layering: usecases depend on
repository *ports* (interfaces), never a concrete Postgres type directly.

```go
// internal/usecase/deactivate_user.go
type UserRepository interface {
    // ... existing methods ...
    SetActive(ctx context.Context, userID string, active bool) error
}

func (uc *UserUseCase) DeactivateUser(ctx context.Context, userID string) error {
    // Admin-only — enforced by the same requireAdminActor OPA check
    // mountAuthAdminRoutes already relies on (see auth_admin_routes.go's
    // doc comment) — no duplicate check needed here if the gRPC
    // interceptor already denies non-admin callers before reaching this
    // usecase (08-inter-service-communication.md's interceptor list).
    return uc.repo.SetActive(ctx, userID, false)
}
```

Access-policy versioning (per auth-service.md:150 — update = new version,
not in-place mutation):

```go
// internal/usecase/update_access_policy.go
func (uc *PolicyUseCase) UpdateAccessPolicy(ctx context.Context, id string, doc json.RawMessage, actorID string) (*domain.AccessPolicy, error) {
    current, err := uc.repo.Get(ctx, id)
    if err != nil {
        return nil, err
    }
    next := domain.AccessPolicy{
        ID: current.ID, Name: current.Name, Kind: current.Kind,
        Document: doc, Version: current.Version + 1, UpdatedBy: actorID,
    }
    if err := uc.repo.InsertVersion(ctx, next); err != nil {
        return nil, err
    }
    // Publish to wherever the OPA bundle registry lives — auth-service.md:194's
    // PolicyDataPublisher port. Outbox pattern (05-data-architecture.md):
    // this publish must be transactional with the insert above, not a
    // separate step that can drift if it fails independently.
    return &next, uc.publisher.PublishPolicyChange(ctx, next)
}
```

---

## Design — REST routes (`api-gateway`)

New file `admin_routes.go`, mirroring `auth_admin_routes.go`'s existing
pattern exactly (identity from context, `gatewaygrpc.AttachIdentity`,
`writeGRPCError` on failure) but mounted at `/admin/api` instead of
`/v1/auth`:

```go
func mountAdminRoutes(r chi.Router, client authv1.AuthServiceClient) {
    r.Route("/admin/api", func(sub chi.Router) {
        sub.Get("/stats", handleAdminStats(client))
        sub.Get("/users", handleListUsers(client))          // reuse existing handler
        sub.Post("/users", handleCreateUser(client))         // reuse existing handler
        sub.Patch("/users/{id}", handleUpdateUser(client))   // NEW — full edit, not just role
        sub.Delete("/users/{id}", handleDeactivateUser(client))
        sub.Get("/sessions", handleListAllSessions(client))
        sub.Delete("/sessions/{sessionId}", handleRevokeSessionByID(client))
        sub.Delete("/users/{userId}/sessions", handleForceRevokeAllSessions(client))
        sub.Get("/policies", handleListPolicies(client))
        sub.Post("/policies", handleCreatePolicy(client))
        sub.Put("/policies/{id}", handleUpdatePolicy(client))
        sub.Delete("/policies/{id}", handleDeletePolicy(client))
        sub.Get("/audit", handleQueryAuditLog(client))        // reuse existing handler
    })
}
```

`router.go`: add `if deps.AuthClient != nil { mountAdminRoutes(authed, deps.AuthClient) }`
next to the existing `mountAuthAdminRoutes` call — same client, same
`authed` group (admin-only enforcement happens server-side in
`auth-service`, per `auth_admin_routes.go`'s doc comment, so no new
gateway-level admin check is needed, only the existing session
authentication).

`PATCH /admin/api/users/:id` needs a corresponding `UpdateUser` RPC (not
just `UpdateUserRole`) if it's meant to edit `email`/`name` too — flag this
as an additional proto RPC beyond the table above if full-edit semantics
are required; if the admin console only ever edits role in practice,
narrow the frontend contract instead (a product decision, not purely
backend).

---

## Test plan

- `services/auth-service/internal/usecase/deactivate_user_test.go` — deactivate then reactivate round-trips `is_active`.
- `access_policy_versioning_test.go` — `UpdateAccessPolicy` twice → 2 rows, `version` 1 then 2, `GetAccessPolicy` returns latest.
- `services/api-gateway/internal/adapter/httpgateway/admin_routes_test.go` — one test per route, mirroring `auth_admin_routes_test.go`'s existing shape (fake gRPC client, assert status code + body shape).
- Contract test: `/admin/api/audit` and `/v1/auth/audit-log` both resolve to the same `QueryAuditLog` RPC — assert identical response shape from both paths (regression guard against the two REST surfaces drifting).

## References

- `specs/backend-go/tdd/services/auth-service.md:30,96-124,147-173,193-200` — the target design this solution implements
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` — usecase/repository layering
- `specs/backend-go/tdd/architecture/05-data-architecture.md` — outbox pattern for `PolicyDataPublisher`
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md` — gRPC conventions, admin interceptor
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_admin_routes.go` — pattern to mirror
