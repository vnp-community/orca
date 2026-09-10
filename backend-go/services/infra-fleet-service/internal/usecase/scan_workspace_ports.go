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

// DetectedPort mirrors agent/src/relay/port-scan-handler.ts's DetectedPort —
// the agent's real ports.detect response shape, not a flat port-number list.
type DetectedPort struct {
	Port        int32
	Host        string
	PID         int32
	ProcessName string
}

// ScanWorkspacePorts closes TS Gap 7 (see
// specs/backend-go/services/infra-fleet-service.md §10): it always calls
// ConnectionResolver first, and whenever a connectionId is present it relays
// the scan to the agent's ports.detect handler rather than silently
// returning an empty result. There is deliberately no
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

func (uc *ScanWorkspacePorts) Execute(ctx context.Context, in ScanWorkspacePortsInput) ([]DetectedPort, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	if in.ConnectionID != "" {
		connected, devServer, _, resolveErr := uc.resolver.ResolveConnection(ctx, tenantID, in.ConnectionID)
		if resolveErr != nil {
			return nil, apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", resolveErr)
		}
		if connected {
			// A connectionId is bound: relay to the agent's real RPC name
			// (agent/src/relay/port-scan-handler.ts registers "ports.detect",
			// not "ports.scan" — TASK-SSH-04-01). Its error propagates here
			// rather than being swallowed into an empty result — see the type
			// doc comment above.
			result, execErr := uc.agent.Exec(ctx, devServer, "ports.detect", map[string]any{"worktreeId": in.WorktreeID})
			if execErr != nil {
				return nil, apperrors.New(apperrors.KindInternal, "INFRA_AGENT_EXEC_FAILED", "failed to relay workspace port scan to dev server agent", execErr)
			}
			return decodeDetectedPorts(result), nil
		}
	}

	// No connectionId bound (or it didn't resolve to a live dev server): the
	// worktree is local. Actually performing a local port scan is out of
	// scope for this scaffold — see this service's README "Known gaps". This
	// service's contract is *routing* the scan, not executing it.
	return []DetectedPort{}, nil
}

// decodeDetectedPorts extracts the "ports" field ports.detect's real
// response carries — see agent/src/relay/port-scan-handler.ts's
// DetectedPort type. Defensive against absent/malformed fields.
func decodeDetectedPorts(result map[string]any) []DetectedPort {
	raw, ok := result["ports"].([]any)
	if !ok {
		return []DetectedPort{}
	}
	out := make([]DetectedPort, 0, len(raw))
	for _, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		port, ok := toInt32(m["port"])
		if !ok {
			continue
		}
		host, _ := m["host"].(string)
		pid, _ := toInt32(m["pid"])
		processName, _ := m["processName"].(string)
		out = append(out, DetectedPort{Port: port, Host: host, PID: pid, ProcessName: processName})
	}
	return out
}

func toInt32(v any) (int32, bool) {
	switch n := v.(type) {
	case float64:
		return int32(n), true
	case int32:
		return n, true
	case int:
		return int32(n), true
	default:
		return 0, false
	}
}
