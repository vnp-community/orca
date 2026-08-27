package usecase

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
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
	resolver   ConnectionResolver
	projects   ProjectClient
	scrollback ScrollbackCleaner
	scm        SCMClient
	local      GitExecutor
	relay      GitExecutor
	terminals  TerminalSessionLister
}

func NewRemoveWorktree(resolver ConnectionResolver, projects ProjectClient, scrollback ScrollbackCleaner, scm SCMClient, local, relay GitExecutor, terminals TerminalSessionLister) *RemoveWorktree {
	return &RemoveWorktree{resolver: resolver, projects: projects, scrollback: scrollback, scm: scm, local: local, relay: relay, terminals: terminals}
}

type RemoveWorktreeInput struct {
	WorktreeID string
	Force      bool
	// AllowOpenPR is BR-AT-12's separate, explicit override — NEVER set
	// true by the automated cleanup_worktrees path.
	AllowOpenPR bool
	// StopAgents is the spec's "Stop & Delete" choice: kill active PTY
	// sessions found in this worktree before removing it.
	StopAgents bool
}

// Execute enforces BR-AT-11 (uncommitted changes), BR-AT-12 (open PR), and
// BR-WT-10 (active agent session) before the actual `git worktree remove` —
// for every caller (manual worktree.rm AND the automated bulk
// cleanup_worktrees step, which always calls with Force=false,
// AllowOpenPR=false, per workflow-service.CleanupWorktreesStepExecutor's
// doc comment: an automated bulk delete must never bypass either rule).
func (uc *RemoveWorktree) Execute(ctx context.Context, in RemoveWorktreeInput) (domain.RemoveWorktreeResult, error) {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.RemoveWorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}

	// BR-AT-11 / BR-WT-09 — re-run the uncommitted-changes check
	// server-side, don't trust that the client already called
	// CheckWorktreeDeleteSafety. Fails closed on its own check error: an
	// uncommitted-changes check that can't run must not silently let a
	// dirty worktree through.
	status, err := executor.GetStatus(ctx, repoPath)
	if err != nil {
		return domain.RemoveWorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_STATUS_CHECK_FAILED", "failed to check worktree status before removal", err)
	}
	uncommittedCount := len(status.Files)
	if uncommittedCount > 0 && !in.Force {
		return domain.RemoveWorktreeResult{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_HAS_UNCOMMITTED_CHANGES",
			fmt.Sprintf("%d files uncommitted", uncommittedCount), nil)
	}

	// BR-AT-12 — independent of force; requires the SEPARATE allow_open_pr
	// override. A GetPullRequestForBranch error (no SCM integration
	// configured, no tenant in context, or the KNOWN GAP in
	// internal/adapter/scmclient) fails OPEN on this check only — a repo
	// with no way to answer "does this branch have an open PR" must not
	// become permanently undeletable.
	if branch := status.Branch; branch != "" {
		if tenantID, ok := tenant.TenantID(ctx); ok {
			pr, found, err := uc.scm.GetPullRequestForBranch(ctx, tenantID, branch)
			if err == nil && found && pr.State == "open" && !in.AllowOpenPR {
				return domain.RemoveWorktreeResult{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_HAS_OPEN_PR", "worktree's branch has an open pull request", nil)
			}
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

	// Best-effort: an orphaned scrollback row is caught by BR-TM-12's 30-day
	// sweep either way, so a cleanup failure here must not fail the worktree
	// removal itself.
	if err := uc.scrollback.DeleteTerminalScrollbackSnapshots(ctx, in.WorktreeID); err != nil {
		// TODO: thread a structured logger into RemoveWorktree if one isn't
		// already available at this call site; log.Printf is a placeholder.
		log.Printf("remove_worktree: best-effort scrollback cleanup failed for worktree %s: %v", in.WorktreeID, err)
	}

	if err := uc.projects.RecordWorktreeRemoved(ctx, in.WorktreeID); err != nil {
		return domain.RemoveWorktreeResult{StoppedPtyIDs: stoppedPtyIDs}, apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_STALE", "worktree removed but bookkeeping update failed; will self-heal via worktree.detectedList", err)
	}
	return domain.RemoveWorktreeResult{UncommittedFilesDiscarded: uncommittedCount, StoppedPtyIDs: stoppedPtyIDs}, nil
}
