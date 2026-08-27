package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// StopAgentSession — BR-AG-05: graceful stop is Ctrl+C via agent.sendInput,
// not agent.kill (see KillAgentSession for full teardown). Mirrors
// StopTerminalProcess's shape, swapping SendSignal(SIGINT) for
// SendAgentInput('\x03') because that's the RPC that actually reaches an
// agent-spawned PTY — see agent_methods.go / TASK-AG-01-05's doc comment.
type StopAgentSession struct {
	sessions AgentSessionRepository
	resolver ConnectionResolver
	agent    DevServerAgentClient
}

func NewStopAgentSession(sessions AgentSessionRepository, resolver ConnectionResolver, agent DevServerAgentClient) *StopAgentSession {
	return &StopAgentSession{sessions: sessions, resolver: resolver, agent: agent}
}

func (uc *StopAgentSession) Execute(ctx context.Context, sessionID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	session, devServer, err := resolveAgentSession(ctx, tenantID, sessionID, uc.sessions, uc.resolver)
	if err != nil {
		return err
	}
	if err := uc.agent.SendAgentInput(ctx, devServer, session.PtyID, []byte{0x03}); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_STOP_FAILED", "failed to send graceful interrupt to agent pty", err)
	}
	// No status write here — the transition to 'stopped' is driven by the
	// agent's own agent.exited notification, consumed by TASK-AG-05's
	// classifier. Writing 'stopped' here would race a graceful exit that
	// takes a few hundred ms and briefly lie about state.
	return nil
}
