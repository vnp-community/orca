# SOL-027: Wire `workspacePorts.scan` to the existing `ScanWorkspacePorts` RPC; add `KillWorkspacePort` following the identical resolve→(relay|local) shape

**Resolves:** [BUG-027](../BUG-027-workspaceports-channels-not-implemented.md)
**Service:** `infra-fleet-service` (new `KillWorkspacePort` RPC) + `api-gateway` (both `wscompat` channels)
**Affected files (proposed):**
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
- `backend-go/services/infra-fleet-service/internal/usecase/kill_workspace_port.go` (new)
- `backend-go/services/infra-fleet-service/internal/usecase/kill_workspace_port_test.go` (new)
- `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/infra_routes.go` (REST parity, optional)
**Status:** 📋 Proposed — not yet implemented

---

## Two different states, one namespace — `scan` is wiring-only, `kill` is new work

BUG-027 already drew this line precisely: `ScanWorkspacePorts` is real,
tested, REST-wired, and already correctly relays per-`connectionId`
(closes TS Gap 7, `scan_workspace_ports.go:17-24`) — it just isn't a
`wscompat` channel yet. `KillWorkspacePort` doesn't exist at any layer.
This solution does not invent a different transport for `kill` — it
follows the exact `resolve → (relay to agent | local no-op)` shape
`ScanWorkspacePorts` already established, per the task's own instruction
not to reproduce the old TS bug class (`workspacePorts.scan`/`kill`
silently dropping remote-connected repos, `backend-agent-execution-boundary.md:118,179`).

---

## Design — `workspacePorts.scan` wiring (no new RPC, no new usecase)

```go
// ── workspacePorts.* ─────────────────────────────────────────────────────
//
// scan calls the already-implemented ScanWorkspacePorts (infrafleet.proto:15,
// 146-153) — closes TS Gap 7 by construction, see scan_workspace_ports.go's
// doc comment: always resolves the connection first, relays to the agent's
// ports.scan when connectionId is bound, only returns [] for a genuinely
// unconnected worktree. This handler is pure wiring, matching
// registerDevServerChannels's pattern (channels.go:390-433).
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
        // rpcTimeout (8s) matches fleet.health.checkAll's reasoning
        // (channels.go:462-465): enough for one round-trip, still fails
        // before invokeTimeout.
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

    r.Register("workspacePorts.kill", registerKillHandler(client))
}
```

