# TASK-182: Implement `SpawnTerminalSession` + `AttachPty` usecases (the core create+stream flow)

**From Solution:** SOL-029
**Priority:** P0 — the heart of the `AttachPty` design; TASK-185's gRPC wiring and TASK-186's `wscompat` bridge both depend on this
**Service:** `infra-fleet-service`
**File:** `internal/usecase/spawn_terminal_session.go` (new), `internal/usecase/attach_pty.go` (new), `internal/usecase/connection_stream_limiter.go` (new)
**Depends on:** TASK-181
**Status:** `[ ]` TODO

---

## Context

`SpawnTerminalSession` creates the agent-side PTY and the bookkeeping row
(with a defined rollback if bookkeeping fails after the agent-side spawn
succeeds). `AttachPty` is the usecase behind the streaming RPC's gRPC
handler (wired in TASK-185): it bridges `DevServerAgentClient.StreamPty`
(agent → here) with a caller-provided channel of client frames (here →
agent), enforcing `infra-fleet-service.md` §8's
`MAX_CONCURRENT_STREAMS = 16` cap via a small per-`connectionId` semaphore.

## Changes to make

### `internal/usecase/connection_stream_limiter.go` (new)

```go
package usecase

import (
	"fmt"
	"sync"
)

// maxConcurrentPtyStreamsPerConnection mirrors infra-fleet-service.md §8's
// MAX_CONCURRENT_STREAMS = 16 cap, applied per connectionId so one runaway
// frontend opening many panes against one dev server doesn't degrade the
// underlying agent connection for every other session sharing it.
const maxConcurrentPtyStreamsPerConnection = 16

// ConnectionStreamLimiter is a per-connectionId semaphore bounding
// concurrent AttachPty streams.
type ConnectionStreamLimiter struct {
	mu     sync.Mutex
	counts map[string]int
}

func NewConnectionStreamLimiter() *ConnectionStreamLimiter {
	return &ConnectionStreamLimiter{counts: make(map[string]int)}
}

// Acquire reserves one stream slot for connectionID. The returned release
// func MUST be called (typically via defer) once the stream ends.
func (l *ConnectionStreamLimiter) Acquire(connectionID string) (release func(), err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.counts[connectionID] >= maxConcurrentPtyStreamsPerConnection {
		return nil, fmt.Errorf("too many concurrent terminal streams for connection %q (max %d)", connectionID, maxConcurrentPtyStreamsPerConnection)
	}
	l.counts[connectionID]++
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.counts[connectionID]--
		if l.counts[connectionID] <= 0 {
			delete(l.counts, connectionID)
		}
	}, nil
}
```

### `internal/usecase/spawn_terminal_session.go` (new)

```go
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type SpawnTerminalSessionInput struct {
	ConnectionID string
	Cwd          string
	Shell        string
	Cols, Rows   int32
}

// SpawnTerminalSession creates the agent-side PTY, then records bookkeeping
// — if the bookkeeping write fails, the already-spawned agent-side PTY is
// killed rather than left orphaned.
type SpawnTerminalSession struct {
	connections       ConnectionResolver
	sessions          TerminalSessionRepository
	agent             DevServerAgentClient
	clock             func() time.Time
	// serverDeployment gates the "no local shell" rejection — a config
	// flag (wired from cmd/server/main.go), not a hardcoded rejection, so
	// local/dev mode isn't permanently cut off. See BUG-029's
	// server-pty-controller.ts "no local shell" finding.
	serverDeployment bool
}

func NewSpawnTerminalSession(connections ConnectionResolver, sessions TerminalSessionRepository, agent DevServerAgentClient, serverDeployment bool) *SpawnTerminalSession {
	return &SpawnTerminalSession{connections: connections, sessions: sessions, agent: agent, clock: time.Now, serverDeployment: serverDeployment}
}

func (uc *SpawnTerminalSession) Execute(ctx context.Context, tenantID string, in SpawnTerminalSessionInput) (domain.TerminalSession, error) {
	if in.ConnectionID == "" && uc.serverDeployment {
		return domain.TerminalSession{}, apperrors.New(apperrors.KindFailedPrecondition,
			"TERMINAL_NO_LOCAL_SHELL", "this server only supports Dev Server- or SSH-backed terminals", nil)
	}

	connected, devServer, _, err := uc.connections.ResolveConnection(ctx, tenantID, in.ConnectionID)
	if err != nil || !connected {
		return domain.TerminalSession{}, apperrors.New(apperrors.KindNotFound, "TERMINAL_CONNECTION_NOT_FOUND", "connection not found", err)
	}

	ptyID, err := uc.agent.SpawnPty(ctx, devServer, in.Cwd, in.Shell, in.Cols, in.Rows)
	if err != nil {
		return domain.TerminalSession{}, apperrors.New(apperrors.KindInternal, "TERMINAL_SPAWN_FAILED", "failed to spawn terminal session", err)
	}

	now := uc.clock()
	session := domain.TerminalSession{PtyID: ptyID, ConnectionID: in.ConnectionID, Cwd: in.Cwd, CreatedAt: now, LastActiveAt: now}
	if err := uc.sessions.Create(ctx, session); err != nil {
		// Session exists agent-side but bookkeeping failed — kill it rather
		// than leaking an orphaned PTY the fleet has no record of.
		_ = uc.agent.KillPty(ctx, devServer, ptyID)
		return domain.TerminalSession{}, apperrors.New(apperrors.KindInternal, "TERMINAL_BOOKKEEPING_FAILED", "failed to record terminal session", err)
	}
	return session, nil
}
```

