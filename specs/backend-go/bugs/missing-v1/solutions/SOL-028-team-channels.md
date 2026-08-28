# SOL-028: Add `ListTeams`/`RemoveTeamMember` to `tenant-service`, wire all 5 `team.*` channels

**Resolves:** [BUG-028](../BUG-028-team-channels-not-implemented.md)
**Service:** `tenant-service` (2 new RPCs) + `api-gateway` (5 new `wscompat` channels)
**Affected files (proposed):**
- `backend-go/proto/orca/tenant/v1/tenant.proto`
- `backend-go/services/tenant-service/internal/usecase/ports.go`
- `backend-go/services/tenant-service/internal/usecase/list_teams.go` (new)
- `backend-go/services/tenant-service/internal/usecase/remove_team_member.go` (new)
- `backend-go/services/tenant-service/internal/adapter/postgres/team_repository.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_team_test.go` (new)
**Status:** ✅ Implemented — all 4 task(s) (TASK-176–179) DONE; see each task file's own Status/Verify section for evidence.

---

## The design already exists for 3 of 5 methods — this is a gap-closing task for the other 2

BUG-028 already found `team.create`/`team.addMember`/`team.listMembers` fully
backed (`CreateTeam`/`AddTeamMember`/`ListTeamMembers`,
`tenant.proto:15-17`, real usecases, real Postgres persistence against
`tenant.teams`/`tenant.team_members`) — those three need nothing but a
`wscompat` wrapper, covered below alongside the two genuinely missing RPCs
so this solution wires the whole namespace in one pass, matching
`tenant-service.md` §3's already-specified `TenantService` surface, which
lists `ListTeams` and `RemoveTeamMember` verbatim:

```protobuf
rpc ListTeams(ListTeamsRequest) returns (ListTeamsResponse);
rpc RemoveTeamMember(RemoveTeamMemberRequest) returns (google.protobuf.Empty);
```

(`tenant-service.md:82,84`). `tenant.proto`'s current "reduced subset"
(`tenant.proto:9-18`) simply hasn't caught up to that surface yet — same
situation SOL-001 found for `auth-service`'s admin RPCs. `RemoveTeamMember`
is additionally confirmed as a **known, tracked gap**, not an oversight:
`services/tenant-service/README.md:101` lists it explicitly under "Known
gaps / follow-ups."

---

## Design — Proto additions (`tenant.proto`)

```protobuf
// tenant-service.md §3 — list-all-teams-for-a-company, the missing half of
// team.* CRUD (create/get exist; list never did).
rpc ListTeams(ListTeamsRequest) returns (ListTeamsResponse);

// tenant-service.md §3 — the documented gap: services/tenant-service/README.md:101.
rpc RemoveTeamMember(RemoveTeamMemberRequest) returns (google.protobuf.Empty);

message ListTeamsRequest {
  // company_id intentionally omitted, same pattern AddTeamMemberRequest/
  // SetUserDepartmentRequest already use — the scoping company comes from
  // the validated request context (tenant.RequireTenantID), never a
  // client-supplied field, per tenant-service.md §9's "never inferred from
  // a nested resource ID" rule.
}

message ListTeamsResponse {
  repeated Team teams = 1;
}

message RemoveTeamMemberRequest {
  string team_id = 1;
  string user_id = 2;
}
```

Both additive — `buf breaking` passes per `08-inter-service-communication.md`'s
gRPC conventions.

---

## Design — `usecase/` layer

Follows `03-clean-architecture-guidelines.md`'s "interface lives with its
consumer" rule: extend the existing `TeamRepository` port
(`internal/usecase/ports.go:48-59`) rather than adding a parallel one.

```go
// internal/usecase/ports.go — TeamRepository, two new methods
type TeamRepository interface {
    Create(ctx context.Context, team domain.Team) (domain.Team, error)
    Get(ctx context.Context, companyID, id string) (domain.Team, bool, error)
    // ListByCompany backs ListTeams — every team row scoped to companyID,
    // same not-found-not-wrong-company posture as Get (tenant-service.md §9).
    ListByCompany(ctx context.Context, companyID string) ([]domain.Team, error)
    AddMember(ctx context.Context, member domain.TeamMember) error
    // RemoveMember deletes one (team_id, user_id) row — backs
    // RemoveTeamMember. Returns (found bool, err error) so the usecase can
    // distinguish "already removed" (idempotent no-op, matches DELETE
    // semantics elsewhere in this codebase) from a real failure.
    RemoveMember(ctx context.Context, teamID, userID string) (bool, error)
    ListMembers(ctx context.Context, teamID string) ([]domain.TeamMember, error)
    ListUserTeamLayers(ctx context.Context, companyID, userID string) ([]domain.TeamSettingsLayer, error)
}
```

