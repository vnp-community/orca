# TASK-185: Implement `Resize`/`Kill`/`Stop`/`Wait`/`FocusTerminalSession` usecases + wire every terminal RPC into `grpc/server.go` and `main.go`

**From Solution:** SOL-029 (design part 3b: control group — backs `terminal.close`/`terminal.stop`/`terminal.wait`, plus resize/focus)
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `internal/usecase/resize_terminal_session.go`, `kill_terminal_session.go`, `stop_terminal_process.go`, `wait_terminal_session.go`, `focus_terminal_session.go` (all new), `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** TASK-181, TASK-182, TASK-183, TASK-184
**Status:** `[x]` DONE — implemented in worktree `agent-aa8bd8599a599323a` (team/terminal/workflow/worktree pass, merged into `integration/missing-v1` as commit `baa34819a`); this task doc's own Status line was never updated by that implementing pass (a task-doc-capture gap, not a missing-code gap) — verified against the current merged code+tests during a later re-audit: build/vet/test clean.

---

## Context

This task finishes the usecase layer (the last 5 of the 8 non-Spawn/AttachPty
lifecycle RPCs) and — since every terminal usecase now exists — does the
one-shot wiring of all 10 terminal RPCs (9 unary + `AttachPty`) into
`internal/adapter/grpc/server.go` and `cmd/server/main.go`, following the
exact pattern `git-gateway-service`'s `grpc/server.go`/`main.go` already use
(translate-only handlers calling `usecase.X.Execute`, composition root
constructs every usecase once and passes it into `grpc.New(...)`).

## Changes to make

### Step 1 — the 5 remaining usecases

`internal/usecase/resize_terminal_session.go` (new):

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type ResizeTerminalSession struct {
	sessions    TerminalSessionRepository
	connections ConnectionResolver
	agent       DevServerAgentClient
}

func NewResizeTerminalSession(sessions TerminalSessionRepository, connections ConnectionResolver, agent DevServerAgentClient) *ResizeTerminalSession {
	return &ResizeTerminalSession{sessions: sessions, connections: connections, agent: agent}
}

func (uc *ResizeTerminalSession) Execute(ctx context.Context, ptyID string, cols, rows int32) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	session, found, err := uc.sessions.Get(ctx, tenantID, ptyID)
	if err != nil || !found {
		return apperrors.New(apperrors.KindNotFound, "TERMINAL_SESSION_NOT_FOUND", "terminal session not found", err)
	}
	connected, devServer, _, err := uc.connections.ResolveConnection(ctx, tenantID, session.ConnectionID)
	if err != nil || !connected {
		return apperrors.New(apperrors.KindNotFound, "TERMINAL_CONNECTION_NOT_FOUND", "connection not found", err)
	}
	if err := uc.agent.ResizePty(ctx, devServer, ptyID, cols, rows); err != nil {
		return apperrors.New(apperrors.KindInternal, "TERMINAL_RESIZE_FAILED", "failed to resize terminal", err)
	}
	return nil
}
```

`internal/usecase/kill_terminal_session.go` (new) — backs `terminal.close`:

```go
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type KillTerminalSession struct {
	sessions    TerminalSessionRepository
	connections ConnectionResolver
	agent       DevServerAgentClient
	clock       func() time.Time
}

func NewKillTerminalSession(sessions TerminalSessionRepository, connections ConnectionResolver, agent DevServerAgentClient) *KillTerminalSession {
	return &KillTerminalSession{sessions: sessions, connections: connections, agent: agent, clock: time.Now}
}

func (uc *KillTerminalSession) Execute(ctx context.Context, ptyID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	session, found, err := uc.sessions.Get(ctx, tenantID, ptyID)
	if err != nil || !found {
		return apperrors.New(apperrors.KindNotFound, "TERMINAL_SESSION_NOT_FOUND", "terminal session not found", err)
	}
	connected, devServer, _, err := uc.connections.ResolveConnection(ctx, tenantID, session.ConnectionID)
	if err != nil || !connected {
		return apperrors.New(apperrors.KindNotFound, "TERMINAL_CONNECTION_NOT_FOUND", "connection not found", err)
	}
	if err := uc.agent.KillPty(ctx, devServer, ptyID); err != nil {
		return apperrors.New(apperrors.KindInternal, "TERMINAL_KILL_FAILED", "failed to kill terminal session", err)
	}
	_ = uc.sessions.MarkClosed(ctx, ptyID, uc.clock())
	return nil
}
```

