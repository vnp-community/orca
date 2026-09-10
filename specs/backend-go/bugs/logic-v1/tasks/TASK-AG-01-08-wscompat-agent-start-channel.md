# TASK-AG-01-08: Register `agent.start` wscompat channel

**From Solution:** SOL-AG-01
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_agent.go` (new)
**Depends on:** TASK-AG-01-01, TASK-AG-01-07
**Status:** `[x]` DONE — `channels_agent.go` registers `agent.start` via `RegisterStreamChannel`, reusing `terminalStreamRegistry`/`drainAttachPtyOutput`; wired into `registerAgentChannels`/`RegisterRealChannels`; `TestAgentStartChannel_*` pass (`go test ./services/api-gateway/internal/adapter/wscompat/... -run TestAgentStart -v`).

---

## Context

Exposes `StartAgentSession` to the renderer as `agent.start`, following `channels_terminal.go`'s `terminal.create` shape exactly: `agent.spawn`'s output/exit notifications arrive over the same `AttachPty` stream mechanism plain PTYs already use (SOL-AG-01's rationale), so this channel reuses `terminalStreamRegistry`/`drainAttachPtyOutput` rather than inventing a second push path. New file per this repo's naming discipline — agent sessions are a distinct concept from generic terminals.

## Changes to make

Create `backend-go/services/api-gateway/internal/adapter/wscompat/channels_agent.go`:

```go
// channels_agent.go registers the agent.* wscompat channels (AI-CLI agent
// session control-plane) against infra-fleet-service's agent-session gRPC
// surface (TASK-AG-01..05). agent.start's output/exit notifications reuse
// the SAME AttachPty stream mechanism terminal.create already sets up
// (agent.spawn's ptyId is attachable exactly like a plain PTY's, per
// agent-spawner.ts's agent.output/agent.exited notifications) — this file
// reuses terminalStreamRegistry/drainAttachPtyOutput from channels_terminal.go
// rather than introducing a second push path.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

type agentStartArgs struct {
	ConnectionID string `json:"connectionId"`
	WorktreeID   string `json:"worktreeId"`
	UserID       string `json:"userId"`
	Cwd          string `json:"cwd"`
	ModelID      string `json:"modelId"`
	AccountID    string `json:"accountId"`
	TrustPreset  string `json:"trustPreset"`
	Cols         int32  `json:"cols"`
	Rows         int32  `json:"rows"`
}

func registerAgentChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	registerAgentStartChannel(r, client)
}

func registerAgentStartChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.RegisterStreamChannel("agent.start", func(ctx context.Context, id Identity, args []json.RawMessage) (any, <-chan PushEvent, error) {
		in, err := decodeArg[agentStartArgs](args, 0)
		if err != nil {
			return nil, nil, err
		}
		streams := terminalStreamsFromContext(ctx)
		if streams == nil {
			return nil, nil, errNoTerminalStreamRegistry
		}

		invokeCtx := gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		session, err := client.StartAgentSession(invokeCtx, &infrafleetv1.StartAgentSessionRequest{
			ConnectionId: in.ConnectionID,
			WorktreeId:   in.WorktreeID,
			UserId:       in.UserID,
			Cwd:          in.Cwd,
			ModelId:      in.ModelID,
			AccountId:    in.AccountID,
			TrustPreset:  in.TrustPreset,
			Cols:         in.Cols,
			Rows:         in.Rows,
		})
		if err != nil {
			return nil, nil, err
		}

		// See attachContext's doc comment: the stream MUST outlive this
		// invoke's own 25s deadline.
		streamCtx, cancel := attachContext(id)
		stream, err := client.AttachPty(streamCtx)
		if err != nil {
			cancel()
			return nil, nil, fmt.Errorf("wscompat: opening AttachPty stream for agent pty %q: %w", session.GetPtyId(), err)
		}
		if err := stream.Send(&infrafleetv1.PtyClientFrame{
			Frame: &infrafleetv1.PtyClientFrame_Attach{Attach: &infrafleetv1.AttachToSession{PtyId: session.GetPtyId()}},
		}); err != nil {
			cancel()
			return nil, nil, fmt.Errorf("wscompat: sending AttachPty's initial attach frame for agent pty %q: %w", session.GetPtyId(), err)
		}

		entry := &terminalStreamEntry{stream: stream, cancel: cancel}
		streams.put(session.GetPtyId(), entry)

		events := make(chan PushEvent)
		go drainAttachPtyOutput(streamCtx, session.GetPtyId(), entry, streams, events)

		return toAgentSessionView(session), events, nil
	})
}

// agentSessionView is the JSON shape sent back to the renderer on
// agent.start's ack frame.
type agentSessionView struct {
	ID                 string `json:"id"`
	PtyID              string `json:"ptyId"`
	WorktreeID         string `json:"worktreeId"`
	DevServerID        string `json:"devServerId"`
	UserID             string `json:"userId"`
	ModelID            string `json:"modelId"`
	AccountID          string `json:"accountId"`
	Status             string `json:"status"`
	StartedAtUnixMs    int64  `json:"startedAtUnixMs"`
	LastActiveAtUnixMs int64  `json:"lastActiveAtUnixMs"`
}

func toAgentSessionView(s *infrafleetv1.AgentSession) agentSessionView {
	return agentSessionView{
		ID: s.GetId(), PtyID: s.GetPtyId(), WorktreeID: s.GetWorktreeId(), DevServerID: s.GetDevServerId(),
		UserID: s.GetUserId(), ModelID: s.GetModelId(), AccountID: s.GetAccountId(), Status: s.GetStatus(),
		StartedAtUnixMs: s.GetStartedAtUnixMs(), LastActiveAtUnixMs: s.GetLastActiveAtUnixMs(),
	}
}
```

Call `registerAgentChannels(r, client)` from the same registration site that
already calls `registerTerminalCreateChannel(r, client)` (check
`channels_terminal.go`'s `registerTerminalChannels`-equivalent entry point
and the `Registry` construction site in `handler.go`/`registry.go`).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestAgentStart -v
```

Add a channel test mirroring `channels_terminal_test.go`'s `terminal.create`
coverage: a fake `InfraFleetServiceClient` returning a session on
`StartAgentSession` → `agent.start`'s ack carries the session view and a
live `AttachPty` stream is opened for its `ptyId`.
