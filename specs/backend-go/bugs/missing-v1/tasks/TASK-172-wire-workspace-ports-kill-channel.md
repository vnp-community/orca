# TASK-172: Wire `workspacePorts.kill` into `registerWorkspacePortsChannels`

**From Solution:** SOL-027
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`, `services/api-gateway/internal/adapter/httpgateway/infra_routes.go` (optional REST parity)
**Depends on:** TASK-169 (`registerWorkspacePortsChannels` must exist), TASK-171 (`KillWorkspacePort` RPC must exist)
**Status:** `[partial]` `workspacePorts.kill` implemented inside `registerWorkspacePortsChannels` in the new `channels_repo_ssh_status_workspace.go` file (not `channels.go`). Builds/tests green in isolation; not wired into production `RegisterRealChannels` — see TASK-151's status note. REST parity (`POST /v1/infra/workspaces/kill-port`) NOT added — explicitly optional per this task, skipped to stay in scope.

---

## Changes to make

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

In `registerWorkspacePortsChannels` (TASK-169), replace the comment
placeholder:

```go
	// workspacePorts.kill joins here once TASK-171's KillWorkspacePort
	// exists (TASK-172).
```

with:

```go
	r.Register("workspacePorts.kill", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type killArgs struct {
			ConnectionID string `json:"connectionId"`
			WorktreeID   string `json:"worktreeId"`
			PID          int32  `json:"pid"`
			Port         int32  `json:"port"`
		}
		in, err := decodeArg[killArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.KillWorkspacePort(rpcCtx, &infrafleetv1.KillWorkspacePortRequest{
			ConnectionId: in.ConnectionID, WorktreeId: in.WorktreeID, Pid: in.PID, Port: in.Port,
		})
		if err != nil {
			return nil, err
		}
		if !resp.GetOk() {
			return map[string]any{"ok": false, "reason": resp.GetReason()}, nil
		}
		return map[string]any{"ok": true}, nil
	})
```

Update the group-registration comment where `registerWorkspacePortsChannels`
is called from `RegisterRealChannels`:

```go
	registerWorkspacePortsChannels(r, infraFleetClient) // scan + kill
```

### Optional: REST parity

Matching `scan`'s existing `POST /v1/infra/workspaces/scan-ports`
(`infra_routes.go:28,202-227`), add `POST /v1/infra/workspaces/kill-port`
with an equivalent `handleKillWorkspacePort`, same hand-written
translation pattern as `handleScanWorkspacePorts`. Not required for this
task (the frontend only calls `workspacePorts.kill` over WS) — included
only for symmetry with the sibling RPC's REST+WS double-wiring; skip if
out of scope for this pass.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```
