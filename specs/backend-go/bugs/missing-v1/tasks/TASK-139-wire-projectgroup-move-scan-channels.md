# TASK-139: Wire `projectGroup.moveProject`/`scanNested`/`importNested`

**From Solution:** SOL-021
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-136 (creates `registerProjectGroupChannels`), TASK-138 (generated `projectv1` stubs for the new RPCs)
**Status:** `[ ]` TODO

---

## Context

Extends `registerProjectGroupChannels` (added in TASK-136, currently
`create`/`update`/`delete`/`list`) with the 3 channels that call the new
RPCs TASK-138 implemented. `scanNested` gets a longer, explicit 30s per-call
deadline instead of the standard `rpcTimeout` — a filesystem scan over a
possibly-deep tree on a remote host can legitimately exceed 8s, same
reasoning `infra-fleet-service.md` §8's "Deadlines" note documents for
Agent-bound calls.

## Changes to make

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

Inside `registerProjectGroupChannels` (from TASK-136), append these 3
`r.Register` calls right after `projectGroup.list`'s block, before the
function's closing `}`:

```go
	r.Register("projectGroup.moveProject", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type moveArgs struct {
			ProjectID           string `json:"projectId"`
			TargetParentGroupID string `json:"targetParentGroupId"`
		}
		in, err := decodeArg[moveArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.MoveProject(rpcCtx, &projectv1.MoveProjectRequest{
			ProjectId: in.ProjectID, TargetParentGroupId: in.TargetParentGroupID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetGroup(), nil
	})

	r.Register("projectGroup.scanNested", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type scanArgs struct {
			DevServerID string `json:"devServerId"`
			RootPath    string `json:"rootPath"`
		}
		in, err := decodeArg[scanArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		// Longer, explicit deadline: a filesystem scan over a possibly-deep
		// tree on a remote host can legitimately exceed rpcTimeout — see
		// infra-fleet-service.md §8's "Deadlines" note for Agent-bound calls.
		rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		resp, err := client.ScanNested(rpcCtx, &projectv1.ScanNestedRequest{
			DevServerId: in.DevServerID, RootPath: in.RootPath,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetCandidates(), nil
	})

	r.Register("projectGroup.importNested", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type candidateArg struct {
			Path          string `json:"path"`
			SuggestedName string `json:"suggestedName"`
			IsGitRepo     bool   `json:"isGitRepo"`
		}
		type importArgs struct {
			DevServerID   string         `json:"devServerId"`
			ParentGroupID string         `json:"parentGroupId"`
			Selected      []candidateArg `json:"selected"`
		}
		in, err := decodeArg[importArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		selected := make([]*projectv1.NestedRepoCandidate, 0, len(in.Selected))
		for _, c := range in.Selected {
			selected = append(selected, &projectv1.NestedRepoCandidate{
				Path: c.Path, SuggestedName: c.SuggestedName, IsGitRepo: c.IsGitRepo,
			})
		}
		resp, err := client.ImportNested(rpcCtx, &projectv1.ImportNestedRequest{
			DevServerId: in.DevServerID, ParentGroupId: in.ParentGroupID, Selected: selected,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
```

Note: `ImportNestedRequest` (per TASK-137's proto) has no `dev_server_id`
field on `project.proto`'s message itself in this sketch's earlier draft —
double check against the actual generated `projectv1.ImportNestedRequest`
struct before wiring; if the field is present (as TASK-137 defines it),
the mapping above is correct as written.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```

Expected: clean build. All 7 `projectGroup.*` channels now resolve through
`wscompat.Registry`.
