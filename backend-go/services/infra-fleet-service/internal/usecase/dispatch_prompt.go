package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// previewLength caps ExistingQueuedPromptPreview's size — a caller only
// needs enough of the existing prompt to recognize it and confirm the
// overwrite, not the full (up to 10,000-char) text.
const previewLength = 200

type DispatchPromptInput struct {
	PtyID     string
	Prompt    string
	Overwrite bool
	DeviceID  string
}

// DispatchOutcome carries DispatchPromptResponse's fields — Outcome is one
// of "INJECTED_IMMEDIATELY" | "QUEUED" | "REJECTED_NEEDS_CONFIRMATION",
// mirroring the proto enum's string names so adapter/grpc can translate via
// the generated *_value map without a second mapping table here.
type DispatchOutcome struct {
	Outcome         string
	ExistingPreview string
}

// DispatchPrompt is the ONE decision point BR-MB-09/10/12 all reduce to
// (SOL-MB-03): gate on agent readiness, queue if the agent is running,
// require explicit confirmation to overwrite an existing queued prompt. It
// reuses DevServerAgentClient.WritePty — the same primitive
// RouteTerminalWrite/terminal.send already call — rather than forking a
// second PTY-write path, and consumes TASK-MB-02-02's quiescence-based
// ReadyForInput signal via AgentStatus.
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

	_, devServer, err := resolveTerminalSession(ctx, tenantID, in.PtyID, uc.sessions, uc.resolver)
	if err != nil {
		return DispatchOutcome{}, err
	}

	status, _ := uc.agent.AgentStatus(ctx, devServer, in.PtyID) // best-effort, same degrade-to-false convention as GetTerminalAgentStatus

	existing, hasExisting, err := uc.queue.Get(ctx, in.PtyID)
	if err != nil {
		return DispatchOutcome{}, apperrors.New(apperrors.KindInternal, "INFRA_QUEUE_LOOKUP_FAILED", "failed to check queued prompt", err)
	}
	if hasExisting && !in.Overwrite { // BR-MB-12
		return DispatchOutcome{Outcome: "REJECTED_NEEDS_CONFIRMATION", ExistingPreview: preview(existing.Prompt, previewLength)}, nil
	}

	idle := !status.AgentRunning || status.ReadyForInput // BR-MB-09: "idle" (no agent) or "waiting" (ready) both qualify
	if idle {
		if err := uc.agent.WritePty(ctx, devServer, in.PtyID, []byte(prompt.Prompt)); err != nil {
			return DispatchOutcome{}, apperrors.New(apperrors.KindInternal, "INFRA_DISPATCH_WRITE_FAILED", "failed to write prompt to pty", err)
		}
		_ = uc.queue.Delete(ctx, in.PtyID) // clears any stale queued entry now that we injected directly
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
