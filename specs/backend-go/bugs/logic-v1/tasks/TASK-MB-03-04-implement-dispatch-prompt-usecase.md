# TASK-MB-03-04: Implement `DispatchPrompt` usecase (gate/queue/overwrite)

**From Solution:** SOL-MB-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/dispatch_prompt.go`, `backend-go/services/infra-fleet-service/internal/usecase/get_queued_prompt.go`
**Depends on:** TASK-MB-03-02, TASK-MB-03-03, TASK-MB-02-02 (`ReadyForInput` quiescence signal + shared `liveStates` registry)
**Status:** `[ ]` TODO

---

## Context

`DispatchPrompt` is the one decision point BR-MB-09/10/12 reduce to. It
reuses `DevServerAgentClient.WritePty` (the same primitive `RouteTerminalWrite`/
`terminal.send` already call) rather than forking a second PTY-write path,
and consumes SOL-MB-02's quiescence-based `ReadyForInput` — a real
cross-solution dependency.

## Changes to make

`backend-go/services/infra-fleet-service/internal/usecase/dispatch_prompt.go`:

```go
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type DispatchPromptInput struct {
	PtyID     string
	Prompt    string
	Overwrite bool
	DeviceID  string
}

type DispatchOutcome struct {
	Outcome         string // "INJECTED_IMMEDIATELY" | "QUEUED" | "REJECTED_NEEDS_CONFIRMATION"
	ExistingPreview string
}

type DispatchPrompt struct {
	sessions TerminalSessionRepository
	resolver ConnectionResolver
	agent    DevServerAgentClient
	queue    QueuedPromptRepository
}

func NewDispatchPrompt(sessions TerminalSessionRepository, resolver ConnectionResolver, agent DevServerAgentClient, queue QueuedPromptRepository) *DispatchPrompt {
	return &DispatchPrompt{sessions: sessions, resolver: resolver, agent: agent, queue: queue}
}

func (uc *DispatchPrompt) Execute(ctx context.Context, in DispatchPromptInput) (DispatchOutcome, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return DispatchOutcome{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	prompt, err := domain.NewQueuedPrompt(in.PtyID, tenantID, in.Prompt, in.DeviceID, time.Now()) // BR-MB-11
	if err != nil {
		return DispatchOutcome{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_PROMPT_INVALID", err.Error(), err)
	}

	session, devServer, err := resolveTerminalSession(ctx, tenantID, in.PtyID, uc.sessions, uc.resolver)
	if err != nil {
		return DispatchOutcome{}, err
	}

	status, _ := uc.agent.AgentStatus(ctx, devServer, in.PtyID) // best-effort, same degrade-to-false convention as GetTerminalAgentStatus

	existing, hasExisting, err := uc.queue.Get(ctx, in.PtyID)
	if err != nil {
		return DispatchOutcome{}, apperrors.New(apperrors.KindInternal, "INFRA_QUEUE_LOOKUP_FAILED", "failed to check queued prompt", err)
	}
	if hasExisting && !in.Overwrite { // BR-MB-12
		return DispatchOutcome{Outcome: "REJECTED_NEEDS_CONFIRMATION", ExistingPreview: preview(existing.Prompt, 200)}, nil
	}

	idle := !status.AgentRunning || status.ReadyForInput // BR-MB-09: "idle" (no agent) or "waiting" (ready) both qualify
	if idle {
		if err := uc.agent.WritePty(ctx, devServer, in.PtyID, []byte(prompt.Prompt)); err != nil {
			return DispatchOutcome{}, apperrors.New(apperrors.KindInternal, "INFRA_DISPATCH_WRITE_FAILED", "failed to write prompt to pty", err)
		}
		_ = uc.queue.Delete(ctx, in.PtyID) // clears any stale queued entry now that we injected directly
		_ = session // session currently unused beyond resolveTerminalSession's validation — keep for future audit logging
		return DispatchOutcome{Outcome: "INJECTED_IMMEDIATELY"}, nil
	}

	// BR-MB-10: agent running — queue instead of dropping or rejecting.
	if err := uc.queue.Upsert(ctx, prompt); err != nil {
		return DispatchOutcome{}, apperrors.New(apperrors.KindInternal, "INFRA_QUEUE_UPSERT_FAILED", "failed to queue prompt", err)
	}
	return DispatchOutcome{Outcome: "QUEUED"}, nil
}

func preview(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
```

`backend-go/services/infra-fleet-service/internal/usecase/get_queued_prompt.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
)

type GetQueuedPrompt struct {
	sessions TerminalSessionRepository
	resolver ConnectionResolver
	queue    QueuedPromptRepository
}

func NewGetQueuedPrompt(sessions TerminalSessionRepository, resolver ConnectionResolver, queue QueuedPromptRepository) *GetQueuedPrompt {
	return &GetQueuedPrompt{sessions: sessions, resolver: resolver, queue: queue}
}

func (uc *GetQueuedPrompt) Execute(ctx context.Context, ptyID string) (bool, string, int64, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return false, "", 0, err
	}
	if _, _, err := resolveTerminalSession(ctx, tenantID, ptyID, uc.sessions, uc.resolver); err != nil {
		return false, "", 0, err
	}
	prompt, ok, err := uc.queue.Get(ctx, ptyID)
	if err != nil || !ok {
		return false, "", 0, err
	}
	return true, prompt.Prompt, prompt.QueuedAt.UnixMilli(), nil
}
```

**Draining the queue** — TASK-MB-02-02's `GetTerminalAgentStatus`
ready-transition branch (`if result.ReadyForInput && !wasReady`) additionally
calls `uc.queue.GetAndDelete(ctx, ptyID)` (TASK-MB-03-03's atomic
delete-and-return) on the same transition and, if a row exists, calls
`uc.agent.WritePty(ctx, devServer, ptyID, []byte(prompt.Prompt))` — this is
the mechanism that actually delivers a BR-MB-10-queued prompt once the
agent frees up, not a separate poller. Add this as a follow-up edit to
`GetTerminalAgentStatus.Execute` in this task (it shares the same file
TASK-MB-02-02 touches — coordinate to avoid overlapping edits).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/... && go vet ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run 'DispatchPrompt|GetQueuedPrompt'
```

Test cases: agent not running → `INJECTED_IMMEDIATELY`, `WritePty` called,
no queue row written (BR-MB-09). Agent running, `ReadyForInput=false`, no
existing entry → `QUEUED`, `WritePty` NOT called (BR-MB-10). Agent running,
existing queued prompt, `overwrite=false` → `REJECTED_NEEDS_CONFIRMATION`,
existing row unchanged, preview matches first 200 chars (BR-MB-12). Same
scenario with `overwrite=true` → row replaced. Ready-transition drain:
queued prompt present when agent transitions running→ready → `WritePty`
called exactly once via `GetAndDelete`, queue row gone afterward (regression
guard against double-delivery race).
