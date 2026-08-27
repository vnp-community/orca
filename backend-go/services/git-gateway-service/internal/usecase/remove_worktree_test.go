package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func withTenantCtx() context.Context {
	return tenant.WithTenantID(context.Background(), "tenant-1")
}

func TestRemoveWorktree_HappyPath(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo-feature"}}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	scrollback := &fakeScrollbackCleaner{}
	scm := &fakeSCMClient{}
	uc := NewRemoveWorktree(resolver, projects, scrollback, scm, local, relay)

	err := uc.Execute(withTenantCtx(), "wt-1", true, false)
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
	scm := &fakeSCMClient{}
	uc := NewRemoveWorktree(resolver, projects, scrollback, scm, local, relay)

	err := uc.Execute(withTenantCtx(), "wt-1", true, false)
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
	scm := &fakeSCMClient{}
	uc := NewRemoveWorktree(resolver, projects, scrollback, scm, local, relay)

	// force=true — this test exercises the RemoveWorktree-itself-fails
	// path, not BR-AT-11's uncommitted-changes check (fakeGitExecutor's
	// default GetStatus reports one modified file).
	err := uc.Execute(withTenantCtx(), "wt-1", true, false)
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

// BR-AT-11: uncommitted changes block removal unless force=true.
func TestRemoveWorktree_UncommittedChanges_BlocksWithoutForce(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo-feature"}}
	// Default fakeGitExecutor.GetStatus reports one modified file.
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	scrollback := &fakeScrollbackCleaner{}
	scm := &fakeSCMClient{}
	uc := NewRemoveWorktree(resolver, projects, scrollback, scm, local, relay)

	err := uc.Execute(withTenantCtx(), "wt-1", false, false)
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_HAS_UNCOMMITTED_CHANGES" {
		t.Fatalf("expected WORKTREE_HAS_UNCOMMITTED_CHANGES, got %v", err)
	}
	if local.removeWorktreeCallCount != 0 {
		t.Errorf("expected RemoveWorktree to never be called, got %d", local.removeWorktreeCallCount)
	}
}

func TestRemoveWorktree_UncommittedChanges_ProceedsWithForce(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo-feature"}}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	scrollback := &fakeScrollbackCleaner{}
	scm := &fakeSCMClient{}
	uc := NewRemoveWorktree(resolver, projects, scrollback, scm, local, relay)

	if err := uc.Execute(withTenantCtx(), "wt-1", true, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if local.removeWorktreeCallCount != 1 {
		t.Errorf("expected RemoveWorktree to be called, got %d", local.removeWorktreeCallCount)
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
	scm := &fakeSCMClient{}
	uc := NewRemoveWorktree(resolver, projects, scrollback, scm, local, relay)

	err := uc.Execute(withTenantCtx(), "wt-1", true, false)
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
	scm := &fakeSCMClient{}
	uc := NewRemoveWorktree(resolver, projects, scrollback, scm, local, relay)

	err := uc.Execute(withTenantCtx(), "wt-1", true, false)
	if err != nil {
		t.Fatalf("expected a scrollback cleanup failure not to fail RemoveWorktree, got: %v", err)
	}
	if !scrollback.called {
		t.Error("expected DeleteTerminalScrollbackSnapshots to still have been called")
	}
}

// BR-AT-12: an open PR blocks removal unless allow_open_pr=true — an
// INDEPENDENT override from force.
func TestRemoveWorktree_OpenPR_BlocksWithoutAllowOpenPR(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo-feature"}}
	local := &fakeGitExecutor{statusErr: nil}
	// No uncommitted changes, so we reach the PR check — override the
	// default fake's dirty-file status.
	local.getStatusResult = &domain.GitStatus{Branch: "feature-1"}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	scrollback := &fakeScrollbackCleaner{}
	scm := &fakeSCMClient{prForBranch: PullRequestInfo{State: "open", Number: 42}, prForBranchFound: true}
	uc := NewRemoveWorktree(resolver, projects, scrollback, scm, local, relay)

	err := uc.Execute(withTenantCtx(), "wt-1", true, false)
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_HAS_OPEN_PR" {
		t.Fatalf("expected WORKTREE_HAS_OPEN_PR, got %v", err)
	}
	if local.removeWorktreeCallCount != 0 {
		t.Errorf("expected RemoveWorktree to never be called, got %d", local.removeWorktreeCallCount)
	}
	if !scm.calledGetPRForBranch {
		t.Error("expected GetPullRequestForBranch to be called")
	}
}

func TestRemoveWorktree_OpenPR_ProceedsWithAllowOpenPR(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo-feature"}}
	local := &fakeGitExecutor{}
	local.getStatusResult = &domain.GitStatus{Branch: "feature-1"}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	scrollback := &fakeScrollbackCleaner{}
	scm := &fakeSCMClient{prForBranch: PullRequestInfo{State: "open", Number: 42}, prForBranchFound: true}
	uc := NewRemoveWorktree(resolver, projects, scrollback, scm, local, relay)

	if err := uc.Execute(withTenantCtx(), "wt-1", true, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if local.removeWorktreeCallCount != 1 {
		t.Errorf("expected RemoveWorktree to be called, got %d", local.removeWorktreeCallCount)
	}
}

// A SCMClient lookup error (e.g. no SCM integration configured) fails OPEN
// — deletion proceeds rather than becoming permanently blocked.
func TestRemoveWorktree_SCMClientError_FailsOpenAndProceeds(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo-feature"}}
	local := &fakeGitExecutor{}
	local.getStatusResult = &domain.GitStatus{Branch: "feature-1"}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	scrollback := &fakeScrollbackCleaner{}
	scm := &fakeSCMClient{prForBranchErr: errors.New("no SCM integration configured for this repo")}
	uc := NewRemoveWorktree(resolver, projects, scrollback, scm, local, relay)

	if err := uc.Execute(withTenantCtx(), "wt-1", true, false); err != nil {
		t.Fatalf("expected the SCM lookup failure to fail open (no error), got %v", err)
	}
	if local.removeWorktreeCallCount != 1 {
		t.Errorf("expected RemoveWorktree to be called despite the SCM lookup failure, got %d", local.removeWorktreeCallCount)
	}
}
