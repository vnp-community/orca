# TASK-134: Wire `project.getMembers`/`removeMember`/`updateMemberRole`

**From Solution:** SOL-020
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-131 (creates `registerProjectChannels`), TASK-133 (generated `projectv1` stubs for the new RPCs)
**Status:** `[x]` DONE (verified) — all 3 channels added to `registerProjectChannels` in `channels_tenant_project.go` (new file). `toProjectRoleArg` helper added. `go build`/`go vet` green; dedicated tests pass.

---

## Context

Extends `registerProjectChannels` (added in TASK-131, currently
`create`/`get`/`list`/`update`) with the 3 channels that call the new RPCs
TASK-133 implemented.

## Changes to make

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

Inside `registerProjectChannels` (from TASK-131), append these 3
`r.Register` calls right after `project.update`'s block, before the
function's closing `}`:

```go
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
			ProjectId: in.ProjectID, UserId: in.UserID, Role: toProjectRoleArg(in.Role),
		})
		if err != nil {
			return nil, err
		}
		return resp.GetMember(), nil
	})
```

Add the small arg-string → proto-enum mapper this last handler needs
(append near the other small helpers in this file, e.g. next to
`toConnectionMode`):

```go
// toProjectRoleArg maps the wscompat wire arg's role string ("member" |
// "owner") onto projectv1.ProjectRole — mirrors toConnectionMode's shape
// for the same kind of string-to-enum wire mapping.
func toProjectRoleArg(role string) projectv1.ProjectRole {
	switch role {
	case "owner":
		return projectv1.ProjectRole_PROJECT_ROLE_OWNER
	case "member":
		return projectv1.ProjectRole_PROJECT_ROLE_MEMBER
	default:
		return projectv1.ProjectRole_PROJECT_ROLE_UNSPECIFIED
	}
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```

Expected: clean build. All 7 `project.*` channels now resolve through
`wscompat.Registry`.
