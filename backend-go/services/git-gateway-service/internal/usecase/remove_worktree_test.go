package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
)

func TestRemoveWorktree_HappyPath(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo-feature"}}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	terminals := &fakeTerminalSessionLister{}
	uc := NewRemoveWorktree(resolver, projects, local, relay, terminals)

	_, err := uc.Execute(context.Background(), RemoveWorktreeInput{WorktreeID: "wt-1", Force: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if local.removeWorktreeCallCount != 1 {
		t.Errorf("expected RemoveWorktree to be called exactly once, got %d", local.removeWorktreeCallCount)
	}
	if !projects.calledRecordRemoved {
		t.Error("expected RecordWorktreeRemoved to be called")
	}
	if projects.gotRecordRemovedID != "wt-1" {
		t.Errorf("expected RecordWorktreeRemoved to be called with wt-1, got %q", projects.gotRecordRemovedID)
	}
}

func TestRemoveWorktree_BookkeepingFails_NoCompensatingGitOperation(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo-feature"}}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{recordRemovedErr: errors.New("project-service unreachable")}
	terminals := &fakeTerminalSessionLister{}
	uc := NewRemoveWorktree(resolver, projects, local, relay, terminals)

	_, err := uc.Execute(context.Background(), RemoveWorktreeInput{WorktreeID: "wt-1", Force: true})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_BOOKKEEPING_STALE" {
		t.Fatalf("expected WORKTREE_BOOKKEEPING_STALE, got %v", err)
	}
	// Unlike CreateWorktree's compensation, RemoveWorktree.Execute must NOT
	// run any extra git operation when bookkeeping fails: RemoveWorktree
	// was already called exactly once (the real removal), and
	// CreateWorktree must never be called at all.
	if local.removeWorktreeCallCount != 1 {
		t.Errorf("expected RemoveWorktree to have been called exactly once (no extra compensating call), got %d", local.removeWorktreeCallCount)
	}
	if local.createWorktreeCallCount != 0 {
		t.Errorf("expected CreateWorktree to never be called, got %d calls", local.createWorktreeCallCount)
	}
}

func TestRemoveWorktree_GitRemoveFails_BookkeepingNeverCalled(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo-feature"}}
	local := &fakeGitExecutor{removeWorktreeErr: errors.New("git worktree remove failed: dirty worktree")}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	terminals := &fakeTerminalSessionLister{}
	uc := NewRemoveWorktree(resolver, projects, local, relay, terminals)

	_, err := uc.Execute(context.Background(), RemoveWorktreeInput{WorktreeID: "wt-1", Force: true})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_REMOVE_FAILED" {
		t.Fatalf("expected WORKTREE_REMOVE_FAILED, got %v", err)
	}
	if projects.calledRecordRemoved {
		t.Error("expected RecordWorktreeRemoved NOT to be called when git worktree remove itself fails")
	}
}

// TestRemoveWorktree_UncommittedChanges_RejectsWithoutForce is BR-WT-09's
// server-side re-check — a dirty worktree without force=true is rejected
// before any removal is attempted.
func TestRemoveWorktree_UncommittedChanges_RejectsWithoutForce(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo-feature"}}
	local := &fakeGitExecutor{} // GetStatus's fake default returns one changed file
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	terminals := &fakeTerminalSessionLister{}
	uc := NewRemoveWorktree(resolver, projects, local, relay, terminals)

	_, err := uc.Execute(context.Background(), RemoveWorktreeInput{WorktreeID: "wt-1", Force: false})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_HAS_UNCOMMITTED_CHANGES" {
		t.Fatalf("expected WORKTREE_HAS_UNCOMMITTED_CHANGES, got %v", err)
	}
	if local.calledRemoveWorktree {
		t.Error("expected RemoveWorktree NOT to be called when uncommitted changes block the removal")
	}
}
