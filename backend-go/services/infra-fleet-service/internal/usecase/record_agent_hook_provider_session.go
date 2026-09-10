package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// RecordAgentHookProviderSession runs once per agent.hook notification
// carrying a providerSession field — subscribed alongside StreamPty's
// existing demux (TASK-AG-03-03), not a new transport. See TASK-AG-03-07
// for why this correlates by worktree rather than a direct session id.
type RecordAgentHookProviderSession struct {
	sessions AgentSessionRepository
}

func NewRecordAgentHookProviderSession(sessions AgentSessionRepository) *RecordAgentHookProviderSession {
	return &RecordAgentHookProviderSession{sessions: sessions}
}

func (uc *RecordAgentHookProviderSession) Handle(ctx context.Context, tenantID string, hook AgentHookEvent) error {
	if hook.ProviderSessionID == "" {
		return nil // most agent.hook events carry no providerSession — not every hook fires it
	}

	// TASK-AG-03-07: prefer the exact ptyId join once the agent build sends
	// one — closes the race the worktree fallback below cannot (a second
	// session for the same worktree+user having started between this hook
	// firing and it arriving). Falls back to worktree correlation only when
	// ptyId is empty (older agent builds mid-rollout).
	if hook.PtyID != "" {
		found, session, err := uc.sessions.GetByPtyID(ctx, tenantID, hook.PtyID)
		if err != nil {
			return err
		}
		if found {
			return uc.sessions.UpdateProviderSession(ctx, tenantID, session.ID, hook.ProviderSessionKey, hook.ProviderSessionID)
		}
		// No session for this exact ptyId (yet, or never) — fall through to
		// the worktree fallback rather than giving up outright.
	}

	found, session, err := uc.sessions.MostRecentActiveForWorktree(ctx, tenantID, hook.WorktreeID)
	if err != nil || !found {
		return err // nothing to attach this to yet — not an error worth failing the notification pump over
	}
	return uc.sessions.UpdateProviderSession(ctx, tenantID, session.ID, hook.ProviderSessionKey, hook.ProviderSessionID)
}

// Run subscribes to devServer's agent.hook stream (DevServerAgentClient.
// StreamAgentHooks, TASK-AG-03-03) and calls Handle for every notification
// until the stream closes or ctx is cancelled. One goroutine per registered
// dev server connection, started lazily by StartAgentSession the first time
// it resolves that dev server — see TASK-AG-03-06 for the idempotent
// per-dev-server-id registry that guards against starting it twice.
func (uc *RecordAgentHookProviderSession) Run(ctx context.Context, tenantID string, devServer domain.DevServer, agent DevServerAgentClient) {
	events, unsubscribe, err := agent.StreamAgentHooks(ctx, devServer)
	if err != nil {
		return
	}
	defer unsubscribe()
	for hook := range events {
		_ = uc.Handle(ctx, tenantID, hook) // best-effort — a correlation miss is not fatal, see Handle's doc comment
	}
}
