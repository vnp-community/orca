# SOL-020: Wire `project.*`'s 4 working RPCs; implement `project-service.md`'s already-specified membership RPCs for the other 3

**Resolves:** [BUG-020](../BUG-020-project-channels-not-implemented.md)
**Service:** `project-service` (3 new RPCs) + `api-gateway` (`wscompat` wiring for all 7)
**Affected files (proposed):**
- `backend-go/proto/orca/project/v1/project.proto`
- `backend-go/services/project-service/internal/domain/membership.go`
- `backend-go/services/project-service/internal/usecase/ports.go` (extend `ProjectRepository`)
- `backend-go/services/project-service/internal/usecase/list_members.go` (new)
- `backend-go/services/project-service/internal/usecase/remove_member.go` (new)
- `backend-go/services/project-service/internal/usecase/update_member_role.go` (new)
- `backend-go/services/project-service/internal/adapter/postgres/*.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (new `registerProjectChannels`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## Two independent pieces: 4 pure-wiring channels, 3 that need `project-service.md`'s already-specified RPCs

### The 4 wiring-only channels

BUG-020 confirmed `create`/`get`/`list`/`update` are fully built end-to-end
(usecase, REST proxy) and only missing a `wscompat` registration — same
shape as `devServer.list`/`devServer.add`'s existing pattern. No proto,
usecase, or repository change needed for these 4.

### The 3 member-management channels — the design already exists

`project-service.md` §3's API surface (lines 82–86) already specifies
`ListMembers`/`RemoveMember`/`UpdateMemberRole` alongside the one member
RPC (`AddMember`) that actually shipped:

```protobuf
rpc AddMember(AddMemberRequest) returns (ProjectMember);         // shipped
rpc RemoveMember(RemoveMemberRequest) returns (google.protobuf.Empty);   // missing
rpc UpdateMemberRole(UpdateMemberRoleRequest) returns (ProjectMember);   // missing
rpc ListMembers(ListMembersRequest) returns (ListMembersResponse);       // missing
```

The domain model to build these against is `project-service.md` §4's
`ProjectMember` entity — the Go equivalent of TS's `orca_v5_project_members`
table this task's brief asked to locate — and its **stated invariant**:

> Invariant: at least one `owner` must remain — `RemoveMember`/
> `UpdateMemberRole` reject an operation that would leave a project
> ownerless (closes a gap TS itself doesn't guard).

The actual shipped code confirms both the gap and that this invariant is a
documented, deliberate follow-up, not a surprise: `domain/membership.go`'s
`ProjectRole` doc comment reads *"Deliberately mirrors the proto enum's two
values (member/owner) — the fuller owner/member/viewer model from
project-service.md §4 is a documented follow-up once
ListMembers/UpdateMemberRole/the '≥1 owner' invariant are ported."* This
solution is that follow-up.

**Scope note on the role model**: `project-service.md` §4/§9 specifies
three roles (`owner`/`member`/`viewer`, with `viewer` getting read-only
access per §9's "`GetProject`/`ListMembers`/... require any membership
including `viewer`"). The shipped `domain.ProjectRole` only has
`member`/`owner`. Porting `viewer` is a larger change (touches the OPA
policy's `caller_project_role` input, every read-path authorization check)
than these 3 RPCs strictly require — this solution adds the 3 RPCs against
the **existing 2-role model** and flags 3-role support as a separate,
follow-on scope item rather than silently bundling an authorization-model
change into a wiring fix.

---

## Design — Proto additions (`project.proto`)

```protobuf
message Member {
  string user_id = 1;
  ProjectRole role = 2;
}

message ListMembersRequest {
  string project_id = 1;
}

message ListMembersResponse {
  repeated Member members = 1;
}

message RemoveMemberRequest {
  string project_id = 1;
  string user_id = 2;
}

// Matches this proto's own established convention of an explicit empty
// Response wrapper (AddMemberResponse{}, DeleteProjectResponse{}) rather
// than project-service.md's abbreviated google.protobuf.Empty shorthand —
// keeps every RPC uniform under `resp.Get...()`-style codegen.
message RemoveMemberResponse {}

message UpdateMemberRoleRequest {
  string project_id = 1;
  string user_id = 2;
  ProjectRole role = 3;
}

