package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// KillTerminalSession backs terminal.close — full teardown (pty.destroy),
// distinct from StopTerminalProcess's foreground-interrupt-only contract.
// Marks the session row closed even if the agent call fails, so a dev
// server that has already gone away (or a pty the agent no longer knows
// about) doesn't leave a permanently "open" row behind — mirrors this
// codebase's general "the persisted record must not lie" discipline.
type KillTerminalSession struct {
	sessions TerminalSessionRepository
	resolver ConnectionResolver
	agent    DevServerAgentClient
}

func NewKillTerminalSession(sessions TerminalSessionRepository, resolver ConnectionResolver, agent DevServerAgentClient) *KillTerminalSession {
	return &KillTerminalSession{sessions: sessions, resolver: resolver, agent: agent}
}

func (uc *KillTerminalSession) Execute(ctx context.Context, ptyID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	_, devServer, err := resolveTerminalSession(ctx, tenantID, ptyID, uc.sessions, uc.resolver)
	if err != nil {
		return err
	}

	agentErr := uc.agent.KillPty(ctx, devServer, ptyID, true)

	if err := uc.sessions.Close(ctx, tenantID, ptyID, time.Now().UTC()); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_CLOSE_TERMINAL_SESSION_FAILED", "failed to mark terminal session closed", err)
	}
	if agentErr != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_KILL_PTY_FAILED", "terminal session marked closed, but the dev server agent failed to tear down the pty", agentErr)
	}
	return nil
}
