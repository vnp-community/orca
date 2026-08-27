package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// RelayInput mirrors the gRPC request 1:1 by design, see
// register_dev_server.go's comment for the rationale. Params is already
// decoded from the wire's params_json — see adapter/grpc.Server.Relay.
type RelayInput struct {
	ConnectionID string
	Method       string
	Params       map[string]any
}

// Relay is the generic connectionId+method+params passthrough onto the Dev
// Server Agent execution plane, exposed as its own RPC so cross-service
// callers (git-gateway-service's RelayExecutor, workflow-service's step
// executors, api-gateway's wscompat) can reach a dev server without each
// reimplementing resolve-then-exec. ScanWorkspacePorts is the purpose-built
// precedent this generalizes — see that type's doc comment for the same
// "always relay when connectionId resolves, never silently swallow" rule,
// which applies here too: a resolve failure or a not-connected connectionId
// is a real error, not an empty/zero result.
type Relay struct {
	resolver ConnectionResolver
	agent    DevServerAgentClient
}

func NewRelay(resolver ConnectionResolver, agent DevServerAgentClient) *Relay {
	return &Relay{resolver: resolver, agent: agent}
}

func (uc *Relay) Execute(ctx context.Context, in RelayInput) (map[string]any, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if in.ConnectionID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "INFRA_RELAY_NO_CONNECTION", "connectionId is required", nil)
	}
	if in.Method == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "INFRA_RELAY_NO_METHOD", "method is required", nil)
	}

	connected, devServer, _, err := uc.resolver.ResolveConnection(ctx, tenantID, in.ConnectionID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", err)
	}
	if !connected {
		return nil, apperrors.New(apperrors.KindNotFound, "INFRA_CONNECTION_NOT_FOUND", "no dev server owns this connectionId", nil)
	}

	result, err := uc.agent.Exec(ctx, devServer, in.Method, in.Params)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "INFRA_AGENT_EXEC_FAILED", "failed to relay to dev server agent", err)
	}
	return result, nil
}
