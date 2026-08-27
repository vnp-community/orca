package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// GetQueuedPrompt answers whether a prompt is currently queued for a pty
// (SOL-MB-03) — backs the mobile client's poll/refresh of a
// BR-MB-10-queued prompt while it waits for delivery.
type GetQueuedPrompt struct {
	sessions TerminalSessionRepository
	resolver ConnectionResolver
	queue    QueuedPromptRepository
}

func NewGetQueuedPrompt(sessions TerminalSessionRepository, resolver ConnectionResolver, queue QueuedPromptRepository) *GetQueuedPrompt {
	return &GetQueuedPrompt{sessions: sessions, resolver: resolver, queue: queue}
}

// Execute returns (hasQueuedPrompt, prompt, queuedAtUnixMs, err).
func (uc *GetQueuedPrompt) Execute(ctx context.Context, ptyID string) (bool, string, int64, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return false, "", 0, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if _, _, err := resolveTerminalSession(ctx, tenantID, ptyID, uc.sessions, uc.resolver); err != nil {
		return false, "", 0, err
	}
	prompt, ok, err := uc.queue.Get(ctx, ptyID)
	if err != nil {
		return false, "", 0, apperrors.New(apperrors.KindInternal, "INFRA_QUEUE_LOOKUP_FAILED", "failed to check queued prompt", err)
	}
	if !ok {
		return false, "", 0, nil
	}
	return true, prompt.Prompt, prompt.QueuedAt.UnixMilli(), nil
}
