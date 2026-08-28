package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// GetAgentTerminalSession resolves worktreeID -> the live TerminalSession
// whose Cwd matches that worktree's resolved repo path. Backs the CLI's
// `--worktree <id>` flag (BUG-CLI-02) — closeable with zero new state,
// composing ConnectionResolver + TerminalSessionRepository the same way
// resolveTerminalSession composes them in the reverse direction (pty ->
// connection, here connection -> pty).
type GetAgentTerminalSession struct {
	resolver ConnectionResolver
	sessions TerminalSessionRepository
}

func NewGetAgentTerminalSession(resolver ConnectionResolver, sessions TerminalSessionRepository) *GetAgentTerminalSession {
	return &GetAgentTerminalSession{resolver: resolver, sessions: sessions}
}

func (uc *GetAgentTerminalSession) Execute(ctx context.Context, worktreeID string) (domain.TerminalSession, bool, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.TerminalSession{}, false, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	connected, _, conn, err := uc.resolver.ResolveConnectionByWorktree(ctx, tenantID, worktreeID)
	if err != nil {
		return domain.TerminalSession{}, false, apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve worktree's connection", err)
	}
	if !connected {
		return domain.TerminalSession{}, false, nil
	}

	sessions, err := uc.sessions.List(ctx, tenantID, conn.ID)
	if err != nil {
		return domain.TerminalSession{}, false, apperrors.New(apperrors.KindInternal, "INFRA_TERMINAL_LIST_FAILED", "failed to list terminal sessions", err)
	}

	var best domain.TerminalSession
	found := false
	for _, s := range sessions {
		// Exact match only — a subdirectory cwd is a different terminal
		// (e.g. the user cd'd into a subfolder), not "the" agent session.
		if s.Cwd != conn.RepoPath {
			continue
		}
		if !found || s.LastActiveAt.After(best.LastActiveAt) {
			best, found = s, true
		}
	}
	return best, found, nil
}
