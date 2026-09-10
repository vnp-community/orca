package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// RelayByDevServerInput mirrors the gRPC request 1:1, see Relay's own doc
// comment for the shared convention.
type RelayByDevServerInput struct {
	DevServerID string
	Method      string
	Params      map[string]any
}

// RelayByDevServer is Relay's devServerId-keyed counterpart — for callers
// that need to reach a dev server's agent BEFORE any infra.connections row
// exists for it (Relay's ConnectionResolver.ResolveConnection requires one).
// A dev server genuinely has no connections row until a repo/worktree is
// bound to it (usecase.CreateConnection) — but api-gateway's
// devServer.browseDir/onboarding.detectAgents need to reach the agent
// EXACTLY to let the user pick that first repo/worktree, a chicken-and-egg
// gap Relay's connectionId-only design can't close. This bypasses
// infra.connections entirely: DevServerRepository.Get already confirms
// tenant ownership, and DevServerAgentClient.Exec only ever needed the
// domain.DevServer value (host/mode/sshTargetId), never a connections row.
type RelayByDevServer struct {
	devServers DevServerRepository
	agent      DevServerAgentClient
}

func NewRelayByDevServer(devServers DevServerRepository, agent DevServerAgentClient) *RelayByDevServer {
	return &RelayByDevServer{devServers: devServers, agent: agent}
}

func (uc *RelayByDevServer) Execute(ctx context.Context, in RelayByDevServerInput) (map[string]any, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if in.DevServerID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "INFRA_RELAY_NO_DEV_SERVER", "devServerId is required", nil)
	}
	if in.Method == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "INFRA_RELAY_NO_METHOD", "method is required", nil)
	}

	devServer, err := uc.devServers.Get(ctx, tenantID, in.DevServerID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindNotFound, "INFRA_DEV_SERVER_NOT_FOUND", "dev server not found for this tenant", err)
	}

	if !uc.agent.IsConnected(devServer.ID) {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_DEV_SERVER_NOT_CONNECTED", "this dev server has no live agent connection right now", nil)
	}

	result, err := uc.agent.Exec(ctx, devServer, in.Method, in.Params)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "INFRA_AGENT_EXEC_FAILED", "failed to relay to dev server agent", err)
	}
	return result, nil
}