**Arg-shape caveat, flagged per this file's own package doc comment**: the
frontend's `killWorkspacePortForTarget`/scan call sites pass
`{ repoId, pid, port }` / `{ repoId }`
(`workspace-port-actions.ts:151-152,320-321`), not `{connectionId,
worktreeId}` directly — `repoId` needs resolving to the
worktree's `connectionId` before calling `ScanWorkspacePorts`/
`KillWorkspacePort` (both take `connection_id`/`worktree_id`, not
`repo_id`, per `infrafleet.proto:146-149` and the new message below).
Verify the exact `repoId` → `connectionId`/`worktreeId` lookup against
the real frontend call site before shipping — likely a
`project-service.ListWorktrees` join keyed by `repoId`, resolved either in
this handler or upstream of it. Not resolved further in this proposal
per this file's own "best-effort, verify against the actual call site"
convention (`channels.go`'s top-of-file doc comment).

`toWorkspacePortScanResult` maps `[]int32` open ports onto the frontend's
`WorkspacePortScanResult{platform, scannedAt, ports, unavailableReason?}`
shape (`frontend/src/shared/workspace-ports.ts:70-75`) — `platform`
reported as `"unknown"` (this service never inspects the target host's
OS) and `unavailableReason` left empty on success, honest-placeholder
convention again.

---

## Design — `KillWorkspacePort`, new RPC on `infra-fleet-service`

```protobuf
// infrafleet.proto — next to ScanWorkspacePorts, same request shape plus
// the process identity to kill.
rpc KillWorkspacePort(KillWorkspacePortRequest) returns (KillWorkspacePortResponse);

message KillWorkspacePortRequest {
  string connection_id = 1;
  string worktree_id = 2;
  int32 pid = 3;
  int32 port = 4;
}

message KillWorkspacePortResponse {
  bool ok = 1;
  string reason = 2; // populated only when ok is false
}
```

```go
// internal/usecase/kill_workspace_port.go
//
// KillWorkspacePort follows ScanWorkspacePorts's exact resolve-then-dispatch
// shape (scan_workspace_ports.go:17-62) deliberately — this is the same
// "always resolve the connection first, relay when bound, never a silent
// if(connectionId) shortcut" structure, applied to a kill instead of a
// scan. Do not reintroduce backend-agent-execution-boundary.md:118's old
// TS bug class here.
type KillWorkspacePortInput struct {
    ConnectionID string
    WorktreeID   string
    PID          int32
    Port         int32
}

type KillWorkspacePort struct {
    resolver ConnectionResolver
    agent    DevServerAgentClient
}

func NewKillWorkspacePort(resolver ConnectionResolver, agent DevServerAgentClient) *KillWorkspacePort {
    return &KillWorkspacePort{resolver: resolver, agent: agent}
}

func (uc *KillWorkspacePort) Execute(ctx context.Context, in KillWorkspacePortInput) (ok bool, reason string, err error) {
    tenantID, err := tenant.RequireTenantID(ctx)
    if err != nil {
        return false, "", apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
    }

    if in.ConnectionID != "" {
        connected, devServer, _, resolveErr := uc.resolver.ResolveConnection(ctx, tenantID, in.ConnectionID)
        if resolveErr != nil {
            return false, "", apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", resolveErr)
        }
        if connected {
            // Relay to the agent's ports.kill handler — same Exec port
            // ScanWorkspacePorts already uses, different method name.
            // A resolve failure or agent error is a real error, propagated
            // here, not swallowed into a false "ok:true".
            result, execErr := uc.agent.Exec(ctx, devServer, "ports.kill", map[string]any{
                "worktreeId": in.WorktreeID, "pid": in.PID, "port": in.Port,
            })
            if execErr != nil {
                return false, "", apperrors.New(apperrors.KindInternal, "INFRA_AGENT_EXEC_FAILED", "failed to relay workspace port kill to dev server agent", execErr)
            }
            return decodeKillResult(result)
        }
    }

    // No connectionId bound (or it didn't resolve): the worktree is local.
    // Actually killing a local process is out of scope for this scaffold —
    // same "routing, not executing" boundary scan_workspace_ports.go:58-62
    // already draws for the local branch. Honest ok:false, not a silent
    // no-op success — the frontend's WorkspacePortKillResult type already
    // has a {ok:false, reason} shape for exactly this case
    // (workspace-port-actions.ts:63-68).
    return false, "local workspace-port kill is not implemented in this scaffold", nil
}

func decodeKillResult(result map[string]any) (bool, string, error) {
    ok, _ := result["ok"].(bool)
    reason, _ := result["reason"].(string)
    return ok, reason, nil
}
```

`ConnectionResolver`/`DevServerAgentClient` are the exact same ports
`ScanWorkspacePorts` already depends on (`ports.go`) — no new port
interface needed, this usecase is a peer of `scan_workspace_ports.go`, not
a new pattern.

gRPC server method (`internal/adapter/grpc/server.go`, next to
`ScanWorkspacePorts`'s existing method):

```go
func (s *Server) KillWorkspacePort(ctx context.Context, req *infrafleetv1.KillWorkspacePortRequest) (*infrafleetv1.KillWorkspacePortResponse, error) {
    ok, reason, err := s.killWorkspacePort.Execute(ctx, usecase.KillWorkspacePortInput{
        ConnectionID: req.GetConnectionId(),
        WorktreeID:   req.GetWorktreeId(),
        PID:          req.GetPid(),
        Port:         req.GetPort(),
    })
    if err != nil {
        return nil, apperrors.ToGRPCStatus(err)
    }
    return &infrafleetv1.KillWorkspacePortResponse{Ok: ok, Reason: reason}, nil
}
```

`wscompat` wiring, joining `registerWorkspacePortsChannels` above:

```go
func registerKillHandler(client infrafleetv1.InfraFleetServiceClient) ChannelHandler {
    return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
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
    }
}
```

Register both in `RegisterRealChannels` (`channels.go:70`, next to
`registerDevServerChannels`/`registerFleetChannels`):

```go
registerWorkspacePortsChannels(r, infraFleetClient) // NEW — scan + kill
```

REST parity (optional, matching `scan`'s existing
`POST /v1/infra/workspaces/scan-ports`, `infra_routes.go:28,202-227`):
add `POST /v1/infra/workspaces/kill-port` with the equivalent
`handleKillWorkspacePort`, same hand-written translation pattern as
`handleScanWorkspacePorts`. Not required for BUG-027 (frontend only calls
this over WS), included for symmetry with the sibling RPC's REST+WS
double-wiring, consistent with every other RPC in this file.

---

## Test plan

- `services/infra-fleet-service/internal/usecase/kill_workspace_port_test.go`
  — mirror `scan_workspace_ports_test.go`'s exact table shape: (1) no
  `connectionId` → `ok:false`, honest "not implemented" reason, no agent
  call; (2) `connectionId` resolves, not connected → same as (1); (3)
  `connectionId` resolves, connected → fake `DevServerAgentClient.Exec`
  called with `"ports.kill"` and the right params, result decoded into
  `ok`/`reason`; (4) agent `Exec` error → propagated as
  `INFRA_AGENT_EXEC_FAILED`, never swallowed into `ok:false`.
- `services/api-gateway/internal/adapter/wscompat/channels_test.go` — one
  test per channel: `workspacePorts.scan` calls
  `ScanWorkspacePortsClient` with decoded args and reshapes the response;
  `workspacePorts.kill` calls `KillWorkspacePortClient` and passes through
  `{ok, reason}` verbatim on failure, `{ok:true}` on success.
- Regression guard test asserting `workspacePorts.scan`/`workspacePorts.kill`
  are both present in the registry (no `notImplementedHandler`
  fallthrough) — closes this bug the same way BUG-002's sibling reports are
  closed.

## References

- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:15,146-153` — existing `ScanWorkspacePorts` RPC/messages, template for `KillWorkspacePort`
- `backend-go/services/infra-fleet-service/internal/usecase/scan_workspace_ports.go:17-79` — the resolve→(relay|local) shape `KillWorkspacePort` mirrors exactly
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go:63-90` — `ConnectionResolver`/`DevServerAgentClient` ports reused as-is, no new port needed
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:390-433` — `registerDevServerChannels`, wiring pattern mirrored for both channels
- `backend-go/services/api-gateway/internal/adapter/httpgateway/infra_routes.go:28,202-227` — `handleScanWorkspacePorts`, REST-parity template for the optional `kill-port` route
- `frontend/src/renderer/src/lib/workspace-port-actions.ts:151-152,275-283,319-330` — actual frontend call sites/args (`{repoId}` / `{repoId, pid, port}`) — flags the `repoId`→`connectionId`/`worktreeId` resolution gap noted above
- `frontend/src/shared/workspace-ports.ts:63-75` — `WorkspacePortKillResult`/`WorkspacePortScanResult` response shapes this design targets
- `specs/backend-go/tdd/services/infra-fleet-service.md` §2,§3,§10 — `ScanWorkspacePorts`'s "always relay, no local shortcut" design (closes TS Gap 7), the precedent this solution's `KillWorkspacePort` follows
- `specs/frontend/api/backend-agent-execution-boundary.md:118,179` — old-backend silent-drop bug class this solution avoids reproducing
- `specs/backend-go/bugs/missing-v1/BUG-027-workspaceports-channels-not-implemented.md` — full analysis this solution implements
