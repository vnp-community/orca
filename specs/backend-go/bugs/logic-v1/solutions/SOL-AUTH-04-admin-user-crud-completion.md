# SOL-AUTH-04: Admin-set password on user creation, cross-tenant session listing, full `UpdateUser`

**Resolves:** [BUG-AUTH-04](../BUG-AUTH-04-admin-user-crud-partial.md)
**Service:** `auth-service` (proto, usecase, postgres) + `api-gateway` (REST wiring)
**Affected files (proposed):**
- `backend-go/proto/orca/auth/v1/auth.proto` — `CreateUserRequest` gains `password`; new `ListSessions` RPC; new `UpdateUser` RPC
- `backend-go/services/auth-service/internal/usecase/create_user.go` — accept and hash the admin-supplied password
- `backend-go/services/auth-service/internal/usecase/list_sessions.go` (new) — cross-user, tenant-scoped session listing
- `backend-go/services/auth-service/internal/usecase/update_user.go` (new) — partial update of email/name/role
- `backend-go/services/auth-service/internal/usecase/ports.go` — `UserRepository.UpdateUser`, `SessionRepository.ListForTenant`
- `backend-go/services/auth-service/internal/adapter/postgres/user_repository.go`, `session_repository.go` — implement the new queries
- `backend-go/services/auth-service/internal/adapter/grpc/server.go` — wire `ListSessions`/`UpdateUser` RPC handlers
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_admin_routes.go` — `handleCreateUser` request body gains `password`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/admin_routes.go` — `handleListAllSessions` calls `ListSessions` when `user_id` is absent instead of 400ing; new `handleUpdateUser` replaces the role-only `PATCH`
- `backend-go/services/auth-service/internal/usecase/create_user_test.go`, `list_sessions_test.go`, `update_user_test.go`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/admin_routes_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

