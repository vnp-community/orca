package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// RelayStreamInput mirrors RelayInput — see that type's doc comment for the
// shared connectionId+method+params shape.
type RelayStreamInput struct {
	ConnectionID string
	Method       string
	Params       map[string]any
}

// RelayStream is Relay's server-streaming counterpart (TASK-PW-03-08,
// SOL-PW-03): the same resolve-then-relay dispatch, but for agent methods
// that reply with multiple frames instead of one (currently only
// git-gateway-service's PushStream/PullStream, relaying to the agent's
// git.execStream). Exposed as its own RPC/usecase rather than folded into
// Relay's own unary shape, mirroring why Relay itself is generic — see that
// type's doc comment.
type RelayStream struct {
	resolver ConnectionResolver
	agent    DevServerAgentClient
}

func NewRelayStream(resolver ConnectionResolver, agent DevServerAgentClient) *RelayStream {
	return &RelayStream{resolver: resolver, agent: agent}
}

// Execute resolves in.ConnectionID's owning dev server, then relays
// in.Method/in.Params to it and calls sink once per streamed frame, in
// order. Execute returns once the agent's terminal frame is observed (sink
// stops being called after that) or ctx is cancelled — a resolve failure or
// a not-connected connectionId is a real error, matching Relay.Execute's
// own "never silently swallow" rule.
func (uc *RelayStream) Execute(ctx context.Context, in RelayStreamInput, sink func(map[string]any) error) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if in.ConnectionID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "INFRA_RELAY_NO_CONNECTION", "connectionId is required", nil)
	}
	if in.Method == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "INFRA_RELAY_NO_METHOD", "method is required", nil)
	}

	connected, devServer, _, err := uc.resolver.ResolveConnection(ctx, tenantID, in.ConnectionID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", err)
	}
	if !connected {
		return apperrors.New(apperrors.KindNotFound, "INFRA_CONNECTION_NOT_FOUND", "no dev server owns this connectionId", nil)
	}

	frames, unsubscribe, err := uc.agent.ExecStream(ctx, devServer, in.Method, in.Params)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_EXECSTREAM_FAILED", "failed to relay stream to dev server agent", err)
	}
	defer unsubscribe()

	for {
		select {
		case frame, ok := <-frames:
			if !ok {
				return nil
			}
			if err := sink(frame); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
