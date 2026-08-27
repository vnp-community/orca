# TASK-CLI-02-03: `SendTerminalInput` + `GetTerminalScrollback` usecases

**From Solution:** SOL-CLI-02
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/send_terminal_input.go`, `get_terminal_scrollback.go`
**Depends on:** TASK-CLI-02-01
**Status:** [x] DONE — both usecases added (added an unexported, test-overridable `drainWindow` field on `GetTerminalScrollback` so its tests don't sleep the real 500ms), wired into `cmd/server/main.go`; all verify-listed cases pass (exact-byte write, unknown-pty `INFRA_TERMINAL_NOT_FOUND`, ordered chunk assembly, exit-event exclusion, bounded drain window).

---

## Context

`SendTerminalInput` is a stateless, unary sibling to `terminal.send` — the same `resolveTerminalSession` + `DevServerAgentClient.WritePty` path `AttachPty`'s stream-attached write already uses, just without requiring a held stream (a REST/CLI caller never attaches). `GetTerminalScrollback` gives callers a flat-text capture of recent pty output: rather than inventing a second recovery-buffer mechanism (the frontend's `channels_terminal_multiplex.go` buffer lives in `api-gateway`'s WS process memory and is not reachable from `infra-fleet-service`), this task collects the output via a short, bounded `StreamPty` subscribe-and-drain — a pragmatic reuse of the existing subscription primitive rather than a new persistent buffer.

## Changes to make

**1. `backend-go/services/infra-fleet-service/internal/usecase/send_terminal_input.go`:**

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// SendTerminalInput writes directly to a pty's stdin, bypassing AttachPty's
// stream — see SendTerminalInputRequest's proto doc comment for why a
// stateless REST/CLI caller needs this sibling to terminal.send.
type SendTerminalInput struct {
	sessions TerminalSessionRepository
	resolver ConnectionResolver
	agent    DevServerAgentClient
}

func NewSendTerminalInput(sessions TerminalSessionRepository, resolver ConnectionResolver, agent DevServerAgentClient) *SendTerminalInput {
	return &SendTerminalInput{sessions: sessions, resolver: resolver, agent: agent}
}

func (uc *SendTerminalInput) Execute(ctx context.Context, ptyID string, data []byte) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	_, devServer, err := resolveTerminalSession(ctx, tenantID, ptyID, uc.sessions, uc.resolver)
	if err != nil {
		return err
	}

	if err := uc.agent.WritePty(ctx, devServer, ptyID, data); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_WRITE_PTY_FAILED", "failed to write to pty", err)
	}
	return nil
}
```

**2. `backend-go/services/infra-fleet-service/internal/usecase/get_terminal_scrollback.go`:**

```go
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// scrollbackDrainWindow bounds how long GetTerminalScrollback waits to
// collect already-buffered output before returning — the agent's
// StreamPty replay delivers buffered chunks immediately on subscribe, so
// this window only needs to be long enough to drain them, not to observe
// new output.
const scrollbackDrainWindow = 500 * time.Millisecond

// GetTerminalScrollbackResult mirrors GetTerminalScrollbackResponse.
type GetTerminalScrollbackResult struct {
	Text      string
	Truncated bool
}

// GetTerminalScrollback subscribes to ptyID's replay buffer via StreamPty
// and concatenates every buffered output chunk delivered within
// scrollbackDrainWindow — a read-only view over the same replay mechanism
// AttachPty/WaitTerminalSession already subscribe to, not a new capture
// path. Truncated is always false today: the agent-side replay buffer's
// own retention bound is not currently surfaced through PtyEvent, so this
// is an honest "we don't know" rather than a fabricated true/false — see
// PtyEvent's doc comment for the gap.
type GetTerminalScrollback struct {
	sessions TerminalSessionRepository
	resolver ConnectionResolver
	agent    DevServerAgentClient
}

func NewGetTerminalScrollback(sessions TerminalSessionRepository, resolver ConnectionResolver, agent DevServerAgentClient) *GetTerminalScrollback {
	return &GetTerminalScrollback{sessions: sessions, resolver: resolver, agent: agent}
}

func (uc *GetTerminalScrollback) Execute(ctx context.Context, ptyID string) (GetTerminalScrollbackResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return GetTerminalScrollbackResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	_, devServer, err := resolveTerminalSession(ctx, tenantID, ptyID, uc.sessions, uc.resolver)
	if err != nil {
		return GetTerminalScrollbackResult{}, err
	}

	drainCtx, cancel := context.WithTimeout(ctx, scrollbackDrainWindow)
	defer cancel()

	events, unsubscribe, err := uc.agent.StreamPty(drainCtx, devServer, ptyID)
	if err != nil {
		return GetTerminalScrollbackResult{}, apperrors.New(apperrors.KindInternal, "INFRA_AGENT_STREAM_PTY_FAILED", "failed to subscribe to pty output", err)
	}
	defer unsubscribe()

	var buf []byte
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return GetTerminalScrollbackResult{Text: string(buf)}, nil
			}
			if !ev.Exited {
				buf = append(buf, ev.Data...)
			}
		case <-drainCtx.Done():
			return GetTerminalScrollbackResult{Text: string(buf)}, nil
		}
	}
}
```

Wire both into `internal/adapter/grpc/server.go`'s `Server` struct/`New(...)` and `cmd/server/main.go`'s DI construction, matching `waitTerminalSessionUC`'s existing construction pattern.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run 'TestSendTerminalInput|TestGetTerminalScrollback' -v
```

Expected: `send_terminal_input_test.go` — writes reach the fake `DevServerAgentClient.WritePty` call with the exact bytes given, no framing added; unknown `pty_id` returns `INFRA_TERMINAL_NOT_FOUND` without calling `WritePty`. `get_terminal_scrollback_test.go` — a fake `DevServerAgentClient.StreamPty` emitting buffered chunks assembles them in order; an exit event is excluded from the text; the drain window bounds the call (test uses a short window override or a fake clock, not a real 500ms sleep).
