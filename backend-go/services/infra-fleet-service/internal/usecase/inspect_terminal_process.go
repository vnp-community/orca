package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// InspectTerminalProcess backs InspectTerminalProcessRequest — best-effort
// by design (see InspectTerminalProcessResponse's proto doc comment):
// Known=false is an honest "the agent couldn't answer", never a fabricated
// zero-valued pid/command/cwd. Same session-lookup-is-real-error /
// agent-call-degrades-gracefully split as GetTerminalAgentStatus — see that
// type's doc comment for the rationale.
type InspectTerminalProcess struct {
	sessions   TerminalSessionRepository
	resolver   ConnectionResolver
	devServers DevServerRepository
	agent      DevServerAgentClient
}

func NewInspectTerminalProcess(sessions TerminalSessionRepository, resolver ConnectionResolver, devServers DevServerRepository, agent DevServerAgentClient) *InspectTerminalProcess {
	return &InspectTerminalProcess{sessions: sessions, resolver: resolver, devServers: devServers, agent: agent}
}

func (uc *InspectTerminalProcess) Execute(ctx context.Context, ptyID string) (InspectProcessResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return InspectProcessResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	_, devServer, err := resolveTerminalSession(ctx, tenantID, ptyID, uc.sessions, uc.resolver, uc.devServers)
	if err != nil {
		return InspectProcessResult{}, err
	}

	result, err := uc.agent.InspectProcess(ctx, devServer, ptyID)
	if err != nil {
		return InspectProcessResult{Known: false}, nil
	}
	return result, nil
}
