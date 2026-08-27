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
	scrollback := &fakeScrollbackCleaner{}
	uc := NewRemoveWorktree(resolver, projects, scrollback, local, relay)

	err := uc.Execute(context.Background(), "wt-1", true)
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
	scrollback := &fakeScrollbackCleaner{}
	uc := NewRemoveWorktree(resolver, projects, scrollback, local, relay)

	err := uc.Execute(context.Background(), "wt-1", true)
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
	scrollback := &fakeScrollbackCleaner{}
	uc := NewRemoveWorktree(resolver, projects, scrollback, local, relay)

	err := uc.Execute(context.Background(), "wt-1", false)
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

// TestRemoveWorktree_ScrollbackCleanupCalledWithWorktreeID guards TASK-TM-03-08's
// best-effort cleanup hook: DeleteTerminalScrollbackSnapshots must be called
// with the removed worktree's ID.
func TestRemoveWorktree_ScrollbackCleanupCalledWithWorktreeID(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo-feature"}}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	scrollback := &fakeScrollbackCleaner{}
	uc := NewRemoveWorktree(resolver, projects, scrollback, local, relay)

	err := uc.Execute(context.Background(), "wt-1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !scrollback.called {
		t.Error("expected DeleteTerminalScrollbackSnapshots to be called")
	}
	if scrollback.gotWorktreeID != "wt-1" {
		t.Errorf("expected DeleteTerminalScrollbackSnapshots to be called with wt-1, got %q", scrollback.gotWorktreeID)
	}
}

// TestRemoveWorktree_ScrollbackCleanupFails_DoesNotFailRemoveWorktree guards
// the "best-effort, logged not surfaced" contract: a cleanup RPC failure
// must not fail the overall RemoveWorktree.Execute call.
func TestRemoveWorktree_ScrollbackCleanupFails_DoesNotFailRemoveWorktree(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo-feature"}}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	scrollback := &fakeScrollbackCleaner{err: errors.New("infra-fleet-service unreachable")}
	uc := NewRemoveWorktree(resolver, projects, scrollback, local, relay)

	err := uc.Execute(context.Background(), "wt-1", true)
	if err != nil {
		t.Fatalf("expected a scrollback cleanup failure not to fail RemoveWorktree, got: %v", err)
	}
	if !scrollback.called {
		t.Error("expected DeleteTerminalScrollbackSnapshots to still have been called")
	}
}
