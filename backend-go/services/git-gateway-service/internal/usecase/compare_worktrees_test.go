package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func TestCompareWorktrees_LessThanTwoWorktrees_Rejects(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	uc := NewCompareWorktrees(resolver, projects, local, relay)

	for _, ids := range [][]string{nil, {"wt-1"}} {
		_, err := uc.Execute(context.Background(), ids)
		if err == nil {
			t.Fatalf("expected an error for %v", ids)
		}
		var ae *apperrors.AppError
		if !errors.As(err, &ae) || ae.Code != "COMPARE_NEEDS_AT_LEAST_TWO" {
			t.Fatalf("expected COMPARE_NEEDS_AT_LEAST_TWO, got %v", err)
		}
	}
}

// TestCompareWorktrees_BaseRefMismatch_RejectsBeforeAnyBranchCompareCall
// (BR-WT-13).
func TestCompareWorktrees_BaseRefMismatch_RejectsBeforeAnyBranchCompareCall(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClientWithWorktrees{
		byID: map[string]domain.WorktreeInfo{
			"wt-1": {ID: "wt-1", BaseRef: "main"},
			"wt-2": {ID: "wt-2", BaseRef: "develop"},
		},
	}
	uc := NewCompareWorktrees(resolver, projects, local, relay)

	_, err := uc.Execute(context.Background(), []string{"wt-1", "wt-2"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_COMPARE_BASE_MISMATCH" {
		t.Fatalf("expected WORKTREE_COMPARE_BASE_MISMATCH, got %v", err)
	}
	if local.calledBranchCompare || relay.calledBranchCompare {
		t.Error("expected BranchCompare to never be called when base_ref mismatches")
	}
}

func TestCompareWorktrees_MissingBaseRef_RejectsWithClearCode(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClientWithWorktrees{
		byID: map[string]domain.WorktreeInfo{
			"wt-1": {ID: "wt-1", BaseRef: ""},
			"wt-2": {ID: "wt-2", BaseRef: "main"},
		},
	}
	uc := NewCompareWorktrees(resolver, projects, local, relay)

	_, err := uc.Execute(context.Background(), []string{"wt-1", "wt-2"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_BASE_REF_UNKNOWN" {
		t.Fatalf("expected WORKTREE_BASE_REF_UNKNOWN, got %v", err)
	}
}

func TestCompareWorktrees_AggregatesAddedRemovedFromEntries(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	branchCompareResult := domain.BranchCompareResult{
		Status: "ready", MergeBase: "merge123", ChangedFiles: 2,
		Entries: []domain.GitChangeEntry{
			{Path: "a.txt", Added: 3, Removed: 1},
			{Path: "b.txt", Added: 5, Removed: 2},
		},
	}
	local := &fakeGitExecutor{branchCompareResult: branchCompareResult}
	relay := &fakeGitExecutor{branchCompareResult: branchCompareResult}
	projects := &fakeProjectClientWithWorktrees{
		byID: map[string]domain.WorktreeInfo{
			"wt-1": {ID: "wt-1", BaseRef: "main"},
			"wt-2": {ID: "wt-2", BaseRef: "main"},
		},
	}
	uc := NewCompareWorktrees(resolver, projects, local, relay)

	result, err := uc.Execute(context.Background(), []string{"wt-1", "wt-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BaseRef != "main" {
		t.Errorf("expected shared base_ref main, got %q", result.BaseRef)
	}
	if len(result.Worktrees) != 2 {
		t.Fatalf("expected 2 comparisons, got %d", len(result.Worktrees))
	}
	for _, w := range result.Worktrees {
		if w.AddedLines != 8 || w.RemovedLines != 3 {
			t.Errorf("expected AddedLines=8/RemovedLines=3 (sum of entries), got %+v", w)
		}
	}
}

// TestCompareWorktrees_OneWorktreeCompareFails_WholeCallFails is deliberately
// fail-fast — this usecase does NOT use SOL-WT-02's per-item isolation
// pattern, since a partial comparison is not useful (unlike SOL-WT-02's
// fan-out, where a partial answer is still actionable).
func TestCompareWorktrees_OneWorktreeCompareFails_WholeCallFails(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{branchCompareErr: errors.New("git error on one worktree")}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClientWithWorktrees{
		byID: map[string]domain.WorktreeInfo{
			"wt-1": {ID: "wt-1", BaseRef: "main"},
			"wt-2": {ID: "wt-2", BaseRef: "main"},
			"wt-3": {ID: "wt-3", BaseRef: "main"},
		},
	}
	uc := NewCompareWorktrees(resolver, projects, local, relay)

	_, err := uc.Execute(context.Background(), []string{"wt-1", "wt-2", "wt-3"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "COMPARE_WORKTREES_FAILED" {
		t.Fatalf("expected COMPARE_WORKTREES_FAILED, got %v", err)
	}
}

// fakeProjectClientWithWorktrees is a ProjectClient fake whose GetWorktree
// answers per-id, needed because fakeProjectClient (worktree_fakes_test.go)
// only supports one canned GetWorktree answer for every id.
type fakeProjectClientWithWorktrees struct {
	fakeProjectClient
	byID map[string]domain.WorktreeInfo
}

func (f *fakeProjectClientWithWorktrees) GetWorktree(ctx context.Context, worktreeID string) (domain.WorktreeInfo, error) {
	wt, ok := f.byID[worktreeID]
	if !ok {
		return domain.WorktreeInfo{}, errors.New("not found")
	}
	return wt, nil
}
