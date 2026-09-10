package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// fakeGitExecutorWithStatus wraps a *fakeGitExecutor and overrides
// GetStatus to report a caller-supplied status — the shared fakeGitExecutor's
// default GetStatus always returns exactly one modified file (for
// gatherFullDiff's tests), which this file's tests need finer control over.
type fakeGitExecutorWithStatus struct {
	*fakeGitExecutor
	status domain.GitStatus
}

func (f *fakeGitExecutorWithStatus) GetStatus(ctx context.Context, repoPath string) (domain.GitStatus, error) {
	f.fakeGitExecutor.calledGetStatus = true
	f.fakeGitExecutor.gotRepoPath = repoPath
	return f.status, nil
}

func TestCheckWorktreeDeleteSafety_CountsUncommittedAndUntrackedSeparately(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo-feature"}}
	local := &fakeGitExecutorWithStatus{
		fakeGitExecutor: &fakeGitExecutor{},
		status: domain.GitStatus{Branch: "main", Files: []domain.FileStatus{
			{Path: "a.txt", State: domain.FileStateModified},
			{Path: "b.txt", State: domain.FileStateAdded},
			{Path: "c.txt", State: domain.FileStateDeleted},
			{Path: "d.txt", State: domain.FileStateConflicted},
			{Path: "e.txt", State: domain.FileStateUntracked},
			{Path: "f.txt", State: domain.FileStateUntracked},
		}},
	}
	relay := &fakeGitExecutor{}
	terminals := &fakeTerminalSessionLister{}
	uc := NewCheckWorktreeDeleteSafety(resolver, local, relay, terminals)

	report, err := uc.Execute(context.Background(), "wt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.UncommittedFiles != 4 {
		t.Errorf("expected 4 uncommitted files (modified+added+deleted+conflicted), got %d", report.UncommittedFiles)
	}
	if report.UntrackedFiles != 2 {
		t.Errorf("expected 2 untracked files, got %d", report.UntrackedFiles)
	}
}

func TestCheckWorktreeDeleteSafety_NoActiveConnection_AgentRunningFalse_NoTerminalCall(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo-feature"}}
	local := &fakeGitExecutorWithStatus{fakeGitExecutor: &fakeGitExecutor{}, status: domain.GitStatus{Branch: "main"}}
	relay := &fakeGitExecutor{}
	terminals := &fakeTerminalSessionLister{}
	uc := NewCheckWorktreeDeleteSafety(resolver, local, relay, terminals)

	report, err := uc.Execute(context.Background(), "wt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.AgentRunning {
		t.Error("expected AgentRunning=false when there is no active connection")
	}
	if terminals.calledList {
		t.Error("expected TerminalSessionLister.ListSessions NOT to be called when there is no active connection")
	}
}

func TestCheckWorktreeDeleteSafety_ActiveSessionInWorktree_ReportsPtyID(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, ConnectionID: "conn-1", RepoPath: "/repo-feature"}}
	local := &fakeGitExecutorWithStatus{fakeGitExecutor: &fakeGitExecutor{}, status: domain.GitStatus{Branch: "main"}}
	relay := &fakeGitExecutor{}
	terminals := &fakeTerminalSessionLister{sessions: []domain.TerminalSessionRef{
		{PtyID: "pty-match", Cwd: "/repo-feature/sub"},
		{PtyID: "pty-nomatch", Cwd: "/repo-other"},
	}}
	uc := NewCheckWorktreeDeleteSafety(resolver, local, relay, terminals)

	report, err := uc.Execute(context.Background(), "wt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.AgentRunning {
		t.Error("expected AgentRunning=true")
	}
	if len(report.ActivePtyIDs) != 1 || report.ActivePtyIDs[0] != "pty-match" {
		t.Errorf("expected ActivePtyIDs=[pty-match], got %v", report.ActivePtyIDs)
	}
}

func TestCheckWorktreeDeleteSafety_SafeToDelete_TrueOnlyWhenAllCountsZero(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo-feature"}}
	local := &fakeGitExecutorWithStatus{fakeGitExecutor: &fakeGitExecutor{}, status: domain.GitStatus{Branch: "main"}}
	relay := &fakeGitExecutor{}
	terminals := &fakeTerminalSessionLister{}
	uc := NewCheckWorktreeDeleteSafety(resolver, local, relay, terminals)

	report, err := uc.Execute(context.Background(), "wt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.SafeToDelete {
		t.Errorf("expected SafeToDelete=true when all counts are zero, got %+v", report)
	}

	// Dirty case: SafeToDelete must flip to false.
	dirtyLocal := &fakeGitExecutorWithStatus{fakeGitExecutor: &fakeGitExecutor{}, status: domain.GitStatus{
		Branch: "main", Files: []domain.FileStatus{{Path: "a.txt", State: domain.FileStateModified}},
	}}
	uc2 := NewCheckWorktreeDeleteSafety(resolver, dirtyLocal, relay, terminals)
	report2, err := uc2.Execute(context.Background(), "wt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report2.SafeToDelete {
		t.Errorf("expected SafeToDelete=false when there are uncommitted files, got %+v", report2)
	}
}
