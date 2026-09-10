package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func TestMergeWorktreeIntoBase_InvalidStrategy_RejectsBeforeAnyExecutorCall(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	uc := NewMergeWorktreeIntoBase(resolver, projects, local, relay)

	_, err := uc.Execute(context.Background(), MergeWorktreeInput{WorktreeID: "wt-1", BaseBranch: "main", Strategy: "bogus"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "MERGE_STRATEGY_INVALID" {
		t.Fatalf("expected MERGE_STRATEGY_INVALID, got %v", err)
	}
	if local.calledGetStatus || local.calledMergeBranch || local.calledRebaseFromBase || local.calledFastForward {
		t.Error("expected zero executor calls when the strategy is invalid")
	}
	if projects.calledGetWorktree {
		t.Error("expected GetWorktree NOT to be called when the strategy is invalid")
	}
}

// TestMergeWorktreeIntoBase_UncommittedChangesInWinner_Rejects (BR-WT-16).
func TestMergeWorktreeIntoBase_UncommittedChangesInWinner_Rejects(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{} // fakeGitExecutor's GetStatus default returns one changed file
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getWorktreeResult: domain.WorktreeInfo{ID: "wt-1", RepoID: "repo-1", Branch: "feature"}}
	uc := NewMergeWorktreeIntoBase(resolver, projects, local, relay)

	_, err := uc.Execute(context.Background(), MergeWorktreeInput{WorktreeID: "wt-1", BaseBranch: "main", Strategy: "merge"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "MERGE_UNCOMMITTED_CHANGES" {
		t.Fatalf("expected MERGE_UNCOMMITTED_CHANGES, got %v", err)
	}
	if local.calledMergeBranch {
		t.Error("expected MergeBranch to never be called when the winning worktree has uncommitted changes")
	}
}

// TestMergeWorktreeIntoBase_RebaseStrategy_CallsRebaseFromBaseThenFastForward_NotMergeBranch
// is the key regression guard distinguishing the two code paths.
func TestMergeWorktreeIntoBase_RebaseStrategy_CallsRebaseFromBaseThenFastForward_NotMergeBranch(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{}
	// Make GetStatus report a clean worktree (no uncommitted changes).
	cleanLocal := &fakeGitExecutorCleanStatus{fakeGitExecutor: local}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getWorktreeResult: domain.WorktreeInfo{ID: "wt-1", RepoID: "repo-1", Branch: "feature"}}
	uc := NewMergeWorktreeIntoBase(resolver, projects, cleanLocal, relay)

	result, err := uc.Execute(context.Background(), MergeWorktreeInput{WorktreeID: "wt-1", BaseBranch: "main", Strategy: "rebase"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !local.calledRebaseFromBase {
		t.Error("expected RebaseFromBase to be called")
	}
	if !local.calledFastForward {
		t.Error("expected FastForward to be called")
	}
	if local.calledMergeBranch {
		t.Error("expected MergeBranch to NOT be called for the rebase strategy")
	}
	if result.HasConflicts {
		t.Errorf("expected no conflicts, got %+v", result)
	}
}

func TestMergeWorktreeIntoBase_MergeStrategy_PassesNoFF(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{}
	cleanLocal := &fakeGitExecutorCleanStatus{fakeGitExecutor: local}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getWorktreeResult: domain.WorktreeInfo{ID: "wt-1", RepoID: "repo-1", Branch: "feature"}}
	uc := NewMergeWorktreeIntoBase(resolver, projects, cleanLocal, relay)

	_, err := uc.Execute(context.Background(), MergeWorktreeInput{WorktreeID: "wt-1", BaseBranch: "main", Strategy: "merge"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !local.calledMergeBranch {
		t.Fatal("expected MergeBranch to be called")
	}
	if local.gotMergeBranchStrategy != "merge" {
		t.Errorf("expected strategy=merge, got %q", local.gotMergeBranchStrategy)
	}
}

func TestMergeWorktreeIntoBase_SquashStrategy_CommitsWithMessage(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{}
	cleanLocal := &fakeGitExecutorCleanStatus{fakeGitExecutor: local}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getWorktreeResult: domain.WorktreeInfo{ID: "wt-1", RepoID: "repo-1", Branch: "feature"}}
	uc := NewMergeWorktreeIntoBase(resolver, projects, cleanLocal, relay)

	_, err := uc.Execute(context.Background(), MergeWorktreeInput{WorktreeID: "wt-1", BaseBranch: "main", Strategy: "squash", CommitMessage: "Squash it"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if local.gotMergeBranchStrategy != "squash" {
		t.Errorf("expected strategy=squash, got %q", local.gotMergeBranchStrategy)
	}
	if local.gotMergeBranchMessage != "Squash it" {
		t.Errorf("expected commitMessage to be forwarded, got %q", local.gotMergeBranchMessage)
	}
}

// TestMergeWorktreeIntoBase_Conflict_ReturnsHasConflictsTrue_NotAnError
// (BR-WT-17).
func TestMergeWorktreeIntoBase_Conflict_ReturnsHasConflictsTrue_NotAnError(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{mergeBranchResult: domain.MergeResult{HasConflicts: true, ConflictedPaths: []string{"a.txt"}}}
	cleanLocal := &fakeGitExecutorCleanStatus{fakeGitExecutor: local}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getWorktreeResult: domain.WorktreeInfo{ID: "wt-1", RepoID: "repo-1", Branch: "feature"}}
	uc := NewMergeWorktreeIntoBase(resolver, projects, cleanLocal, relay)

	result, err := uc.Execute(context.Background(), MergeWorktreeInput{WorktreeID: "wt-1", BaseBranch: "main", Strategy: "merge"})
	if err != nil {
		t.Fatalf("expected a nil error on conflict (a conflict is a domain outcome, not a Go error), got %v", err)
	}
	if !result.HasConflicts {
		t.Fatalf("expected HasConflicts=true, got %+v", result)
	}
	// This usecase must not itself call AbortMerge or any resolve method —
	// BR-WT-17 requires manual resolution only.
	if local.calledAbortMerge || local.calledResolveConflict {
		t.Error("expected MergeWorktreeIntoBase to never call AbortMerge/ResolveConflict itself")
	}
	if result.ConflictDispatchKey != "repo:repo-1" {
		t.Errorf("expected ConflictDispatchKey to be the repo-scoped dispatch key, got %q", result.ConflictDispatchKey)
	}
}

// TestMergeWorktreeIntoBase_RepoScopedDispatch_UsesRepoIDNotWorktreeID is a
// regression guard for the repo-scoped dispatch pattern this usecase
// follows from CreateWorktree.
func TestMergeWorktreeIntoBase_RepoScopedDispatch_UsesRepoIDNotWorktreeID(t *testing.T) {
	resolver := &fakeConnectionResolverRecordsCalls{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{}
	cleanLocal := &fakeGitExecutorCleanStatus{fakeGitExecutor: local}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getWorktreeResult: domain.WorktreeInfo{ID: "wt-1", RepoID: "repo-99", Branch: "feature"}}
	uc := NewMergeWorktreeIntoBase(resolver, projects, cleanLocal, relay)

	_, err := uc.Execute(context.Background(), MergeWorktreeInput{WorktreeID: "wt-1", BaseBranch: "main", Strategy: "merge"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, id := range resolver.gotIDs {
		if id == "repo-99" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected dispatchExecutor to have been called with repo.ID (repo-99) for the main-repo dispatch, got calls: %v", resolver.gotIDs)
	}
}

// fakeGitExecutorCleanStatus wraps a *fakeGitExecutor and overrides
// GetStatus to report a clean (no uncommitted changes) worktree — the
// shared fakeGitExecutor's default GetStatus always returns one changed
// file (for gatherFullDiff's tests), which would spuriously trip BR-WT-16.
type fakeGitExecutorCleanStatus struct {
	*fakeGitExecutor
}

func (f *fakeGitExecutorCleanStatus) GetStatus(ctx context.Context, repoPath string) (domain.GitStatus, error) {
	f.fakeGitExecutor.calledGetStatus = true
	f.fakeGitExecutor.gotRepoPath = repoPath
	return domain.GitStatus{Branch: "main"}, nil
}

// fakeConnectionResolverRecordsCalls records every worktreeID
// dispatchExecutor resolved against, for regression-guarding dispatch-key
// choices (e.g. repo.ID vs. in.WorktreeID).
type fakeConnectionResolverRecordsCalls struct {
	conn   ResolvedConnection
	err    error
	gotIDs []string
}

func (f *fakeConnectionResolverRecordsCalls) ResolveConnection(ctx context.Context, worktreeID string) (ResolvedConnection, error) {
	f.gotIDs = append(f.gotIDs, worktreeID)
	if f.err != nil {
		return ResolvedConnection{}, f.err
	}
	return f.conn, nil
}
