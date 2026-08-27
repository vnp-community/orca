# SOL-019: Implement `profile.*` against `tenant-service.md`'s already-specified API surface

**Resolves:** [BUG-019](../BUG-019-profile-channels-not-implemented.md)
**Service:** `tenant-service` (new RPCs) + `api-gateway` (`wscompat` wiring)
**Affected files (proposed):**
- `backend-go/proto/orca/tenant/v1/tenant.proto`
- `backend-go/services/tenant-service/internal/domain/department.go`, `company.go`
- `backend-go/services/tenant-service/internal/usecase/ports.go` (extend 3 ports)
- `backend-go/services/tenant-service/internal/usecase/get_user_profile.go` (new)
- `backend-go/services/tenant-service/internal/usecase/list_departments.go` (new)
- `backend-go/services/tenant-service/internal/usecase/update_company.go` (new)
- `backend-go/services/tenant-service/internal/usecase/update_department.go` (new)
- `backend-go/services/tenant-service/internal/usecase/update_user_profile.go` (new)
- `backend-go/services/tenant-service/internal/adapter/postgres/*.go` (repository methods backing the above)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (new `registerProfileChannels`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## The design already exists — this is a gap-closing task, not a new design

`tenant-service.md` §3's own API-surface sketch (lines 60–86) already
specifies **all five** of the RPCs BUG-019 found missing from the actual
`tenant.proto`:

| BUG-019 missing channel | Already-specified target RPC (`tenant-service.md`) |
|---|---|
| `profile.getUserProfile` | `GetUserProfile(GetUserProfileRequest) returns (UserProfile)` — line 72 |
| `profile.listDepts` | `ListDepartments(ListDepartmentsRequest) returns (ListDepartmentsResponse)` — line 68 |
| `profile.updateCompany` | `UpdateCompany(UpdateCompanyRequest) returns (Company)` — line 62 |
| `profile.updateDept` | `UpdateDepartment(UpdateDepartmentRequest) returns (Department)` — line 67 |
| `profile.updateUser` | `UpdateUserProfile(UpdateUserProfileRequest) returns (UserProfile)` — line 73 |

The **actual** shipped `tenant.proto` only implements a subset of
`tenant-service.md`'s sketch (`CreateCompany, ValidateTenant,
CreateDepartment, SetUserDepartment, GetResolvedProfile, CreateTeam,
AddTeamMember, ListTeamMembers`) — this solution closes exactly that
delta, RPC-for-RPC, rather than inventing a different shape. Two pieces of
existing code independently corroborate the gap is known and intentional,
not an oversight:

- `usecase/ports.go`'s `UserProfileRepository` doc comment: *"there's no
  `UpdateUserProfile` RPC yet to set Settings directly"* (used today only
  internally by `SetUserDepartment`'s `Upsert` call).
- `domain/department.go`'s doc comment: *"Deliberately no
  `parent_department_id` field: ... isn't exercised by any RPC in
  `tenant.proto`'s current surface (no `ListDepartments`/hierarchy RPC), so
  it isn't modeled here — see this service's README 'Known gaps'."*

`profile.getResolved` (the 6th channel) needs no service-side work — its
RPC (`GetResolvedProfile`) and REST wiring (`GET /v1/tenants/profile`) both
already exist; it only needs the `wscompat` registration below.

---

## Design — Proto additions (`tenant.proto`)

```protobuf
rpc GetUserProfile(GetUserProfileRequest) returns (UserProfile);
rpc ListDepartments(ListDepartmentsRequest) returns (ListDepartmentsResponse);
rpc UpdateCompany(UpdateCompanyRequest) returns (Company);
rpc UpdateDepartment(UpdateDepartmentRequest) returns (Department);
rpc UpdateUserProfile(UpdateUserProfileRequest) returns (UserProfile);

message UserProfile {
  string user_id = 1;
  string company_id = 2;
  string department_id = 3;  // empty = company-only inheritance
  string settings_json = 4;
}

message GetUserProfileRequest {
  string user_id = 1;
}

message ListDepartmentsRequest {
  string company_id = 1;
}

message ListDepartmentsResponse {
  repeated Department departments = 1;
}

message UpdateCompanyRequest {
  string id = 1;
  string name = 2;          // empty = no change
  string settings_json = 3; // empty = no change
}

message UpdateDepartmentRequest {
  string id = 1;
  string name = 2;          // empty = no change
  string settings_json = 3; // empty = no change
}

message UpdateUserProfileRequest {
  string user_id = 1;
  string department_id = 2; // empty string is ambiguous with "no change" vs
                             // "clear department" — see Design note below
  string settings_json = 3; // empty = no change
}
```

Additive only — no `buf breaking` risk (`08-inter-service-communication.md`'s
gRPC conventions).

**`UpdateUserProfileRequest.department_id` ambiguity**: `UserProfile`'s own
doc comment already treats an empty `DepartmentID` as a *meaningful* state
("company-only inheritance"), unlike every other `Update*Request` in this
codebase where empty means "no change" (`UpdateProjectRequest`'s convention,
mirrored by `UpdateCompanyRequest`/`UpdateDepartmentRequest` above). Reusing
that convention here would make "clear the department" inexpressible.
Resolve with a field mask (`google.protobuf.FieldMask update_mask = 4` or a
`bool clear_department = 4` flag) rather than silently reusing the
ambiguous empty-string idiom — flagged explicitly rather than picked
silently, since it's a real behavioral fork for the reviewer to confirm.

---

## Design — `domain/` changes

`Department` currently has no `ParentDepartmentID` field (deliberately
omitted per its doc comment, quoted above, precisely because no RPC
exercised it). `ListDepartments` is a flat list per `tenant-service.md`
§3/§5 (no hierarchy RPC is proposed here either — the TDD's `departments`
table has `parent_department_id` for the tree, but no RPC surfaces it yet
beyond storage) — so this solution's `ListDepartments` returns a flat,
company-scoped list and does **not** add `ParentDepartmentID` to the domain
struct; that stays out of scope, consistent with the existing doc comment's
reasoning, until an RPC actually needs it.

`Company`/`Department`/`UserProfile` need no new invariants for `Update*` —
the same constructors (`NewCompany`/`NewDepartment`/`NewUserProfile`)
validate the patched result before persisting, matching `UpdateProject`'s
pattern in `project-service`.

---

## Design — `usecase/` layer

Three of the five need a new repository method; one (`GetUserProfile`)
needs none — `UserProfileRepository.Get(ctx, companyID, userID)` already
exists and already does exactly this lookup (added for `SetUserDepartment`'s
internal use, never exposed as its own RPC).

```go
// internal/usecase/get_user_profile.go — thin: the port already exists.
func (uc *ProfileUseCase) GetUserProfile(ctx context.Context, companyID, userID string) (domain.UserProfile, error) {
    profile, found, err := uc.userProfiles.Get(ctx, companyID, userID)
    if err != nil {
        return domain.UserProfile{}, err
    }
    if !found {
        return domain.UserProfile{}, domain.ErrUserProfileNotFound
    }
    return profile, nil
}
```

```go
// internal/usecase/ports.go additions
type CompanyRepository interface {
    // ... existing Create/Get/Exists ...
    Update(ctx context.Context, id string, patch domain.CompanySettingsPatch) (domain.Company, error)
}

type DepartmentRepository interface {
    // ... existing Create/Get ...
    List(ctx context.Context, companyID string) ([]domain.Department, error)
    Update(ctx context.Context, companyID, id string, patch domain.DepartmentSettingsPatch) (domain.Department, error)
}
```

`UpdateUserProfile` reuses `UserProfileRepository.Upsert` — already built,
already the exact primitive `SetUserDepartment` uses internally; this RPC
is the "expose it directly" case the port's own doc comment flagged as
missing.

**Cache invalidation is the correctness-critical part of all three
`Update*` usecases**, per `tenant-service.md` §8's explicit requirement
("any mutation ... MUST invalidate every cached `ResolvedProfile` for
transitively affected users ... before the write's RPC returns success"):

```go
// internal/usecase/update_department.go
func (uc *ProfileUseCase) UpdateDepartment(ctx context.Context, companyID, id string, patch domain.DepartmentSettingsPatch) (domain.Department, error) {
    dept, err := uc.departments.Update(ctx, companyID, id, patch)
    if err != nil {
        return domain.Department{}, err
    }
    // Scope: every user in this department (tenant-service.md §8's
    // per-mutation invalidation-scope table) — not just the caller.
    userIDs, err := uc.userProfiles.ListUserIDsByDepartment(ctx, companyID, id)
    if err != nil {
        return domain.Department{}, err
    }
    for _, uid := range userIDs {
        uc.cache.Invalidate(ctx, uid)
        if uc.invalidationPublisher != nil {
            _ = uc.invalidationPublisher.PublishProfileInvalidated(ctx, companyID, uid)
        }
    }
    return dept, nil
}
```

This needs one more repository method not on the port today —
`UserProfileRepository.ListUserIDsByDepartment` (and the company-wide
equivalent, `ListUserIDsByCompany`, for `UpdateCompany`'s wider
invalidation scope) — both cheap indexed reads against
`idx_user_profiles_department`/`idx_user_profiles_company`
(`tenant-service.md` §5). `UpdateUserProfile`'s own invalidation scope is
just the one `user_id` being updated — no extra lookup needed.

---

## Design — `wscompat` wiring (all 6 channels)

```go
// ── profile.* ──────────────────────────────────────────────────────────
// profile.getResolved: RPC + REST already exist (tenant_routes.go's
// handleGetResolvedProfile); this is the wiring-only case. The other 5
// call the new RPCs proposed above.

func registerProfileChannels(r *Registry, client tenantv1.TenantServiceClient) {
    r.Register("profile.getResolved", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.GetResolvedProfile(rpcCtx, &tenantv1.GetResolvedProfileRequest{UserId: id.UserID})
        if err != nil {
            return nil, err
        }
        return resp, nil
    })

    r.Register("profile.getUserProfile", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type getArgs struct {
            UserID string `json:"userId"`
        }
        in, err := decodeArg[getArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.GetUserProfile(rpcCtx, &tenantv1.GetUserProfileRequest{UserId: in.UserID})
        if err != nil {
            return nil, err
        }
        return resp, nil
    })

    r.Register("profile.listDepts", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type listArgs struct {
            CompanyID string `json:"companyId"`
        }
        in, err := decodeArg[listArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.ListDepartments(rpcCtx, &tenantv1.ListDepartmentsRequest{CompanyId: in.CompanyID})
        if err != nil {
            return nil, err
        }
        return resp.GetDepartments(), nil
    })

    r.Register("profile.updateCompany", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type updateArgs struct {
            ID           string `json:"id"`
            Name         string `json:"name"`
            SettingsJSON string `json:"settingsJson"`
        }
        in, err := decodeArg[updateArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.UpdateCompany(rpcCtx, &tenantv1.UpdateCompanyRequest{
            Id: in.ID, Name: in.Name, SettingsJson: in.SettingsJSON,
        })
        if err != nil {
            return nil, err
        }
        return resp, nil
    })

    r.Register("profile.updateDept", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type updateArgs struct {
            ID           string `json:"id"`
            Name         string `json:"name"`
            SettingsJSON string `json:"settingsJson"`
        }
        in, err := decodeArg[updateArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.UpdateDepartment(rpcCtx, &tenantv1.UpdateDepartmentRequest{
            Id: in.ID, Name: in.Name, SettingsJson: in.SettingsJSON,
        })
        if err != nil {
            return nil, err
        }
        return resp, nil
    })

    r.Register("profile.updateUser", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type updateArgs struct {
            UserID       string `json:"userId"`
            DepartmentID string `json:"departmentId"`
            SettingsJSON string `json:"settingsJson"`
        }
        in, err := decodeArg[updateArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.UpdateUserProfile(rpcCtx, &tenantv1.UpdateUserProfileRequest{
            UserId: in.UserID, DepartmentId: in.DepartmentID, SettingsJson: in.SettingsJSON,
        })
        if err != nil {
            return nil, err
        }
        return resp, nil
    })
}
```

`RegisterRealChannels` (channels.go top) gains a `tenantClient
tenantv1.TenantServiceClient` parameter and a `registerProfileChannels(r,
tenantClient)` call — `main.go`'s composition root already dials a
`TenantServiceClient` for `tenant_routes.go`, so this is passing an
already-constructed client through, not adding a new dial.

---

## Test plan

- `services/tenant-service/internal/usecase/get_user_profile_test.go` —
  found/not-found cases against an in-memory `UserProfileRepository` fake.
- `list_departments_test.go` — company-scoped list; a department belonging
  to a different `company_id` must never appear (tenant-service.md §9's
  cross-company isolation rule).
- `update_company_test.go` / `update_department_test.go` /
  `update_user_profile_test.go` — patch applies; **and** a fake `ProfileCache`
  records `Invalidate` was called for exactly the right scope (one user /
  every department member / every company member) — this is the
  production-readiness-checklist-gated invariant from §8, not optional
  coverage.
- `services/tenant-service/internal/adapter/postgres/*_test.go` —
  `testcontainers-go` integration tests for the three new repository
  methods plus the two new `ListUserIDsBy*` lookups.
- `services/api-gateway/internal/adapter/wscompat/channels_test.go` — one
  test per channel (6 total), fake `TenantServiceClient`, asserting
  request-field mapping and response passthrough — mirrors
  `registerDevServerChannels`'s existing test shape.

## References

- `specs/backend-go/tdd/services/tenant-service.md:60-86` — `TenantService`'s already-specified target RPC list (the design this solution implements)
- `specs/backend-go/tdd/services/tenant-service.md:203-228` (§6/§8) — `ProfileCache` port, invalidation-scope requirement
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` — usecase/repository/port layering
- `backend-go/proto/orca/tenant/v1/tenant.proto` — current (reduced) RPC surface
- `backend-go/services/tenant-service/internal/usecase/ports.go` — `UserProfileRepository`'s "no UpdateUserProfile RPC yet" doc comment; `ProfileCache`/`CacheInvalidationPublisher` ports to reuse
- `backend-go/services/tenant-service/internal/domain/department.go` — "Deliberately no parent_department_id field ... no ListDepartments RPC" doc comment
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:390-433` — `registerDevServerChannels`, the wiring pattern mirrored above
- `backend-go/services/api-gateway/internal/adapter/httpgateway/tenant_routes.go:154-177` — `handleGetResolvedProfile`, the already-working REST path `profile.getResolved` reuses
- `specs/backend-go/bugs/missing-v1/BUG-019-profile-channels-not-implemented.md` — the bug this resolves