```go
// internal/usecase/list_teams.go
type ListTeams struct {
    teams TeamRepository
}

func NewListTeams(teams TeamRepository) *ListTeams { return &ListTeams{teams: teams} }

func (uc *ListTeams) Execute(ctx context.Context) ([]domain.Team, error) {
    companyID, err := tenant.RequireTenantID(ctx)
    if err != nil {
        return nil, apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
    }
    teams, err := uc.teams.ListByCompany(ctx, companyID)
    if err != nil {
        return nil, apperrors.New(apperrors.KindInternal, "TENANT_LIST_TEAMS_FAILED", "failed to list teams", err)
    }
    return teams, nil
}
```

```go
// internal/usecase/remove_team_member.go — mirrors AddTeamMember.Execute's
// shape (internal/usecase/add_team_member.go:36-70) exactly, including its
// cache-invalidation obligation: tenant-service.md §8 requires "every team
// member for a Team/TeamMembership edit" invalidates that user's cached
// ResolvedProfile — a removal is exactly as invalidation-relevant as an add.
type RemoveTeamMember struct {
    teams        TeamRepository
    cache        ProfileCache
    invalidation CacheInvalidationPublisher
}

func NewRemoveTeamMember(teams TeamRepository, cache ProfileCache, invalidation CacheInvalidationPublisher) *RemoveTeamMember {
    return &RemoveTeamMember{teams: teams, cache: cache, invalidation: invalidation}
}

func (uc *RemoveTeamMember) Execute(ctx context.Context, teamID, userID string) error {
    companyID, err := tenant.RequireTenantID(ctx)
    if err != nil {
        return apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
    }
    if _, found, err := uc.teams.Get(ctx, companyID, teamID); err != nil {
        return apperrors.New(apperrors.KindInternal, "TENANT_TEAM_LOOKUP_FAILED", "failed to look up team", err)
    } else if !found {
        return apperrors.New(apperrors.KindNotFound, "TENANT_TEAM_NOT_FOUND", "team does not exist", nil)
    }
    if _, err := uc.teams.RemoveMember(ctx, teamID, userID); err != nil {
        return apperrors.New(apperrors.KindInternal, "TENANT_REMOVE_TEAM_MEMBER_FAILED", "failed to remove team member", err)
    }
    // Same invalidation obligation as AddTeamMember.Execute (§8) — the
    // removed member's team-layer contribution to ResolveProfile changes.
    if uc.cache != nil {
        uc.cache.Invalidate(ctx, userID)
    }
    if uc.invalidation != nil {
        _ = uc.invalidation.PublishProfileInvalidated(ctx, companyID, userID)
    }
    return nil
}
```

`internal/adapter/postgres/team_repository.go` additions follow
`AddMember`/`ListMembers`'s existing style (`team_repository.go:63-90`)
directly:

```go
func (r *TeamRepository) ListByCompany(ctx context.Context, companyID string) ([]domain.Team, error) {
    rows, err := r.pool.Query(ctx, `
        SELECT id, company_id, name, settings_json FROM tenant.teams WHERE company_id = $1
    `, companyID)
    // ... scan loop identical in shape to Get's single-row scan, unmarshalSettings per row
}

func (r *TeamRepository) RemoveMember(ctx context.Context, teamID, userID string) (bool, error) {
    tag, err := r.pool.Exec(ctx, `
        DELETE FROM tenant.team_members WHERE team_id = $1 AND user_id = $2
    `, teamID, userID)
    if err != nil {
        return false, fmt.Errorf("postgres: delete team member: %w", err)
    }
    return tag.RowsAffected() > 0, nil
}
```

---

## Design — `wscompat` wiring (`api-gateway`)

New `registerTeamChannels`, following `registerAnnotationChannels`'s exact
pattern (`channels.go:93-175`) — all 5 methods, 3 thin wrappers around
already-real RPCs plus the 2 new ones proposed above:

```go
func registerTeamChannels(r *Registry, client tenantv1.TenantServiceClient) {
    r.Register("team.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type createArgs struct {
            Name         string `json:"name"`
            SettingsJSON string `json:"settingsJson"`
        }
        in, err := decodeArg[createArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        resp, err := client.CreateTeam(ctx, &tenantv1.CreateTeamRequest{
            CompanyId: id.TenantID, Name: in.Name, SettingsJson: in.SettingsJSON,
        })
        if err != nil {
            return nil, err
        }
        return resp.GetTeam(), nil
    })

    r.Register("team.list", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        resp, err := client.ListTeams(ctx, &tenantv1.ListTeamsRequest{})
        if err != nil {
            return nil, err
        }
        return resp.GetTeams(), nil
    })

    r.Register("team.addMember", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type addMemberArgs struct {
            TeamID string `json:"teamId"`
            UserID string `json:"userId"`
            // Role has nowhere to go — AddTeamMemberRequest carries only
            // priority (tenant.proto:90-94), role defaults to 'member'
            // server-side (README "Known gaps", cited by BUG-028). Decoded
            // and silently dropped here rather than erroring, matching this
            // file's existing "best-effort, not verified against every
            // frontend call site" convention (channels.go:6-14).
            Role     string `json:"role"`
            Priority int32  `json:"priority"`
        }
        in, err := decodeArg[addMemberArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        if _, err := client.AddTeamMember(ctx, &tenantv1.AddTeamMemberRequest{
            TeamId: in.TeamID, UserId: in.UserID, Priority: in.Priority,
        }); err != nil {
            return nil, err
        }
        return map[string]bool{"ok": true}, nil
    })

    r.Register("team.removeMember", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type removeMemberArgs struct {
            TeamID string `json:"teamId"`
            UserID string `json:"userId"`
        }
        in, err := decodeArg[removeMemberArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        if _, err := client.RemoveTeamMember(ctx, &tenantv1.RemoveTeamMemberRequest{
            TeamId: in.TeamID, UserId: in.UserID,
        }); err != nil {
            return nil, err
        }
        return map[string]bool{"ok": true}, nil
    })

    r.Register("team.listMembers", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type listMembersArgs struct {
            TeamID string `json:"teamId"`
        }
        in, err := decodeArg[listMembersArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        resp, err := client.ListTeamMembers(ctx, &tenantv1.ListTeamMembersRequest{TeamId: in.TeamID})
        if err != nil {
            return nil, err
        }
        return resp.GetMembers(), nil
    })
}
```

`RegisterRealChannels` (`channels.go:64-82`) grows one line:
`registerTeamChannels(r, tenantClient)`, and `main.go`'s composition root
dials `tenant-service`'s gRPC client (not currently wired into
`api-gateway`'s `main.go` — flag as an additional dependency this solution
introduces, same class of change SOL-001 flagged for `admin_routes.go`'s
new `AuthClient` usage).

`AttachIdentity` is used on every handler above (not just
`devServer.*`/`fleet.*`) because `tenant-service`'s RPCs bind `tenant_id`
from gRPC metadata for every mutating/scoped call per
`tenant-service.md`'s "every request carries `tenant_id` explicitly...
never inferred from a nested resource ID" rule (§3) — this is the correct
default for this client, unlike the exception `channels.go`'s doc comment
carves out for `infrafleetv1`.

---

## Test plan

- `services/tenant-service/internal/usecase/list_teams_test.go` — fake
  `TeamRepository` returns N teams for the caller's company; a team
  belonging to a different `companyID` is never returned (adversarial
  cross-company case, per `tenant-service.md` §9's mandate that this
  service's test suite treat that as first-class).
- `services/tenant-service/internal/usecase/remove_team_member_test.go` —
  removing an existing member invalidates the cache for that `userID` and
  best-effort-publishes; removing a non-member is a no-op, not an error
  (idempotent DELETE semantics); removing from a team in another company
  returns `TENANT_TEAM_NOT_FOUND`.
- `services/tenant-service/internal/adapter/postgres/team_repository_test.go`
  (testcontainers-go, per `03-clean-architecture-guidelines.md`'s layering)
  — `ListByCompany` round-trips multiple teams; `RemoveMember` deletes
  exactly one row and reports `found=false` on a second call.
- `services/api-gateway/internal/adapter/wscompat/channels_team_test.go` —
  one test per `team.*` channel against a fake `TenantServiceClient`,
  mirroring `channels_test.go`'s existing shape for `annotation.*`; assert
  `team.addMember`'s dropped `role` field doesn't error.

## References

- `specs/backend-go/tdd/services/tenant-service.md:82,84` — `ListTeams`/`RemoveTeamMember` already specified in the target RPC surface
- `specs/backend-go/tdd/services/tenant-service.md:264-274` (§8) — cache-invalidation-correctness requirement covering team-membership edits
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` — usecase/port layering
- `backend-go/proto/orca/tenant/v1/tenant.proto:9-18,73-109` — current `TenantService` surface, `Team`/`TeamMember` messages
- `backend-go/services/tenant-service/internal/usecase/ports.go:48-59` — `TeamRepository` interface to extend
- `backend-go/services/tenant-service/internal/usecase/add_team_member.go` — invalidation pattern to mirror in `RemoveTeamMember`
- `backend-go/services/tenant-service/internal/adapter/postgres/team_repository.go` — existing `AddMember`/`ListMembers` implementations to extend
- `backend-go/services/tenant-service/README.md:99-101` — documented gap: no `RemoveTeamMember`/`UpdateTeam` RPC yet
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:64-82,93-175` — `RegisterRealChannels`, `registerAnnotationChannels` pattern to mirror
- [BUG-028](../BUG-028-team-channels-not-implemented.md) — full findings this solution builds on