message UpdateMemberRoleResponse {
  Member member = 1;
}
```

Additive only — no `buf breaking` risk.

---

## Design — `usecase/` layer

New `ProjectRepository` methods (`ports.go`):

```go
type ProjectRepository interface {
    // ... existing Create/Get/List/AddMember/UpdateDevServerID/UpdateProject/DeleteProject/GetMembership ...

    // ListMembers returns every membership row for a project.
    ListMembers(ctx context.Context, tenantID, projectID string) ([]domain.ProjectMember, error)
    // RemoveMember deletes one membership row. Returns
    // domain.ErrMembershipNotFound if none exists.
    RemoveMember(ctx context.Context, tenantID, projectID, userID string) error
    // UpdateMemberRole changes one membership row's role.
    UpdateMemberRole(ctx context.Context, tenantID, projectID, userID string, role domain.ProjectRole) (domain.ProjectMember, error)
    // CountOwners is the read RemoveMember/UpdateMemberRole use to enforce
    // the "≥1 owner" invariant before mutating.
    CountOwners(ctx context.Context, tenantID, projectID string) (int, error)
}
```

The ownerless-guard, in `domain/membership.go` as a pure function (per
`03-clean-architecture-guidelines.md`'s "domain/ has zero I/O" rule — the
usecase does the count read, the domain function decides):

```go
// ErrProjectWouldBeOwnerless is returned by the ownerless guard below.
var ErrProjectWouldBeOwnerless = errors.New("domain: project must retain at least one owner")

// AssertNotLastOwnerRemoval enforces project-service.md §4's invariant:
// removing membership or demoting a role must never leave zero owners.
func AssertNotLastOwnerRemoval(currentOwnerCount int, targetIsCurrentlyOwner bool, targetRoleAfter ProjectRole) error {
    if targetIsCurrentlyOwner && targetRoleAfter != ProjectRoleOwner && currentOwnerCount <= 1 {
        return ErrProjectWouldBeOwnerless
    }
    return nil
}
```

```go
// internal/usecase/remove_member.go
func (uc *MembershipUseCase) RemoveMember(ctx context.Context, tenantID, projectID, actorID, targetUserID string) error {
    if err := uc.requireProjectAccess(ctx, tenantID, projectID, actorID, actionRemoveMember); err != nil {
        return err // owner or global admin only — project-service.md §9
    }
    target, err := uc.repo.GetMembership(ctx, projectID, targetUserID)
    if err != nil {
        return err
    }
    owners, err := uc.repo.CountOwners(ctx, tenantID, projectID)
    if err != nil {
        return err
    }
    if err := domain.AssertNotLastOwnerRemoval(owners, target.Role == domain.ProjectRoleOwner, ""); err != nil {
        return err // maps to FAILED_PRECONDITION at the gRPC boundary
    }
    return uc.repo.RemoveMember(ctx, tenantID, projectID, targetUserID)
}
```

`UpdateMemberRole` follows the identical shape, passing the *new* role into
`AssertNotLastOwnerRemoval`'s third argument instead of `""`. `ListMembers`
requires only membership (viewer-equivalent under the 2-role model, i.e.
`member` or `owner`), per `project-service.md` §9's "`ListMembers` ...
require[s] any membership."

---

## Design — `wscompat` wiring (all 7 channels)

```go
// ── project.* ──────────────────────────────────────────────────────────
// create/get/list/update: RPC + REST already exist, wiring-only.
// getMembers/removeMember/updateMemberRole: call the 3 new RPCs above.

