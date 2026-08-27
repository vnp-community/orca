package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// resolveAgentSession is resolveTerminalSession's (terminal_session_lookup.go)
// agent-session equivalent: fetch the AgentSession row, then resolve its
// stored ConnectionID into a live DevServer. Kept as its own function
// (rather than folded into resolveTerminalSession) because AgentSession and
// TerminalSession are different repository types, not because the
// resolution logic differs.
func resolveAgentSession(ctx context.Context, tenantID, sessionID string, sessions AgentSessionRepository, resolver ConnectionResolver) (domain.AgentSession, domain.DevServer, error) {
	found, session, err := sessions.Get(ctx, tenantID, sessionID)
	if err != nil {
		return domain.AgentSession{}, domain.DevServer{}, apperrors.New(apperrors.KindInternal, "INFRA_AGENT_SESSION_LOOKUP_FAILED", "failed to look up agent session", err)
	}
	if !found {
		return domain.AgentSession{}, domain.DevServer{}, apperrors.New(apperrors.KindNotFound, "INFRA_AGENT_SESSION_NOT_FOUND", "agent session not found", nil)
	}

	connected, devServer, _, err := resolver.ResolveConnection(ctx, tenantID, session.ConnectionID)
	if err != nil {
		return domain.AgentSession{}, domain.DevServer{}, apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", err)
	}
	if !connected {
		return domain.AgentSession{}, domain.DevServer{}, apperrors.New(apperrors.KindNotFound, "INFRA_CONNECTION_NOT_FOUND", "agent session's connection is no longer bound to a dev server", nil)
	}
	return session, devServer, nil
}
