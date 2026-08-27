package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// RemoveWorktree has no compensating action on bookkeeping failure — see
// this package's ports.go doc comment for why "on-disk gone, bookkeeping
// stale" is a safe terminal state, unlike CreateWorktree's failure
// direction: "worktree doesn't exist" is a safe terminal state to leave a
// stale bookkeeping record pointing at (unlike a create failure, which
// leaves live, unaccounted-for disk usage and a dangling branch).
//
// worktreeID is passed straight into dispatchExecutor here (not through
// ProjectClient.GetRepo first) — RemoveWorktreeRequest carries a
// worktree_id directly, which IS the dispatch key ConnectionResolver
// expects (see resolver.go's ResolveConnection doc comment), unlike the
// repo-scoped usecases (CreateWorktree, DetectWorktrees, ...) that only
// have a repo_id and must resolve it via GetRepo first.
//
// BR-WT-09/10 (SOL-WT-03): re-run the uncommitted-changes and
// active-agent-session checks server-side, as a guard against a client
// that skips CheckWorktreeDeleteSafety's pre-check call or races a change
// between check and confirm.
type RemoveWorktree struct {
	resolver  ConnectionResolver
	projects  ProjectClient
	local     GitExecutor
	relay     GitExecutor
	terminals TerminalSessionLister
}

func NewRemoveWorktree(resolver ConnectionResolver, projects ProjectClient, local, relay GitExecutor, terminals TerminalSessionLister) *RemoveWorktree {
	return &RemoveWorktree{resolver: resolver, projects: projects, local: local, relay: relay, terminals: terminals}
}

type RemoveWorktreeInput struct {
	WorktreeID string
	Force      bool
	StopAgents bool
}

func (uc *RemoveWorktree) Execute(ctx context.Context, in RemoveWorktreeInput) (domain.RemoveWorktreeResult, error) {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.RemoveWorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}

	// BR-WT-09 — re-run the uncommitted-changes check server-side, don't
	// trust that the client already called CheckWorktreeDeleteSafety.
	var uncommittedCount int
	if status, err := executor.GetStatus(ctx, repoPath); err == nil {
		uncommittedCount = len(status.Files)
		if uncommittedCount > 0 && !in.Force {
			return domain.RemoveWorktreeResult{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_HAS_UNCOMMITTED_CHANGES",
				fmt.Sprintf("%d files uncommitted", uncommittedCount), nil)
		}
	}

	// BR-WT-10 — same re-check for active agent sessions.
	var stoppedPtyIDs []string
	if conn, cErr := uc.resolver.ResolveConnection(ctx, in.WorktreeID); cErr == nil && conn.Connected {
		if sessions, lErr := uc.terminals.ListSessions(ctx, conn.ConnectionID); lErr == nil {
			var active []string
			for _, s := range sessions {
				if strings.HasPrefix(s.Cwd, repoPath) {
					active = append(active, s.PtyID)
				}
			}
			if len(active) > 0 && !in.StopAgents {
				return domain.RemoveWorktreeResult{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_AGENT_RUNNING",
					fmt.Sprintf("%d active session(s) in this worktree", len(active)), nil)
			}
			for _, ptyID := range active {
				if err := uc.terminals.Kill(ctx, ptyID); err != nil {
					// Best-effort — a kill failure must not block a delete
					// the user explicitly confirmed; the orphaned PTY
					// self-heals when its process exits against a
					// now-removed cwd.
					continue
				}
				stoppedPtyIDs = append(stoppedPtyIDs, ptyID)
			}
		}
	}

	if err := executor.RemoveWorktree(ctx, repoPath, in.Force); err != nil {
		return domain.RemoveWorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_REMOVE_FAILED", "git worktree remove failed", err)
	}
	if err := uc.projects.RecordWorktreeRemoved(ctx, in.WorktreeID); err != nil {
		return domain.RemoveWorktreeResult{StoppedPtyIDs: stoppedPtyIDs}, apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_STALE", "worktree removed but bookkeeping update failed; will self-heal via worktree.detectedList", err)
	}
	return domain.RemoveWorktreeResult{UncommittedFilesDiscarded: uncommittedCount, StoppedPtyIDs: stoppedPtyIDs}, nil
}
