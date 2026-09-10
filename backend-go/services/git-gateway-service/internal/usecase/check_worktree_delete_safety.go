package usecase

import (
	"context"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// CheckWorktreeDeleteSafety is the read RPC a client calls before rendering
// the delete-confirm dialog. Uncommitted/untracked counts come from
// GitExecutor.GetStatus (already required); the agent-running heuristic
// filters TerminalSessionLister.ListSessions by cwd prefix — flagged as
// imprecise per SOL-WT-03 (a session could be an ordinary shell, not an
// AI-CLI process; closing that gap needs SpawnTerminalSession-time tagging,
// out of scope here).
type CheckWorktreeDeleteSafety struct {
	resolver  ConnectionResolver
	local     GitExecutor
	relay     GitExecutor
	terminals TerminalSessionLister
}

func NewCheckWorktreeDeleteSafety(resolver ConnectionResolver, local, relay GitExecutor, terminals TerminalSessionLister) *CheckWorktreeDeleteSafety {
	return &CheckWorktreeDeleteSafety{resolver: resolver, local: local, relay: relay, terminals: terminals}
}

func (uc *CheckWorktreeDeleteSafety) Execute(ctx context.Context, worktreeID string) (domain.DeleteSafetyReport, error) {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return domain.DeleteSafetyReport{}, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	status, err := executor.GetStatus(ctx, repoPath)
	if err != nil {
		return domain.DeleteSafetyReport{}, apperrors.New(apperrors.KindInternal, "WORKTREE_STATUS_FAILED", "failed to check worktree status", err)
	}

	report := domain.DeleteSafetyReport{}
	for _, f := range status.Files {
		if f.State == domain.FileStateUntracked {
			report.UntrackedFiles++
		} else {
			report.UncommittedFiles++ // modified/added/deleted/conflicted
		}
	}

	conn, err := uc.resolver.ResolveConnection(ctx, worktreeID)
	if err == nil && conn.Connected {
		if sessions, listErr := uc.terminals.ListSessions(ctx, conn.ConnectionID); listErr == nil {
			for _, s := range sessions {
				if strings.HasPrefix(s.Cwd, repoPath) {
					report.ActivePtyIDs = append(report.ActivePtyIDs, s.PtyID)
				}
			}
		}
	}
	report.AgentRunning = len(report.ActivePtyIDs) > 0
	report.SafeToDelete = report.UncommittedFiles == 0 && report.UntrackedFiles == 0 && !report.AgentRunning
	return report, nil
}