`internal/usecase/stop_terminal_process.go` (new) — "stop" is an interrupt,
not a teardown (distinct from `KillTerminalSession`). No new agent-side
capability needed: it writes the interrupt control byte (`0x03`, Ctrl+C)
through the same `WritePty` path `terminal.send` uses:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type StopTerminalProcess struct {
	sessions    TerminalSessionRepository
	connections ConnectionResolver
	agent       DevServerAgentClient
}

func NewStopTerminalProcess(sessions TerminalSessionRepository, connections ConnectionResolver, agent DevServerAgentClient) *StopTerminalProcess {
	return &StopTerminalProcess{sessions: sessions, connections: connections, agent: agent}
}

func (uc *StopTerminalProcess) Execute(ctx context.Context, ptyID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	session, found, err := uc.sessions.Get(ctx, tenantID, ptyID)
	if err != nil || !found {
		return apperrors.New(apperrors.KindNotFound, "TERMINAL_SESSION_NOT_FOUND", "terminal session not found", err)
	}
	connected, devServer, _, err := uc.connections.ResolveConnection(ctx, tenantID, session.ConnectionID)
	if err != nil || !connected {
		return apperrors.New(apperrors.KindNotFound, "TERMINAL_CONNECTION_NOT_FOUND", "connection not found", err)
	}
	if err := uc.agent.WritePty(ctx, devServer, ptyID, []byte{0x03}); err != nil {
		return apperrors.New(apperrors.KindInternal, "TERMINAL_STOP_FAILED", "failed to send interrupt", err)
	}
	return nil
}
```

`internal/usecase/wait_terminal_session.go` (new) — bounded blocking poll,
mandatory deadline (`08-inter-service-communication.md`):

```go
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// maxWaitTimeout caps WaitTerminalSessionRequest.timeout_ms — default 30s,
// same posture as workflow-service's 30-minute step timeout being an
// explicit, documented exception to the 5s intra-cluster default.
const maxWaitTimeout = 30 * time.Second

type WaitTerminalSessionResult struct {
	Exited   bool
	ExitCode int32
	TimedOut bool
}

type WaitTerminalSession struct {
	sessions    TerminalSessionRepository
	connections ConnectionResolver
	agent       DevServerAgentClient
}

func NewWaitTerminalSession(sessions TerminalSessionRepository, connections ConnectionResolver, agent DevServerAgentClient) *WaitTerminalSession {
	return &WaitTerminalSession{sessions: sessions, connections: connections, agent: agent}
}

