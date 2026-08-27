package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
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

// TestRemoveWorktree_UncommittedChanges_ForceFalse_RejectsBeforeGitCall is
// BR-WT-09's server-side re-check — a dirty worktree without force=true is
// rejected before any removal is attempted. This is the core regression
// guard against this bug's own finding: force=true previously bypassed the
// only existing check entirely.
func TestRemoveWorktree_UncommittedChanges_ForceFalse_RejectsBeforeGitCall(t *testing.T) {
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

func TestRemoveWorktree_UncommittedChanges_ForceTrue_ProceedsToGitCall(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo-feature"}}
	local := &fakeGitExecutor{} // GetStatus's fake default returns one changed file
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	terminals := &fakeTerminalSessionLister{}
	uc := NewRemoveWorktree(resolver, projects, local, relay, terminals)

	result, err := uc.Execute(context.Background(), RemoveWorktreeInput{WorktreeID: "wt-1", Force: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if local.removeWorktreeCallCount != 1 {
		t.Errorf("expected RemoveWorktree to be called exactly once, got %d", local.removeWorktreeCallCount)
	}
	if result.UncommittedFilesDiscarded != 1 {
		t.Errorf("expected UncommittedFilesDiscarded=1 (echoing the overridden safety-check count), got %d", result.UncommittedFilesDiscarded)
	}
}

func TestRemoveWorktree_AgentRunning_StopAgentsFalse_RejectsBeforeGitCall(t *testing.T) {
	// Connected: true routes dispatchExecutor to relay, not local — see
	// dispatchExecutor's doc comment.
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, ConnectionID: "conn-1", RepoPath: "/repo-feature"}}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutorCleanStatus{fakeGitExecutor: &fakeGitExecutor{}}
	projects := &fakeProjectClient{}
	terminals := &fakeTerminalSessionLister{sessions: []domain.TerminalSessionRef{{PtyID: "pty-1", Cwd: "/repo-feature"}}}
	uc := NewRemoveWorktree(resolver, projects, local, relay, terminals)

	_, err := uc.Execute(context.Background(), RemoveWorktreeInput{WorktreeID: "wt-1", Force: true, StopAgents: false})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_AGENT_RUNNING" {
		t.Fatalf("expected WORKTREE_AGENT_RUNNING, got %v", err)
	}
	if relay.calledRemoveWorktree {
		t.Error("expected RemoveWorktree NOT to be called when an active agent session blocks the removal")
	}
}

func TestRemoveWorktree_AgentRunning_StopAgentsTrue_KillsSessionsThenRemoves(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, ConnectionID: "conn-1", RepoPath: "/repo-feature"}}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutorCleanStatus{fakeGitExecutor: &fakeGitExecutor{}}
	projects := &fakeProjectClient{}
	terminals := &fakeTerminalSessionLister{sessions: []domain.TerminalSessionRef{
		{PtyID: "pty-1", Cwd: "/repo-feature"},
		{PtyID: "pty-2", Cwd: "/repo-feature/sub"},
	}}
	uc := NewRemoveWorktree(resolver, projects, local, relay, terminals)

	result, err := uc.Execute(context.Background(), RemoveWorktreeInput{WorktreeID: "wt-1", Force: true, StopAgents: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(terminals.killedPtyIDs) != 2 {
		t.Errorf("expected Kill to be called once per active session, got %d calls: %v", len(terminals.killedPtyIDs), terminals.killedPtyIDs)
	}
	if relay.removeWorktreeCallCount != 1 {
		t.Errorf("expected RemoveWorktree to be called exactly once, after the kills, got %d", relay.removeWorktreeCallCount)
	}
	if len(result.StoppedPtyIDs) != 2 {
		t.Errorf("expected StoppedPtyIDs to report both killed sessions, got %v", result.StoppedPtyIDs)
	}
}

func TestRemoveWorktree_KillFails_StillProceedsWithRemoval_BestEffort(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, ConnectionID: "conn-1", RepoPath: "/repo-feature"}}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutorCleanStatus{fakeGitExecutor: &fakeGitExecutor{}}
	projects := &fakeProjectClient{}
	terminals := &fakeTerminalSessionLister{
		sessions: []domain.TerminalSessionRef{{PtyID: "pty-1", Cwd: "/repo-feature"}},
		killErr:  errors.New("kill failed: process already exited"),
	}
	uc := NewRemoveWorktree(resolver, projects, local, relay, terminals)

	result, err := uc.Execute(context.Background(), RemoveWorktreeInput{WorktreeID: "wt-1", Force: true, StopAgents: true})
	if err != nil {
		t.Fatalf("expected a kill failure to be tolerated (best-effort), got error: %v", err)
	}
	if relay.removeWorktreeCallCount != 1 {
		t.Errorf("expected RemoveWorktree to still be called despite the kill failure, got %d calls", relay.removeWorktreeCallCount)
	}
	if len(result.StoppedPtyIDs) != 0 {
		t.Errorf("expected no StoppedPtyIDs recorded for the failed kill, got %v", result.StoppedPtyIDs)
	}
}