`ConnectionResolver.ResolveConnection`'s real signature
(`internal/usecase/ports.go:70`) is
`(ctx, tenantID, connectionID) (connected bool, devServer domain.DevServer, conn domain.Connection, err error)`
— used as-is above (`connected=false` with `err=nil` is also treated as
"not found" here, matching that method's own "no dev server owns this
connectionId" contract).

### `internal/usecase/attach_pty.go` (new)

```go
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

// PtyClientFrame/PtyServerFrame mirror the proto oneofs (TASK-180) at the
// usecase layer, so this package doesn't import proto types directly (per
// 03-clean-architecture-guidelines.md's inbound-adapter-does-translation
// rule) — internal/adapter/grpc's AttachPty handler (TASK-185) converts
// to/from these.
type PtyClientFrame struct {
	Input  *PtyInputFrame
	Resize *PtyResizeFrame
}
type PtyInputFrame struct{ Data []byte }
type PtyResizeFrame struct{ Cols, Rows int32 }

type PtyServerFrame struct {
	Out    *PtyOutputFrame
	Exited *PtyExitedFrame
}
type PtyOutputFrame struct{ Data []byte }
type PtyExitedFrame struct{ ExitCode int32 }

// AttachPty is the usecase behind the AttachPty streaming RPC's gRPC
// handler (TASK-185). Bridges DevServerAgentClient.StreamPty (agent ->
// here) with a caller-provided channel of client frames (here -> agent),
// enforcing infra-fleet-service.md §8's MAX_CONCURRENT_STREAMS cap.
type AttachPty struct {
	sessions    TerminalSessionRepository
	connections ConnectionResolver
	agent       DevServerAgentClient
	limiter     *ConnectionStreamLimiter
	clock       func() time.Time
}

func NewAttachPty(sessions TerminalSessionRepository, connections ConnectionResolver, agent DevServerAgentClient, limiter *ConnectionStreamLimiter) *AttachPty {
	return &AttachPty{sessions: sessions, connections: connections, agent: agent, limiter: limiter, clock: time.Now}
}

// Execute runs until ctx is cancelled, clientFrames closes, or the agent's
// stream ends/exits — whichever comes first. serverFrames is written to by
// this method; the caller (the gRPC handler) reads from it and sends each
// frame over the stream.
func (uc *AttachPty) Execute(ctx context.Context, tenantID, ptyID string, clientFrames <-chan PtyClientFrame, serverFrames chan<- PtyServerFrame) error {
	session, found, err := uc.sessions.Get(ctx, tenantID, ptyID)
	if err != nil || !found {
		return apperrors.New(apperrors.KindNotFound, "TERMINAL_SESSION_NOT_FOUND", "terminal session not found", err)
	}

	release, err := uc.limiter.Acquire(session.ConnectionID)
	if err != nil {
		return apperrors.New(apperrors.KindResourceExhausted, "TERMINAL_TOO_MANY_STREAMS", "too many concurrent terminal streams for this connection", err)
	}
	defer release()

	connected, devServer, _, err := uc.connections.ResolveConnection(ctx, tenantID, session.ConnectionID)
	if err != nil || !connected {
		return apperrors.New(apperrors.KindNotFound, "TERMINAL_CONNECTION_NOT_FOUND", "connection not found", err)
	}

	events, err := uc.agent.StreamPty(ctx, devServer, ptyID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "TERMINAL_STREAM_FAILED", "failed to open pty stream", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case frame, ok := <-clientFrames:
			if !ok {
				return nil
			}
			switch {
			case frame.Input != nil:
				if err := uc.agent.WritePty(ctx, devServer, ptyID, frame.Input.Data); err != nil {
					return apperrors.New(apperrors.KindInternal, "TERMINAL_WRITE_FAILED", "failed to write to terminal", err)
				}
				_ = uc.sessions.Touch(ctx, ptyID, uc.clock())
			case frame.Resize != nil:
				_ = uc.agent.ResizePty(ctx, devServer, ptyID, frame.Resize.Cols, frame.Resize.Rows)
			}
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			if ev.Exited {
				serverFrames <- PtyServerFrame{Exited: &PtyExitedFrame{ExitCode: ev.ExitCode}}
				_ = uc.sessions.MarkClosed(ctx, ptyID, uc.clock())
				return nil
			}
			serverFrames <- PtyServerFrame{Out: &PtyOutputFrame{Data: ev.Output}}
		}
	}
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/infra-fleet-service
go build ./internal/usecase/...
```

Expected: build fails only on missing `DevServerAgentClient`
implementations for any existing fakes in this package's own tests (none
yet reference the new PTY methods) — no failures otherwise. TASK-183
supplies the real implementation.
