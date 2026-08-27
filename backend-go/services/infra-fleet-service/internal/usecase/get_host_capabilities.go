package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// HostCapabilitiesResult mirrors GetHostCapabilitiesResponse's fields.
type HostCapabilitiesResult struct {
	WslAvailable     bool
	WslDistros       []string
	PwshAvailable    bool
	GitBashAvailable bool
}

// GetHostCapabilities implements the host.* per-target usecase (TASK-070):
// resolve the caller's actual dev server via ConnectionResolver and relay a
// real WSL/pwsh/git-bash probe to it, replacing the old backend's bug
// (BUG-011) of only ever probing its own process host.
//
// connectionID == "" (or one that doesn't resolve to a live dev server)
// answers the same honest false/[] TASK-068's stub gives directly — a
// backend-go container has none of these three tools meaningful on it
// regardless (10-deployment-infrastructure.md's deployment model) — computed
// here instead so callers get one consistent code path regardless of
// target, per this task's own usecase sketch's "conn == nil" branch. This is
// the one host.* case that DOES have a local-honest-answer fallback, unlike
// EmulatorRelay's connectionId-required rule — see wscompat's
// registerHostRelayChannels doc comment for the wire-level mirror of this
// distinction.
//
// agent/ has no host-capability-probing method reachable from
// infra-fleet-service today (confirmed absent this pass — the only
// WSL/pwsh/git-bash probing code in agent/src/relay/ lives on
// preflight-handler.ts's RelayDispatcher, a different transport than
// agent-rpc-dispatch.ts's JSON-RPC-over-WebSocket surface this service's
// devserveragent client actually speaks). A resolved connection therefore
// always gets back a real JSON-RPC "method not found", translated into a
// typed, permanent apperrors.KindFailedPrecondition result. The moment
// agent/ adds a host.capabilities handler to agent-rpc-dispatch.ts, this
// call starts working with zero further backend-go changes.
type GetHostCapabilities struct {
	resolver ConnectionResolver
	agent    DevServerAgentClient
}

func NewGetHostCapabilities(resolver ConnectionResolver, agent DevServerAgentClient) *GetHostCapabilities {
	return &GetHostCapabilities{resolver: resolver, agent: agent}
}

func (uc *GetHostCapabilities) Execute(ctx context.Context, connectionID string) (HostCapabilitiesResult, error) {
	if connectionID == "" {
		// No connectionId = the caller's active target is this backend-go
		// replica's own host — same honest false/[] answer TASK-068 gives
		// directly, see type doc comment.
		return HostCapabilitiesResult{WslDistros: []string{}}, nil
	}

	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return HostCapabilitiesResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	connected, devServer, _, resolveErr := uc.resolver.ResolveConnection(ctx, tenantID, connectionID)
	if resolveErr != nil {
		return HostCapabilitiesResult{}, apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", resolveErr)
	}
	if !connected {
		// Same "no connection bound" honest answer as the empty-connectionId
		// case above — not an error, see type doc comment.
		return HostCapabilitiesResult{WslDistros: []string{}}, nil
	}

	result, execErr := uc.agent.Exec(ctx, devServer, "host.capabilities", nil)
	if execErr != nil {
		if errors.Is(execErr, domain.ErrAgentMethodNotFound) {
			return HostCapabilitiesResult{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_HOST_CAPABILITIES_UNSUPPORTED",
				"this dev server's agent build does not support host capability probing — see specs/backend-go/bugs/missing-v1/tasks/TASK-070", execErr)
		}
		return HostCapabilitiesResult{}, apperrors.New(apperrors.KindInternal, "INFRA_AGENT_EXEC_FAILED", "failed to relay host.capabilities to dev server agent", execErr)
	}
	return decodeHostCapabilities(result), nil
}

func decodeHostCapabilities(result map[string]any) HostCapabilitiesResult {
	out := HostCapabilitiesResult{WslDistros: []string{}}
	if v, ok := result["wslAvailable"].(bool); ok {
		out.WslAvailable = v
	}
	if v, ok := result["pwshAvailable"].(bool); ok {
		out.PwshAvailable = v
	}
	if v, ok := result["gitBashAvailable"].(bool); ok {
		out.GitBashAvailable = v
	}
	if raw, ok := result["wslDistros"].([]any); ok {
		distros := make([]string, 0, len(raw))
		for _, d := range raw {
			if s, ok := d.(string); ok {
				distros = append(distros, s)
			}
		}
		out.WslDistros = distros
	}
	return out
}
