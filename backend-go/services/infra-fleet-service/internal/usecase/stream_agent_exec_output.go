package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// StreamAgentExecOutputInput mirrors the gRPC request 1:1, same rationale
// as RelayInput's own doc comment.
type StreamAgentExecOutputInput struct {
	ConnectionID string
	// DevServerID is the fallback for callers without a connection row; ignored when ConnectionID is set.
	DevServerID string
	StepID      string
}

// StreamAgentExecOutput is Relay's streaming counterpart for
// agent.execPrompt.output (TASK-AG-FLOWTASK-002/003) — resolves
// connectionId the same way Relay does, then subscribes to stepID's
// exec-output notifications instead of issuing a one-shot Exec call.
// task-service's SimpleExecutor is the one caller (TASK-AG-FLOWTASK-003):
// it calls the agent.execPrompt RPC itself via Relay (unary, blocks until
// the CLI exits), and consumes this stream CONCURRENTLY, over the same
// connectionId/stepId pair, purely to observe the in-flight run's output —
// this usecase never issues agent.execPrompt itself.
type StreamAgentExecOutput struct {
	resolver   ConnectionResolver
	devServers DevServerRepository
	agent      DevServerAgentClient
}

func NewStreamAgentExecOutput(resolver ConnectionResolver, devServers DevServerRepository, agent DevServerAgentClient) *StreamAgentExecOutput {
	return &StreamAgentExecOutput{resolver: resolver, devServers: devServers, agent: agent}
}

// Execute returns a receive-only event channel plus an unsubscribe func —
// unsubscribe MUST be called exactly once by the caller (typically via
// defer), same contract as DevServerAgentClient.StreamExecOutput itself.
func (uc *StreamAgentExecOutput) Execute(ctx context.Context, in StreamAgentExecOutputInput) (<-chan ExecOutputEvent, func(), error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if in.ConnectionID == "" && in.DevServerID == "" {
		return nil, nil, apperrors.New(apperrors.KindInvalidArgument, "INFRA_STREAM_EXEC_OUTPUT_NO_CONNECTION", "connectionId or devServerId is required", nil)
	}
	if in.StepID == "" {
		return nil, nil, apperrors.New(apperrors.KindInvalidArgument, "INFRA_STREAM_EXEC_OUTPUT_NO_STEP", "stepId is required", nil)
	}

	var devServer domain.DevServer
	if in.ConnectionID != "" {
		connected, ds, _, err := uc.resolver.ResolveConnection(ctx, tenantID, in.ConnectionID)
		if err != nil {
			return nil, nil, apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", err)
		}
		if !connected {
			return nil, nil, apperrors.New(apperrors.KindNotFound, "INFRA_CONNECTION_NOT_FOUND", "no dev server owns this connectionId", nil)
		}
		devServer = ds
	} else {
		// Same lookup as RelayByDevServer: the tenant-scoped Get is what stops one tenant observing another's dev server.
		ds, err := uc.devServers.Get(ctx, tenantID, in.DevServerID)
		if err != nil {
			return nil, nil, devServerLookupError(err)
		}
		if !uc.agent.IsConnected(ds.ID) {
			return nil, nil, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_DEV_SERVER_NOT_CONNECTED", "this dev server has no live agent connection right now", nil)
		}
		devServer = ds
	}

	out, unsubscribe, err := uc.agent.StreamExecOutput(ctx, devServer, in.StepID)
	if err != nil {
		return nil, nil, apperrors.New(apperrors.KindInternal, "INFRA_AGENT_STREAM_EXEC_OUTPUT_FAILED", "failed to subscribe to dev server agent exec output", err)
	}
	return out, unsubscribe, nil
}
