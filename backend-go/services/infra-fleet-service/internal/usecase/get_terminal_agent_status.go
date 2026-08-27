package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// GetTerminalAgentStatus backs BOTH the terminal.agentStatus and
// terminal.isRunningAgent wscompat channels (see
// GetTerminalAgentStatusResponse's proto doc comment) — one RPC, since both
// questions ("is an agentic CLI running in this pane" / "richer: is it
// idle-and-ready") read the same underlying signal.
//
// A session-lookup failure (unknown pty_id, connection no longer bound) is a
// real error. An agent-level failure to answer (AgentStatus returning an
// error) is NOT — it degrades to the honest zero value
// {AgentRunning:false}, matching InspectTerminalProcess's "known=false, not
// a fabricated result" convention, since this is documented as best-effort
// (see DevServerAgentClient.AgentStatus's FLAGGED doc comment).
type GetTerminalAgentStatus struct {
	sessions TerminalSessionRepository
	resolver ConnectionResolver
	agent    DevServerAgentClient
}

func NewGetTerminalAgentStatus(sessions TerminalSessionRepository, resolver ConnectionResolver, agent DevServerAgentClient) *GetTerminalAgentStatus {
	return &GetTerminalAgentStatus{sessions: sessions, resolver: resolver, agent: agent}
}

func (uc *GetTerminalAgentStatus) Execute(ctx context.Context, ptyID string) (AgentStatusResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return AgentStatusResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	_, devServer, err := resolveTerminalSession(ctx, tenantID, ptyID, uc.sessions, uc.resolver)
	if err != nil {
		return AgentStatusResult{}, err
	}

	result, err := uc.agent.AgentStatus(ctx, devServer, ptyID)
	if err != nil {
		return AgentStatusResult{AgentRunning: false}, nil
	}
	return result, nil
}
