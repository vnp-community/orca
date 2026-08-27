package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// KillWorkspacePortInput mirrors the gRPC request 1:1.
type KillWorkspacePortInput struct {
	ConnectionID string
	WorktreeID   string
	PID          int32
	Port         int32
}

// KillWorkspacePort follows ScanWorkspacePorts's exact resolve-then-dispatch
// shape (scan_workspace_ports.go) deliberately — the same "always
// resolve the connection first, relay when bound, never a silent
// if(connectionId) shortcut" structure, applied to a kill instead of a
// scan. Do not reintroduce backend-agent-execution-boundary.md's old
// TS bug class here.
type KillWorkspacePort struct {
	resolver ConnectionResolver
	agent    DevServerAgentClient
}

func NewKillWorkspacePort(resolver ConnectionResolver, agent DevServerAgentClient) *KillWorkspacePort {
	return &KillWorkspacePort{resolver: resolver, agent: agent}
}

func (uc *KillWorkspacePort) Execute(ctx context.Context, in KillWorkspacePortInput) (ok bool, reason string, err error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return false, "", apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	if in.ConnectionID != "" {
		connected, devServer, _, resolveErr := uc.resolver.ResolveConnection(ctx, tenantID, in.ConnectionID)
		if resolveErr != nil {
			return false, "", apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", resolveErr)
		}
		if connected {
			// Relay to the agent's ports.kill handler — same Exec port
			// ScanWorkspacePorts already uses, different method name. A
			// resolve failure or agent error is a real error, propagated
			// here, not swallowed into a false "ok:true".
			result, execErr := uc.agent.Exec(ctx, devServer, "ports.kill", map[string]any{
				"worktreeId": in.WorktreeID, "pid": in.PID, "port": in.Port,
			})
			if execErr != nil {
				return false, "", apperrors.New(apperrors.KindInternal, "INFRA_AGENT_EXEC_FAILED", "failed to relay workspace port kill to dev server agent", execErr)
			}
			return decodeKillResult(result)
		}
	}

	// No connectionId bound (or it didn't resolve): the worktree is local.
	// Actually killing a local process is out of scope for this scaffold —
	// same "routing, not executing" boundary scan_workspace_ports.go
	// already draws for the local branch. Honest ok:false, not a silent
	// no-op success — the frontend's WorkspacePortKillResult type already
	// has a {ok:false, reason} shape for exactly this case.
	return false, "local workspace-port kill is not implemented in this scaffold", nil
}

func decodeKillResult(result map[string]any) (bool, string, error) {
	ok, _ := result["ok"].(bool)
	reason, _ := result["reason"].(string)
	return ok, reason, nil
}
