# TASK-AG-03-06: Wire `ResumeAgentSession` (grpc/main/wscompat) + start the per-dev-server `agent.hook` consumer

**From Solution:** SOL-AG-03
**Priority:** P0
**Service:** `infra-fleet-service` + `api-gateway`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`, `backend-go/services/infra-fleet-service/cmd/server/main.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_agent.go`
**Depends on:** TASK-AG-03-04, TASK-AG-03-05
**Status:** `[ ]` TODO

---

## Context

Wires `ResumeAgentSession` the same way `StartAgentSession` was wired (TASK-AG-01-07/08), and starts exactly one `RecordAgentHookProviderSession.Run` goroutine per dev server connection — guarded by a small in-process registry so a second `StartAgentSession`/`ResumeAgentSession` call against an already-subscribed dev server doesn't start a duplicate consumer.

## Changes to make

### `internal/adapter/grpc/server.go`

```go
func (s *Server) ResumeAgentSession(ctx context.Context, req *infrafleetv1.ResumeAgentSessionRequest) (*infrafleetv1.AgentSession, error) {
	session, err := s.resumeAgentSession.Execute(ctx, usecase.ResumeAgentSessionInput{
		ConnectionID: req.GetConnectionId(),
		WorktreeID:   req.GetWorktreeId(),
		UserID:       req.GetUserId(),
		Cwd:          req.GetCwd(),
		Cols:         req.GetCols(),
		Rows:         req.GetRows(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoAgentSession(session), nil
}
```

Add `resumeAgentSession *usecase.ResumeAgentSession` to `Server` and thread
it through the constructor, same as every other usecase field.

### `cmd/server/main.go`

```go
resumeAgentSessionUC := usecase.NewResumeAgentSession(agentSessionStore, repo, startAgentSessionUC)

// Best-effort agent.hook consumer — one goroutine per dev server this
// process has resolved a connection for, guarded against duplicate starts.
// This is intentionally simple (a map + mutex in main.go, not a separate
// registry type) since it has exactly one caller today.
var (
	hookConsumersMu sync.Mutex
	hookConsumers   = map[string]bool{} // dev server id -> already started
)
ensureAgentHookConsumer := func(ctx context.Context, tenantID string, devServer domain.DevServer) {
	hookConsumersMu.Lock()
	defer hookConsumersMu.Unlock()
	if hookConsumers[devServer.ID] {
		return
	}
	hookConsumers[devServer.ID] = true
	recorder := usecase.NewRecordAgentHookProviderSession(agentSessionStore)
	go recorder.Run(context.Background(), tenantID, devServer, agentClient)
}
```

`ensureAgentHookConsumer` needs a call site inside `StartAgentSession`/
`ResumeAgentSession` (or immediately after each in the gRPC handler) — the
cleanest place is `StartAgentSession.Execute` itself, right after
`ResolveConnection` succeeds, since every path that needs a hook consumer
running already resolves a `devServer` there. If threading a callback into
`StartAgentSession` is too invasive for this pass, an acceptable interim
is calling `ensureAgentHookConsumer` from the two gRPC handlers
(`StartAgentSession`/`ResumeAgentSession` in `server.go`) right after a
successful `Execute` — slightly later (the hook consumer starts a beat
after the spawn call), acceptable since `agent.hook` events arrive well
after the CLI itself starts producing output.

### `api-gateway`'s `channels_agent.go`

```go
type agentResumeArgs struct {
	ConnectionID string `json:"connectionId"`
	WorktreeID   string `json:"worktreeId"`
	UserID       string `json:"userId"`
	Cwd          string `json:"cwd"`
	Cols         int32  `json:"cols"`
	Rows         int32  `json:"rows"`
}

func registerAgentResumeChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.RegisterStreamChannel("agent.resume", func(ctx context.Context, id Identity, args []json.RawMessage) (any, <-chan PushEvent, error) {
		in, err := decodeArg[agentResumeArgs](args, 0)
		if err != nil {
			return nil, nil, err
		}
		streams := terminalStreamsFromContext(ctx)
		if streams == nil {
			return nil, nil, errNoTerminalStreamRegistry
		}
		invokeCtx := gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		session, err := client.ResumeAgentSession(invokeCtx, &infrafleetv1.ResumeAgentSessionRequest{
			ConnectionId: in.ConnectionID, WorktreeId: in.WorktreeID, UserId: in.UserID,
			Cwd: in.Cwd, Cols: in.Cols, Rows: in.Rows,
		})
		if err != nil {
			return nil, nil, err
		}
		// Same AttachPty-stream setup as agent.start (TASK-AG-01-08) — a
		// resumed session's ptyId is attachable exactly like a fresh one's.
		streamCtx, cancel := attachContext(id)
		stream, err := client.AttachPty(streamCtx)
		if err != nil {
			cancel()
			return nil, nil, fmt.Errorf("wscompat: opening AttachPty stream for resumed agent pty %q: %w", session.GetPtyId(), err)
		}
		if err := stream.Send(&infrafleetv1.PtyClientFrame{
			Frame: &infrafleetv1.PtyClientFrame_Attach{Attach: &infrafleetv1.AttachToSession{PtyId: session.GetPtyId()}},
		}); err != nil {
			cancel()
			return nil, nil, fmt.Errorf("wscompat: sending AttachPty's initial attach frame for resumed agent pty %q: %w", session.GetPtyId(), err)
		}
		entry := &terminalStreamEntry{stream: stream, cancel: cancel}
		streams.put(session.GetPtyId(), entry)
		events := make(chan PushEvent)
		go drainAttachPtyOutput(streamCtx, session.GetPtyId(), entry, streams, events)
		return toAgentSessionView(session), events, nil
	})
}
```

Add `registerAgentResumeChannel(r, client)` to `registerAgentChannels`
(TASK-AG-01-08).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/... ./services/api-gateway/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestResumeAgentSession -v
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestAgentResume -v
```
