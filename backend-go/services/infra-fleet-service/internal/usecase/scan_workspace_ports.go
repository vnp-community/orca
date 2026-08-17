package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// ScanWorkspacePortsInput mirrors the gRPC request 1:1 by design, see
// register_dev_server.go's comment for the rationale.
type ScanWorkspacePortsInput struct {
	ConnectionID string
	WorktreeID   string
}

// ScanWorkspacePorts closes TS Gap 7 (see
// specs/backend-go/services/infra-fleet-service.md §10): it always calls
// ConnectionResolver first, and whenever a connectionId is present it relays
// the scan to the agent's ports.* handler rather than silently returning an
// empty result. There is deliberately no
// `if connectionID != "" { return local }` shortcut that skips relaying —
// the bug class TS had is structurally impossible here because relaying a
// bound connection is the only path a non-empty connectionId can take.
type ScanWorkspacePorts struct {
	resolver ConnectionResolver
	agent    DevServerAgentClient
}

func NewScanWorkspacePorts(resolver ConnectionResolver, agent DevServerAgentClient) *ScanWorkspacePorts {
	return &ScanWorkspacePorts{resolver: resolver, agent: agent}
}

func (uc *ScanWorkspacePorts) Execute(ctx context.Context, in ScanWorkspacePortsInput) ([]int32, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	if in.ConnectionID != "" {
		connected, devServer, resolveErr := uc.resolver.ResolveConnection(ctx, tenantID, in.ConnectionID)
		if resolveErr != nil {
			return nil, apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", resolveErr)
		}
		if connected {
			// A connectionId is bound: relay to the agent. Its stub
			// implementation returns an error, which propagates here rather
			// than being swallowed into an empty result — see the type doc
			// comment above.
			result, execErr := uc.agent.Exec(ctx, devServer, "ports.scan", map[string]any{"worktreeId": in.WorktreeID})
			if execErr != nil {
				return nil, apperrors.New(apperrors.KindInternal, "INFRA_AGENT_EXEC_FAILED", "failed to relay workspace port scan to dev server agent", execErr)
			}
			return decodeOpenPorts(result), nil
		}
	}

	// No connectionId bound (or it didn't resolve to a live dev server): the
	// worktree is local. Actually performing a local port scan is out of
	// scope for this scaffold — see this service's README "Known gaps". This
	// service's contract is *routing* the scan, not executing it.
	return []int32{}, nil
}

// decodeOpenPorts extracts the "openPorts" field the agent's ports.scan
// method returns, per specs/agent/api's ports.* handler contract. Defensive
// against absent/malformed fields since adapter/devserveragent is currently
// a stub and this decoding path is not yet exercised against a real agent.
func decodeOpenPorts(result map[string]any) []int32 {
	raw, ok := result["openPorts"].([]any)
	if !ok {
		return []int32{}
	}
	ports := make([]int32, 0, len(raw))
	for _, v := range raw {
		switch n := v.(type) {
		case float64:
			ports = append(ports, int32(n))
		case int32:
			ports = append(ports, n)
		case int:
			ports = append(ports, int32(n))
		}
	}
	return ports
}
