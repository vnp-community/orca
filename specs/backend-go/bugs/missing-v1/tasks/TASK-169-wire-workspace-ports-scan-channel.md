# TASK-169: Wire `workspacePorts.scan` to the existing `ScanWorkspacePorts` RPC (no new RPC)

**From Solution:** SOL-027
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** none
**Status:** `[partial]` `registerWorkspacePortsChannels`/`workspacePorts.scan`/`toWorkspacePortScanResult` implemented in the new `channels_repo_ssh_status_workspace.go` file (not `channels.go`). The `repoId` → `connectionId`/`worktreeId` resolution gap flagged in the task's context note is NOT resolved — this handler decodes `{connectionId, worktreeId}` directly, as the task itself anticipates. Builds/tests green in isolation; not wired into production `RegisterRealChannels` — see TASK-151's status note.

---

## Context

`ScanWorkspacePorts` is real, tested, REST-wired, and already correctly
relays per-`connectionId` (closes TS Gap 7, see
`services/infra-fleet-service/internal/usecase/scan_workspace_ports.go:17-24`'s
doc comment) — it just isn't a `wscompat` channel yet. This task is pure
wiring, no new RPC, no new usecase. `workspacePorts.kill` (needs a new
RPC) is TASK-170 through TASK-172.

**Arg-shape caveat**: the frontend's `killWorkspacePortForTarget`/scan
call sites pass `{ repoId, pid, port }` / `{ repoId }`
(`frontend/src/renderer/src/lib/workspace-port-actions.ts:151-152,320-321`),
not `{connectionId, worktreeId}` directly — `repoId` needs resolving to
the worktree's `connectionId` before calling `ScanWorkspacePorts` (which
takes `connection_id`/`worktree_id`, per `infrafleet.proto:146-149`).
This handler decodes `{connectionId, worktreeId}` directly per this file's
own "best-effort, verify against the actual call site" convention
(top-of-file doc comment) — verify the exact `repoId` → `connectionId`/
`worktreeId` lookup against the real frontend call site before shipping;
likely a `project-service.ListWorktrees` join keyed by `repoId`, resolved
either in this handler or upstream of it. Not resolved further here.

## Changes to make

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

```go
// ── workspacePorts.* ─────────────────────────────────────────────────────
//
// scan calls the already-implemented ScanWorkspacePorts (infrafleet.proto:15,
// 146-153) — closes TS Gap 7 by construction, see scan_workspace_ports.go's
// doc comment: always resolves the connection first, relays to the agent's
// ports.scan when connectionId is bound, only returns [] for a genuinely
// unconnected worktree. This handler is pure wiring, matching
// registerDevServerChannels's pattern. workspacePorts.kill (TASK-172)
// joins this function once TASK-170/TASK-171's KillWorkspacePort RPC exists.
func registerWorkspacePortsChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("workspacePorts.scan", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type scanArgs struct {
			ConnectionID string `json:"connectionId"`
			WorktreeID   string `json:"worktreeId"`
		}
		in, err := decodeArg[scanArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		// scan can legitimately touch a remote host (relay branch) —
		// rpcTimeout (8s) matches fleet.health.checkAll's reasoning:
		// enough for one round-trip, still fails before invokeTimeout.
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ScanWorkspacePorts(rpcCtx, &infrafleetv1.ScanWorkspacePortsRequest{
			ConnectionId: in.ConnectionID,
			WorktreeId:   in.WorktreeID,
		})
		if err != nil {
			return nil, err
		}
		return toWorkspacePortScanResult(resp.GetOpenPorts()), nil
	})

	// workspacePorts.kill joins here once TASK-171's KillWorkspacePort
	// exists (TASK-172).
}

// toWorkspacePortScanResult maps ScanWorkspacePortsResponse's []int32 open
// ports onto the frontend's WorkspacePortScanResult{platform, scannedAt,
// ports, unavailableReason?} shape (frontend/src/shared/workspace-ports.ts:70-75)
// — platform reported as "unknown" (this service never inspects the
// target host's OS) and unavailableReason left empty on success, honest-
// placeholder convention.
func toWorkspacePortScanResult(openPorts []int32) map[string]any {
	return map[string]any{
		"platform":  "unknown",
		"scannedAt": time.Now().UnixMilli(),
		"ports":     openPorts,
	}
}
```

Register from `RegisterRealChannels`, next to
`registerDevServerChannels`/`registerFleetChannels`:

```go
	registerWorkspacePortsChannels(r, infraFleetClient) // NEW — scan (kill joins in TASK-172)
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```