- **The password field is the spec's literal, already-cited fix.** BUG-AUTH-04's own "Spec summary" states the contract directly: "`POST /admin/api/users` takes `{email, name, role, password}` and creates a working account" (BUG-AUTH-04 line 11). `create_user.go`'s own doc comment already names the gap: "there is no invite/reset-link flow implemented in this scaffold to hand a chosen password to the new user" (cited at BUG-AUTH-04 line 30) — the fix the code comment itself points at is letting the admin hand over a chosen password, not building a new invite-link subsystem `auth-service.md` §3's RPC surface never mentions (`auth-service.md:96-103` lists only `CreateUser, GetUser, ListUsers, UpdateUser, DeactivateUser, ReactivateUser` — no `InviteUser`/`CompletePasswordReset`/anything token-based). Inventing an invite-token flow here would be an unwarranted architecture addition the TDD gives no basis for; adding the `password` field the spec already documents is the disciplined fix.
- **`UpdateUser` is already named in the TDD, just unimplemented.** `auth-service.md:98` explicitly lists `UpdateUser (email, role, profile fields that live here vs. delegate to `tenant-service` for the rest)` as part of the "Admin — users" RPC group — the current codebase only has the narrower `UpdateUserRole`. This solution adds the RPC the TDD already specifies, rather than inventing a new one.
- **Cross-user session listing is a genuine scope addition beyond `auth-service.md` §3**, which lists only `ListSessionsForUser` (`auth-service.md:105-108`) — flagged explicitly here the same way SOL-009 flagged `GetAdminStats` as a scope addition beyond `git-gateway-service.md`. `ListSessionsForUser` is kept as-is (a user or admin looking at one specific user's sessions is still a real, narrower operation); `ListSessions` is added alongside it for the admin dashboard's "active sessions across all users" view the spec's `GET /admin/api/sessions` documents (BUG-AUTH-04 spec summary line 13).
- **Tenant scoping, not global scoping, for cross-user listing** — per `07-security-architecture.md`'s multi-tenancy isolation layer 2 ("Application-layer `tenant_id` filtering on every query (primary)", `07-security-architecture.md:61`), `ListSessions` is scoped to the calling admin's own `tenant_id` (resolved from their validated identity, same as every other admin-console usecase's `requireAdminActor` check), never a caller-supplied tenant. An admin cannot list another tenant's sessions by construction.

## Design — proto

```protobuf
message CreateUserRequest {
  string email = 1;
  string name = 2;
  string tenant_id = 3;
  Role role = 4;
  // password: the admin-chosen initial credential, per this RPC's spec
  // contract (docs/logic/auth/BL-AUTH-04-admin-user-crud.md). Communicated
  // to the new user out-of-band (email/Slack/etc.) by the admin — this
  // service has no email-sending capability and none is added by this
  // change. Contrast with Bootstrap.EnsureAdmin (bootstrap.go:52-100),
  // which generates+prints its own password because no admin actor exists
  // yet to supply one for the very first account.
  string password = 5;
}

// ListSessions is the cross-user, tenant-scoped admin session-dashboard
// RPC — distinct from ListSessionsForUser (single-user scope, kept as-is).
// Added per SOL-AUTH-04 as a scope addition beyond this service's original
// RPC surface (auth-service.md §3 only lists ListSessionsForUser).
rpc ListSessions(ListSessionsRequest) returns (ListSessionsResponse);

message ListSessionsRequest {
  string tenant_id = 1;  // ignored if caller-supplied — resolved from the
                          // admin's own validated identity server-side, see
                          // Design rationale
  string page_token = 2;
  int32  page_size  = 3;
}
message ListSessionsResponse {
  repeated SessionWithUser sessions = 1;
  string next_page_token = 2;
}
// SessionWithUser avoids an N+1 user lookup per session row in the admin
// dashboard — email is denormalized into the response via a JOIN, not a
// second round trip per row.
message SessionWithUser {
  Session session = 1;
  string user_email = 2;
}

// UpdateUser — the RPC auth-service.md:98 already names but the current
// codebase never implemented (only the narrower UpdateUserRole exists).
// Wrapper types distinguish "field omitted" from "field explicitly set to
// empty string" for a true partial update.
rpc UpdateUser(UpdateUserRequest) returns (UpdateUserResponse);

message UpdateUserRequest {
  string user_id = 1;
  google.protobuf.StringValue email = 2;
  google.protobuf.StringValue name  = 3;
  optional Role role = 4; // proto3 `optional` scalar — present/absent distinguishable
}
message UpdateUserResponse {
  User user = 1;
}
```

## Design — `usecase/create_user.go`

```go
type CreateUserInput struct {
    Email, Name, TenantID, Password string
    Role                             domain.Role
}

func (uc *CreateUser) Execute(ctx context.Context, in CreateUserInput) (domain.User, error) {
    if _, err := requireAdminActor(ctx, uc.users, uc.opa); err != nil {
        return domain.User{}, err
    }
    if len(in.Password) < 8 { // mirrors SOL-AUTH-01's login-side format floor
        return domain.User{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_WEAK_PASSWORD", "password must be at least 8 characters", nil)
    }
    hash, err := uc.hasher.Hash(in.Password) // was: random 24-char generation, discarded — create_user.go's prior gap
    if err != nil {
        return domain.User{}, apperrors.New(apperrors.KindInternal, "AUTH_HASH_FAILED", "failed to hash password", err)
    }
    // ... unchanged: NewUser construction, users.CreateUser(ctx, user, hash), audit append "user.created"
}
```

## Design — `usecase/list_sessions.go` (new)

```go
type ListSessions struct {
    users    UserRepository
    sessions SessionRepository
    opa      OPAClient
}

func (uc *ListSessions) Execute(ctx context.Context, in ListSessionsInput) (ListSessionsOutput, error) {
    actor, err := requireAdminActor(ctx, uc.users, uc.opa)
    if err != nil {
        return ListSessionsOutput{}, err
    }
    // actor.TenantID, never in.TenantID — see Design rationale
    rows, next, err := uc.sessions.ListForTenant(ctx, actor.TenantID, in.PageToken, in.PageSize)
    if err != nil {
        return ListSessionsOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_LIST_SESSIONS_FAILED", "failed to list sessions", err)
    }
    return ListSessionsOutput{Sessions: rows, NextPageToken: next}, nil
}
```

`SessionRepository.ListForTenant` (new method) joins `auth.sessions` with
`auth.users` for the denormalized email, depends on
[SOL-AUTH-02](./SOL-AUTH-02-session-renew-expire-lifecycle.md)'s
`last_seen_at`/`ip` columns for those fields to be non-null in the response
— sequencing note, not a hard blocker (columns default `NULL` until
SOL-AUTH-02 lands):

```go
func (r *Repository) ListForTenant(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.SessionWithUser, string, error) {
    rows, err := r.pool.Query(ctx, `
        SELECT s.token_hash, s.user_id, s.tenant_id, s.created_at, s.expires_at,
               s.revoked_at, s.last_seen_at, s.ip, s.user_agent, u.email
        FROM auth.sessions s
        JOIN auth.users u ON u.id = s.user_id
        WHERE s.tenant_id = $1 AND s.token_hash > $2
        ORDER BY s.token_hash
        LIMIT $3
    `, tenantID, pageToken, pageSize)
    // ... scan loop, same pagination shape as ListUsers (user_repository.go:53-85)
}
```

## Design — `usecase/update_user.go` (new)

```go
type UpdateUserInput struct {
    UserID         string
    Email, Name    *string // nil = leave unchanged
    Role           *domain.Role
}

func (uc *UpdateUser) Execute(ctx context.Context, in UpdateUserInput) (domain.User, error) {
    if _, err := requireAdminActor(ctx, uc.users, uc.opa); err != nil {
        return domain.User{}, err
    }
    if in.Email != nil && !strings.Contains(*in.Email, "@") {
        return domain.User{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_INVALID_EMAIL", "email must contain '@'", nil)
    }
    if in.Role != nil && !in.Role.Valid() {
        return domain.User{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_INVALID_ROLE", "invalid role", nil)
    }
    user, err := uc.users.UpdateUser(ctx, in.UserID, in.Email, in.Name, in.Role)
    if err != nil {
        return domain.User{}, apperrors.New(apperrors.KindInternal, "AUTH_UPDATE_USER_FAILED", "failed to update user", err)
    }
    uc.appendAuditBestEffort(ctx, user, "user.updated") // distinct from user.role_updated (update_user_role.go:43) when role also changes — see SOL-AUTH-05 for target_type/metadata carrying the before/after diff
    return user, nil
}
```

`UserRepository.UpdateUser` (new, `postgres/user_repository.go`):

```go
func (r *Repository) UpdateUser(ctx context.Context, userID string, email, name *string, role *domain.Role) (domain.User, error) {
    var roleStr *string
    if role != nil {
        s := string(*role)
        roleStr = &s
    }
    row := r.pool.QueryRow(ctx, `
        UPDATE auth.users
        SET email = COALESCE($2, email), name = COALESCE($3, name), role = COALESCE($4, role)
        WHERE id = $1
        RETURNING id, tenant_id, email, name, role, is_active, created_at
    `, userID, email, name, roleStr)
    // ... scan, same ErrUserNotFound-on-pgx.ErrNoRows pattern as UpdateUserRole (user_repository.go:87-105)
}
```

`UpdateUserRole` (existing, `user_repository.go:87-105`) is kept as-is — a
narrower, still-valid single-field update some caller may still prefer; not
deprecated by this change.

## Design — wiring (`api-gateway` REST)

```go
// auth_admin_routes.go
type createUserRequestBody struct {
    Email, Name, Role, Password string // Password added
}
// handleCreateUser (auth_admin_routes.go:48-71) passes body.Password through
// to CreateUserRequest.Password unchanged otherwise.
```

```go
// admin_routes.go — handleListAllSessions (admin_routes.go:86-104) rewritten:
// no user_id -> ListSessions (new, tenant-scoped); user_id present ->
// ListSessionsForUser unchanged, preserving the existing narrower contract.
func handleListAllSessions(client authv1.AuthServiceClient) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        identity, _ := identityFromContext(r.Context())
        ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
        if userID := r.URL.Query().Get("user_id"); userID != "" {
            resp, err := client.ListSessionsForUser(ctx, &authv1.ListSessionsForUserRequest{UserId: userID})
            if err != nil { writeGRPCError(w, err); return }
            writeJSON(w, http.StatusOK, resp)
            return
        }
        resp, err := client.ListSessions(ctx, &authv1.ListSessionsRequest{
            PageToken: r.URL.Query().Get("page_token"),
        })
        if err != nil { writeGRPCError(w, err); return }
        writeJSON(w, http.StatusOK, resp)
    }
}
```

```go
// admin_routes.go — PATCH /admin/api/users/{id} switches from the
// role-only handleUpdateUserRole (admin_routes.go:32) to a new
// handleUpdateUser accepting {email?, name?, role?}. is_active is
// deliberately NOT added here — deactivation stays on DELETE
// /admin/api/users/:id (handleDeactivateUser, admin_routes.go:58-75),
// which BUG-AUTH-04 itself characterizes as already matching the spec's
// actual step-by-step flow, just not its literal PATCH example (BUG-AUTH-04
// line 33) — no need to duplicate that path onto PATCH.
type updateUserRequestBody struct {
    Email *string `json:"email"`
    Name  *string `json:"name"`
    Role  *string `json:"role"`
}
func handleUpdateUser(client authv1.AuthServiceClient) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        identity, _ := identityFromContext(r.Context())
        id := chi.URLParam(r, "id")
        var body updateUserRequestBody
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
            return
        }
        req := &authv1.UpdateUserRequest{UserId: id}
        if body.Email != nil { req.Email = wrapperspb.String(*body.Email) }
        if body.Name != nil { req.Name = wrapperspb.String(*body.Name) }
        if body.Role != nil { r := parseRole(*body.Role); req.Role = &r }
        ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
        resp, err := client.UpdateUser(ctx, req)
        if err != nil { writeGRPCError(w, err); return }
        writeJSON(w, http.StatusOK, resp.GetUser())
    }
}
```

`/v1/auth/users/{id}/role` (`auth_admin_routes.go:32`, backed by
`handleUpdateUserRole`) is left mounted as-is for backward compatibility —
`admin_routes.go`'s `PATCH /admin/api/users/{id}` mount point is the one
that switches handlers.

## Test plan

- `create_user_test.go`: `Password` too short → `AUTH_WEAK_PASSWORD`, no DB write; valid password → `PasswordHasher.Hash` called with the exact plaintext, stored hash verifiable via `PasswordHasher.Compare`; regression — the prior random-24-char-generate-and-discard code path is fully removed (assert no `crypto/rand` call remains reachable, or simply that the created user's password matches what was passed in via a round-trip `Compare` in the test).
- `list_sessions_test.go`: fake `SessionRepository.ListForTenant` scoped to `actor.TenantID` even when a caller passes a different `tenant_id` in the request (assert the fake receives the actor's tenant, not the request's); non-admin actor → `PermissionDenied`, `ListForTenant` never called.
- `update_user_test.go`: partial update (`Email` set, `Name`/`Role` nil) → repository called with `nil` for the untouched fields, existing values preserved; invalid email/role → `KindInvalidArgument` before any DB call.
- `postgres` integration tests: `UpdateUser`'s `COALESCE` semantics — updating only `email` leaves `name`/`role` unchanged in the DB; `ListForTenant` pagination and tenant-scoping (a session belonging to a different tenant never appears in the results).
- `admin_routes_test.go`: `POST /admin/api/users` with `password` in the body → `201`, and a subsequent `POST /auth/local` with that exact password succeeds (end-to-end regression guard for the bug's headline symptom: "that account is permanently unusable"); `GET /admin/api/sessions` with no `user_id` → `200` with cross-user results instead of the current `400`; `PATCH /admin/api/users/{id}` with `{"email": "new@x.com"}` → `200`, role unchanged.

## References

- `backend-go/services/auth-service/internal/usecase/create_user.go:13-82` — current random-password generation this replaces
- `backend-go/services/auth-service/internal/usecase/bootstrap.go:52-100` — the "generate+print" pattern this solution deliberately does NOT reuse for admin-created users (an admin actor exists to supply a real password directly, unlike first-run bootstrap)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/admin_routes.go:27-104` — `mountAdminRoutes`, current `handleListAllSessions`'s `400`-without-`user_id` behavior
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_admin_routes.go:28-130` — current `handleCreateUser`, `handleUpdateUserRole`, `parseRole`
- `backend-go/services/auth-service/internal/usecase/list_sessions_for_user.go:1-36` — the narrower RPC/usecase kept as-is
- `backend-go/services/auth-service/internal/adapter/postgres/user_repository.go:1-156` — current `UpdateUserRole`, `ListUsers` pagination pattern this solution's `UpdateUser`/`ListForTenant` follow
- `backend-go/proto/gen/go/orca/auth/v1/auth.pb.go:635-643` — current `CreateUserRequest` (no password field, confirming the gap)
- `backend-go/proto/gen/go/orca/auth/v1/auth_grpc.pb.go:293-320` — current `AuthServiceServer` RPC list (no `ListSessions`/`UpdateUser`, confirming both are scope additions)
- `specs/backend-go/tdd/services/auth-service.md:96-103` (§3 "Admin — users" RPC group, `UpdateUser` already named), `:105-108` (§3 "Admin — sessions", `ListSessionsForUser` only)
- `specs/backend-go/tdd/architecture/07-security-architecture.md:54-66` (multi-tenancy isolation layer 2, the tenant-scoping principle `ListSessions` follows)
- `specs/backend-go/bugs/missing-v1/solutions/SOL-009-files-channels.md` — precedent for flagging an RPC as a scope addition beyond its owning service's TDD