func (uc *WaitTerminalSession) Execute(ctx context.Context, ptyID string, timeout time.Duration) (WaitTerminalSessionResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return WaitTerminalSessionResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if timeout <= 0 || timeout > maxWaitTimeout {
		timeout = maxWaitTimeout
	}
	session, found, err := uc.sessions.Get(ctx, tenantID, ptyID)
	if err != nil || !found {
		return WaitTerminalSessionResult{}, apperrors.New(apperrors.KindNotFound, "TERMINAL_SESSION_NOT_FOUND", "terminal session not found", err)
	}
	connected, devServer, _, err := uc.connections.ResolveConnection(ctx, tenantID, session.ConnectionID)
	if err != nil || !connected {
		return WaitTerminalSessionResult{}, apperrors.New(apperrors.KindNotFound, "TERMINAL_CONNECTION_NOT_FOUND", "connection not found", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	events, err := uc.agent.StreamPty(waitCtx, devServer, ptyID)
	if err != nil {
		return WaitTerminalSessionResult{}, apperrors.New(apperrors.KindInternal, "TERMINAL_STREAM_FAILED", "failed to open pty stream", err)
	}
	for {
		select {
		case <-waitCtx.Done():
			return WaitTerminalSessionResult{TimedOut: true}, nil
		case ev, ok := <-events:
			if !ok {
				return WaitTerminalSessionResult{TimedOut: true}, nil
			}
			if ev.Exited {
				return WaitTerminalSessionResult{Exited: true, ExitCode: ev.ExitCode}, nil
			}
		}
	}
}
```

`internal/usecase/focus_terminal_session.go` (new) — bookkeeping touch:

```go
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type FocusTerminalSession struct {
	sessions TerminalSessionRepository
	clock    func() time.Time
}

func NewFocusTerminalSession(sessions TerminalSessionRepository) *FocusTerminalSession {
	return &FocusTerminalSession{sessions: sessions, clock: time.Now}
}

func (uc *FocusTerminalSession) Execute(ctx context.Context, ptyID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if _, found, err := uc.sessions.Get(ctx, tenantID, ptyID); err != nil || !found {
		return apperrors.New(apperrors.KindNotFound, "TERMINAL_SESSION_NOT_FOUND", "terminal session not found", err)
	}
	if err := uc.sessions.Touch(ctx, ptyID, uc.clock()); err != nil {
		return apperrors.New(apperrors.KindInternal, "TERMINAL_FOCUS_FAILED", "failed to touch terminal session", err)
	}
	return nil
}
```

### Step 2 — `internal/adapter/grpc/server.go`: wire all 10 terminal RPCs

Add 10 fields to `Server` and 10 params to `New` (following this file's
existing `getStatus *usecase.GetStatus` field-per-usecase pattern from
git-gateway-service's equivalent file):

```go
	spawnTerminalSession    *usecase.SpawnTerminalSession
	attachPty               *usecase.AttachPty
	resizeTerminalSession   *usecase.ResizeTerminalSession
	killTerminalSession     *usecase.KillTerminalSession
	stopTerminalProcess     *usecase.StopTerminalProcess
	listTerminalSessions    *usecase.ListTerminalSessions
	waitTerminalSession     *usecase.WaitTerminalSession
	focusTerminalSession    *usecase.FocusTerminalSession
	getTerminalAgentStatus  *usecase.GetTerminalAgentStatus
	inspectTerminalProcess  *usecase.InspectTerminalProcess
```

Add the 9 unary handlers, each following `ListDevServers`'s existing
translate-only shape in this same file:

```go
func (s *Server) SpawnTerminalSession(ctx context.Context, req *infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
	tenantID, _ := tenant.FromContext(ctx) // however this file's existing handlers already pull tenantID — mirror ListDevServers/RegisterDevServer's exact call, do not invent a new tenant-extraction path
	session, err := s.spawnTerminalSession.Execute(ctx, tenantID, usecase.SpawnTerminalSessionInput{
		ConnectionID: req.GetConnectionId(), Cwd: req.GetCwd(), Shell: req.GetShell(), Cols: req.GetCols(), Rows: req.GetRows(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.SpawnTerminalSessionResponse{Session: toProtoTerminalSession(session)}, nil
}

func (s *Server) ResizeTerminalSession(ctx context.Context, req *infrafleetv1.ResizeTerminalSessionRequest) (*emptypb.Empty, error) {
	if err := s.resizeTerminalSession.Execute(ctx, req.GetPtyId(), req.GetCols(), req.GetRows()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) KillTerminalSession(ctx context.Context, req *infrafleetv1.KillTerminalSessionRequest) (*emptypb.Empty, error) {
	if err := s.killTerminalSession.Execute(ctx, req.GetPtyId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) StopTerminalProcess(ctx context.Context, req *infrafleetv1.StopTerminalProcessRequest) (*emptypb.Empty, error) {
	if err := s.stopTerminalProcess.Execute(ctx, req.GetPtyId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ListTerminalSessions(ctx context.Context, req *infrafleetv1.ListTerminalSessionsRequest) (*infrafleetv1.ListTerminalSessionsResponse, error) {
	sessions, err := s.listTerminalSessions.Execute(ctx, req.GetConnectionId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.TerminalSession, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, toProtoTerminalSession(sess))
	}
	return &infrafleetv1.ListTerminalSessionsResponse{Sessions: out}, nil
}

func (s *Server) WaitTerminalSession(ctx context.Context, req *infrafleetv1.WaitTerminalSessionRequest) (*infrafleetv1.WaitTerminalSessionResponse, error) {
	result, err := s.waitTerminalSession.Execute(ctx, req.GetPtyId(), time.Duration(req.GetTimeoutMs())*time.Millisecond)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.WaitTerminalSessionResponse{Exited: result.Exited, ExitCode: result.ExitCode, TimedOut: result.TimedOut}, nil
}

func (s *Server) FocusTerminalSession(ctx context.Context, req *infrafleetv1.FocusTerminalSessionRequest) (*emptypb.Empty, error) {
	if err := s.focusTerminalSession.Execute(ctx, req.GetPtyId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) GetTerminalAgentStatus(ctx context.Context, req *infrafleetv1.GetTerminalAgentStatusRequest) (*infrafleetv1.GetTerminalAgentStatusResponse, error) {
	status, err := s.getTerminalAgentStatus.Execute(ctx, req.GetPtyId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.GetTerminalAgentStatusResponse{AgentRunning: status.AgentRunning, AgentKind: status.AgentKind, ReadyForInput: status.ReadyForInput}, nil
}

func (s *Server) InspectTerminalProcess(ctx context.Context, req *infrafleetv1.InspectTerminalProcessRequest) (*infrafleetv1.InspectTerminalProcessResponse, error) {
	info, err := s.inspectTerminalProcess.Execute(ctx, req.GetPtyId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.InspectTerminalProcessResponse{Known: info.Known, Pid: info.PID, Command: info.Command, Cwd: info.Cwd}, nil
}

func toProtoTerminalSession(s domain.TerminalSession) *infrafleetv1.TerminalSession {
	return &infrafleetv1.TerminalSession{
		PtyId: s.PtyID, ConnectionId: s.ConnectionID, Cwd: s.Cwd,
		CreatedAtUnixMs: s.CreatedAt.UnixMilli(), LastActiveAtUnixMs: s.LastActiveAt.UnixMilli(),
	}
}
```

Add the streaming `AttachPty` handler — this is the one method whose
signature does NOT follow the unary translate-only shape above; it pumps
`usecase.AttachPty.Execute`'s two channels against the gRPC stream:

```go
func (s *Server) AttachPty(stream infrafleetv1.InfraFleetService_AttachPtyServer) error {
	ctx := stream.Context()
	tenantID, _ := tenant.FromContext(ctx) // mirror whatever this file's other handlers already use

	// First frame MUST be AttachToSession — read it before starting the
	// usecase, since Execute needs ptyID up front.
	first, err := stream.Recv()
	if err != nil {
		return apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInvalidArgument, "TERMINAL_ATTACH_FIRST_FRAME_REQUIRED", "first frame must be AttachToSession", err))
	}
	attach := first.GetAttach()
	if attach == nil || attach.GetPtyId() == "" {
		return apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInvalidArgument, "TERMINAL_ATTACH_FIRST_FRAME_REQUIRED", "first frame must be AttachToSession", nil))
	}
	ptyID := attach.GetPtyId()

	clientFrames := make(chan usecase.PtyClientFrame)
	serverFrames := make(chan usecase.PtyServerFrame)
	errCh := make(chan error, 1)

	go func() {
		errCh <- s.attachPty.Execute(ctx, tenantID, ptyID, clientFrames, serverFrames)
		close(serverFrames)
	}()

	// Pump inbound gRPC frames into clientFrames until the stream ends.
	go func() {
		defer close(clientFrames)
		for {
			frame, err := stream.Recv()
			if err != nil {
				return
			}
			switch f := frame.Frame.(type) {
			case *infrafleetv1.PtyClientFrame_Input:
				clientFrames <- usecase.PtyClientFrame{Input: &usecase.PtyInputFrame{Data: f.Input.GetData()}}
			case *infrafleetv1.PtyClientFrame_Resize:
				clientFrames <- usecase.PtyClientFrame{Resize: &usecase.PtyResizeFrame{Cols: f.Resize.GetCols(), Rows: f.Resize.GetRows()}}
			}
		}
	}()

	// Pump serverFrames out to the gRPC stream until it closes.
	for sf := range serverFrames {
		var out infrafleetv1.PtyServerFrame
		switch {
		case sf.Out != nil:
			out.Frame = &infrafleetv1.PtyServerFrame_Out{Out: &infrafleetv1.PtyOutput{Data: sf.Out.Data}}
		case sf.Exited != nil:
			out.Frame = &infrafleetv1.PtyServerFrame_Exited{Exited: &infrafleetv1.PtyExited{ExitCode: sf.Exited.ExitCode}}
		}
		if err := stream.Send(&out); err != nil {
			return err
		}
	}
	return apperrors.ToGRPCStatus(<-errCh)
}
```

Add `"time"`, `emptypb "google.golang.org/protobuf/types/known/emptypb"`,
and this service's `tenant` import (whatever `common/tenant` accessor this
file's existing handlers already call — check `RegisterDevServer`'s current
implementation for the exact call site and mirror it instead of guessing
`tenant.FromContext`) to `server.go`'s import block.

### Step 3 — `cmd/server/main.go`: construct and wire everything

Alongside the existing usecase construction block, add:

```go
streamLimiter := usecase.NewConnectionStreamLimiter()
spawnTerminalSessionUC := usecase.NewSpawnTerminalSession(connectionResolver, terminalSessionRepo, agentClient, cfg.ServerDeployment)
attachPtyUC := usecase.NewAttachPty(terminalSessionRepo, connectionResolver, agentClient, streamLimiter)
resizeTerminalSessionUC := usecase.NewResizeTerminalSession(terminalSessionRepo, connectionResolver, agentClient)
killTerminalSessionUC := usecase.NewKillTerminalSession(terminalSessionRepo, connectionResolver, agentClient)
stopTerminalProcessUC := usecase.NewStopTerminalProcess(terminalSessionRepo, connectionResolver, agentClient)
listTerminalSessionsUC := usecase.NewListTerminalSessions(terminalSessionRepo)
waitTerminalSessionUC := usecase.NewWaitTerminalSession(terminalSessionRepo, connectionResolver, agentClient)
focusTerminalSessionUC := usecase.NewFocusTerminalSession(terminalSessionRepo)
getTerminalAgentStatusUC := usecase.NewGetTerminalAgentStatus(terminalSessionRepo, connectionResolver, agentClient)
inspectTerminalProcessUC := usecase.NewInspectTerminalProcess(terminalSessionRepo, connectionResolver, agentClient)
```

`terminalSessionRepo := postgres.NewTerminalSessionRepository(pool)` —
construct it next to wherever this file already constructs
`postgres.NewDevServerRepository(pool)` or equivalent, reusing the same
`pool`. `connectionResolver`/`agentClient` are whatever variable names this
file already uses for `usecase.ConnectionResolver`/
`usecase.DevServerAgentClient` (constructed from `devserveragent.New(...)`
— reuse the existing instance, do not construct a second one).
`cfg.ServerDeployment` is a new config field this task adds to
`internal/config/config.go` (bool, env var e.g. `INFRA_FLEET_SERVER_DEPLOYMENT`,
default `true` per BUG-029's "no local shell" finding for this
deployment) — add it alongside this file's other `cfg.X` fields.

Pass all 10 new usecases into the existing `infrafleetgrpc.New(...)` call
(whatever that constructor's variable name is in this file), in the same
field order Step 2 added them to `Server`.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/infra-fleet-service
go build ./... && go vet ./...
```
