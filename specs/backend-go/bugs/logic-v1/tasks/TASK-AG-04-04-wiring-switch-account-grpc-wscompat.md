# TASK-AG-04-04: Wire `SwitchAgentAccount` (grpc server + `agent.switchAccount` wscompat channel)

**From Solution:** SOL-AG-04
**Priority:** P0
**Service:** `infra-fleet-service` + `api-gateway`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_agent.go`
**Depends on:** TASK-AG-04-01, TASK-AG-04-03
**Status:** `[ ]` TODO

---

## Context

Final wiring step: registers the `SwitchAgentAccount` RPC and its `agent.switchAccount` wscompat channel, following the exact pattern `agent.start`/`agent.resume` already established (TASK-AG-01-08/TASK-AG-03-06).

## Changes to make

### `internal/adapter/grpc/server.go`

```go
func (s *Server) SwitchAgentAccount(ctx context.Context, req *infrafleetv1.SwitchAgentAccountRequest) (*infrafleetv1.AgentSession, error) {
	session, err := s.switchAgentAccount.Execute(ctx, usecase.SwitchAgentAccountInput{
		ConnectionID: req.GetConnectionId(),
		WorktreeID:   req.GetWorktreeId(),
		UserID:       req.GetUserId(),
		ProjectID:    req.GetProjectId(),
		Cwd:          req.GetCwd(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoAgentSession(session), nil
}
```

Add `switchAgentAccount *usecase.SwitchAgentAccount` to `Server` and thread
it through the constructor.

### `api-gateway`'s `channels_agent.go`

```go
type agentSwitchAccountArgs struct {
	ConnectionID string `json:"connectionId"`
	WorktreeID   string `json:"worktreeId"`
	UserID       string `json:"userId"`
	ProjectID    string `json:"projectId"`
	Cwd          string `json:"cwd"`
}

func registerAgentSwitchAccountChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.RegisterStreamChannel("agent.switchAccount", func(ctx context.Context, id Identity, args []json.RawMessage) (any, <-chan PushEvent, error) {
		in, err := decodeArg[agentSwitchAccountArgs](args, 0)
		if err != nil {
			return nil, nil, err
		}
		streams := terminalStreamsFromContext(ctx)
		if streams == nil {
			return nil, nil, errNoTerminalStreamRegistry
		}
		invokeCtx := gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		session, err := client.SwitchAgentAccount(invokeCtx, &infrafleetv1.SwitchAgentAccountRequest{
			ConnectionId: in.ConnectionID, WorktreeId: in.WorktreeID, UserId: in.UserID,
			ProjectId: in.ProjectID, Cwd: in.Cwd,
		})
		if err != nil {
			return nil, nil, err
		}
		// Same AttachPty-stream setup as agent.start/agent.resume — the new
		// session's ptyId is attachable exactly like any other.
		streamCtx, cancel := attachContext(id)
		stream, err := client.AttachPty(streamCtx)
		if err != nil {
			cancel()
			return nil, nil, fmt.Errorf("wscompat: opening AttachPty stream for switched agent pty %q: %w", session.GetPtyId(), err)
		}
		if err := stream.Send(&infrafleetv1.PtyClientFrame{
			Frame: &infrafleetv1.PtyClientFrame_Attach{Attach: &infrafleetv1.AttachToSession{PtyId: session.GetPtyId()}},
		}); err != nil {
			cancel()
			return nil, nil, fmt.Errorf("wscompat: sending AttachPty's initial attach frame for switched agent pty %q: %w", session.GetPtyId(), err)
		}
		entry := &terminalStreamEntry{stream: stream, cancel: cancel}
		streams.put(session.GetPtyId(), entry)
		events := make(chan PushEvent)
		go drainAttachPtyOutput(streamCtx, session.GetPtyId(), entry, streams, events)
		return toAgentSessionView(session), events, nil
	})
}
```

Add `registerAgentSwitchAccountChannel(r, client)` to `registerAgentChannels`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/... ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestAgentSwitchAccount -v
```