func registerProjectChannels(r *Registry, client projectv1.ProjectServiceClient) {
    r.Register("project.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type createArgs struct {
            Name          string `json:"name"`
            Description   string `json:"description"`
            DevServerID   string `json:"devServerId"`
            DefaultBranch string `json:"defaultBranch"`
            Visibility    string `json:"visibility"`
        }
        in, err := decodeArg[createArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.CreateProject(rpcCtx, &projectv1.CreateProjectRequest{
            Name: in.Name, Description: in.Description, DevServerId: in.DevServerID,
            DefaultBranch: in.DefaultBranch, Visibility: in.Visibility,
        })
        if err != nil {
            return nil, err
        }
        return resp.GetProject(), nil
    })

    r.Register("project.get", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type getArgs struct {
            ID string `json:"id"`
        }
        in, err := decodeArg[getArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.GetProject(rpcCtx, &projectv1.GetProjectRequest{ProjectId: in.ID})
        if err != nil {
            return nil, err
        }
        return resp.GetProject(), nil
    })

    r.Register("project.list", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.ListProjects(rpcCtx, &projectv1.ListProjectsRequest{})
        if err != nil {
            return nil, err
        }
        return resp.GetProjects(), nil
    })

    r.Register("project.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type updateArgs struct {
            ID            string `json:"id"`
            Name          string `json:"name"`
            Description   string `json:"description"`
            DefaultBranch string `json:"defaultBranch"`
            Visibility    string `json:"visibility"`
        }
        in, err := decodeArg[updateArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.UpdateProject(rpcCtx, &projectv1.UpdateProjectRequest{
            ProjectId: in.ID, Name: in.Name, Description: in.Description,
            DefaultBranch: in.DefaultBranch, Visibility: in.Visibility,
        })
        if err != nil {
            return nil, err
        }
        return resp.GetProject(), nil
    })

    r.Register("project.getMembers", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type getArgs struct {
            ProjectID string `json:"projectId"`
        }
        in, err := decodeArg[getArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.ListMembers(rpcCtx, &projectv1.ListMembersRequest{ProjectId: in.ProjectID})
        if err != nil {
            return nil, err
        }
        return resp.GetMembers(), nil
    })

    r.Register("project.removeMember", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type removeArgs struct {
            ProjectID string `json:"projectId"`
            UserID    string `json:"userId"`
        }
        in, err := decodeArg[removeArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        if _, err := client.RemoveMember(rpcCtx, &projectv1.RemoveMemberRequest{
            ProjectId: in.ProjectID, UserId: in.UserID,
        }); err != nil {
            return nil, err
        }
        return map[string]bool{"ok": true}, nil
    })

    r.Register("project.updateMemberRole", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type updateArgs struct {
            ProjectID string `json:"projectId"`
            UserID    string `json:"userId"`
            Role      string `json:"role"`
        }
        in, err := decodeArg[updateArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.UpdateMemberRole(rpcCtx, &projectv1.UpdateMemberRoleRequest{
            ProjectId: in.ProjectID, UserId: in.UserID, Role: toProjectRole(in.Role),
        })
        if err != nil {
            return nil, err
        }
        return resp.GetMember(), nil
    })
}
```

`RegisterRealChannels` gains a `projectClient projectv1.ProjectServiceClient`
parameter and a `registerProjectChannels(r, projectClient)` call —
`main.go`'s composition root already dials one for `project_routes.go`.

---

## Test plan

- `services/project-service/internal/domain/membership_test.go` —
  `AssertNotLastOwnerRemoval` table test: last owner removed → error; last
  owner demoted → error; non-last owner removed/demoted → ok; removing a
  non-owner never errors regardless of owner count.
- `services/project-service/internal/usecase/remove_member_test.go` /
  `update_member_role_test.go` — in-memory `ProjectRepository` fake;
  assert the ownerless guard fires *before* any repository mutation call
  (no partial write on rejection); assert non-owner/non-admin actor is
  denied by `requireProjectAccess`.
- `services/project-service/internal/usecase/list_members_test.go` — any
  member (including the lowest role) can list; a non-member is denied.
- `services/project-service/internal/adapter/postgres/*_test.go` —
  `testcontainers-go`: `CountOwners` reflects concurrent
  add/remove/role-update correctly (this is the exact number the ownerless
  guard trusts).
- `services/api-gateway/internal/adapter/wscompat/channels_test.go` — 7
  tests (4 wiring-only + 3 new), fake `ProjectServiceClient`, asserting
  request-field mapping including `project.getMembers` → `ListMembers` name
  mismatch is handled correctly.

## References

- `specs/backend-go/tdd/services/project-service.md:82-86` — `ListMembers`/`RemoveMember`/`UpdateMemberRole` already-specified in the target RPC surface
- `specs/backend-go/tdd/services/project-service.md §4,§9` — `ProjectMember` entity, "≥1 owner" invariant, role-based access rules
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` — domain/usecase/port layering (pure-function invariant, I/O-bearing usecase)
- `backend-go/services/project-service/internal/domain/membership.go` — `ProjectRole`'s "documented follow-up once ListMembers/UpdateMemberRole/the '≥1 owner' invariant are ported" doc comment
- `backend-go/services/project-service/internal/usecase/ports.go:22,41-44` — existing `AddMember`/`GetMembership`, the pattern the 4 new methods extend
- `backend-go/proto/orca/project/v1/project.proto:11-45` — current (reduced) RPC surface; `AddMemberResponse{}`/`DeleteProjectResponse{}` empty-wrapper convention followed above
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:390-433` — `registerDevServerChannels`, the wiring pattern mirrored above
- `backend-go/services/api-gateway/internal/adapter/httpgateway/project_routes.go:21-50,75-168` — `mountProjectRoutes`, the 4 already-working REST handlers `project.create/get/list/update` reuse
- `specs/backend-go/bugs/missing-v1/BUG-020-project-channels-not-implemented.md` — the bug this resolves
