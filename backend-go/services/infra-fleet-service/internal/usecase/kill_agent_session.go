package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// KillAgentSession — force teardown. Marks the session row 'stopped' even
// if the agent call fails, same discipline as KillTerminalSession.Execute.
type KillAgentSession struct {
	sessions      AgentSessionRepository
	resolver      ConnectionResolver
	agent         DevServerAgentClient
	writeActivity WriteActivityChecker // BR-AG-06 — nil-safe, see Execute
	clock         func() time.Time
}

func NewKillAgentSession(sessions AgentSessionRepository, resolver ConnectionResolver, agent DevServerAgentClient, writeActivity WriteActivityChecker) *KillAgentSession {
	return &KillAgentSession{sessions: sessions, resolver: resolver, agent: agent, writeActivity: writeActivity, clock: func() time.Time { return time.Now().UTC() }}
}

func (uc *KillAgentSession) Execute(ctx context.Context, sessionID, signal string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	session, devServer, err := resolveAgentSession(ctx, tenantID, sessionID, uc.sessions, uc.resolver)
	if err != nil {
		return err
	}
	if uc.writeActivity != nil {
		busy, checkErr := uc.writeActivity.HasInFlightWrite(ctx, session.WorktreeID)
		if checkErr == nil && busy {
			// BR-AG-06 — best-effort only. A checker error is NOT treated as
			// "busy" (fail open, not closed — an unreachable checker must
			// never block a user's explicit force-kill request).
			return apperrors.New(apperrors.KindFailedPrecondition, "INFRA_AGENT_KILL_BLOCKED_FILE_WRITE_IN_PROGRESS", "cannot kill agent while it is writing a file", nil)
		}
	}
	if signal == "" {
		signal = "SIGKILL"
	}
	agentErr := uc.agent.KillAgent(ctx, devServer, session.PtyID, signal)
	if err := uc.sessions.MarkStopped(ctx, tenantID, sessionID, uc.clock()); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_MARK_AGENT_SESSION_STOPPED_FAILED", "failed to mark agent session stopped", err)
	}
	if agentErr != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_KILL_FAILED", "agent session marked stopped, but the dev server agent failed to tear down the pty", agentErr)
	}
	return nil
}
