package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// manyFileStatus builds n synthetic changed files for BR-CR-15's threshold
// tests.
func manyFileStatus(n int) []domain.FileStatus {
	files := make([]domain.FileStatus, n)
	for i := range files {
		files[i] = domain.FileStatus{Path: "file.go", State: domain.FileStateModified}
	}
	return files
}

func TestGenerateCommitMessage_OverFileThreshold_UsesStatsOnlyAndSkipsGetDiff(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, ConnectionID: "conn-1", RepoPath: "/repo/wt1"}}
	local := &fakeGitExecutor{name: "local"}
	relay := &fakeGitExecutor{name: "relay", statusResult: domain.GitStatus{Branch: "main", Files: manyFileStatus(51)}}
	getStatus := NewGetStatus(resolver, local, relay)
	getDiff := NewGetDiff(resolver, local, relay)
	history := NewHistory(resolver, local, relay)
	completer := &fakeAICompleter{message: "feat: large change"}
	uc := NewGenerateCommitMessage(resolver, getStatus, getDiff, history, completer)

	_, err := uc.Execute(context.Background(), GenerateCommitMessageInput{WorktreeID: "wt1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(completer.gotPrompt, "Staged changes are large") {
		t.Errorf("expected stats-only marker text in prompt, got %q", completer.gotPrompt)
	}
	if relay.calledGetDiff {
		t.Error("expected zero GetDiff calls above the file threshold")
	}
}

func TestGenerateCommitMessage_AtFileThreshold_UsesFullDiff(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, ConnectionID: "conn-1", RepoPath: "/repo/wt1"}}
	local := &fakeGitExecutor{name: "local"}
	relay := &fakeGitExecutor{name: "relay", statusResult: domain.GitStatus{Branch: "main", Files: manyFileStatus(50)}}
	getStatus := NewGetStatus(resolver, local, relay)
	getDiff := NewGetDiff(resolver, local, relay)
	history := NewHistory(resolver, local, relay)
	completer := &fakeAICompleter{message: "feat: boundary change"}
	uc := NewGenerateCommitMessage(resolver, getStatus, getDiff, history, completer)

	_, err := uc.Execute(context.Background(), GenerateCommitMessageInput{WorktreeID: "wt1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(completer.gotPrompt, "Staged changes are large") {
		t.Error("expected full-diff path at exactly the threshold, not stats-only")
	}
	if !relay.calledGetDiff {
		t.Error("expected GetDiff to be called at the threshold boundary")
	}
}

func TestGenerateCommitMessage_AppendsMissingIssueRef(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, ConnectionID: "conn-1", RepoPath: "/repo/wt1"}}
	local := &fakeGitExecutor{name: "local"}
	relay := &fakeGitExecutor{name: "relay", statusResult: domain.GitStatus{Branch: "fix/ORCA-123-foo", Files: []domain.FileStatus{{Path: "a.go", State: domain.FileStateModified}}}}
	getStatus := NewGetStatus(resolver, local, relay)
	getDiff := NewGetDiff(resolver, local, relay)
	history := NewHistory(resolver, local, relay)
	completer := &fakeAICompleter{message: "feat: fix the thing"}
	uc := NewGenerateCommitMessage(resolver, getStatus, getDiff, history, completer)

	got, err := uc.Execute(context.Background(), GenerateCommitMessageInput{WorktreeID: "wt1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "Refs: ORCA-123") {
		t.Errorf("expected a trailing Refs: ORCA-123 line, got %q", got)
	}
}

func TestGenerateCommitMessage_DoesNotDuplicateIssueRef(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, ConnectionID: "conn-1", RepoPath: "/repo/wt1"}}
	local := &fakeGitExecutor{name: "local"}
	relay := &fakeGitExecutor{name: "relay", statusResult: domain.GitStatus{Branch: "fix/ORCA-123-foo", Files: []domain.FileStatus{{Path: "a.go", State: domain.FileStateModified}}}}
	getStatus := NewGetStatus(resolver, local, relay)
	getDiff := NewGetDiff(resolver, local, relay)
	history := NewHistory(resolver, local, relay)
	completer := &fakeAICompleter{message: "feat: fix the thing\n\nRefs: ORCA-123"}
	uc := NewGenerateCommitMessage(resolver, getStatus, getDiff, history, completer)

	got, err := uc.Execute(context.Background(), GenerateCommitMessageInput{WorktreeID: "wt1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Count(got, "Refs:") != 1 {
		t.Errorf("expected exactly one Refs: line, got %q", got)
	}
}

func TestGenerateCommitMessage_HistoryFailure_DegradesToNoStyleContext(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, ConnectionID: "conn-1", RepoPath: "/repo/wt1"}}
	local := &fakeGitExecutor{name: "local"}
	relay := &fakeGitExecutor{name: "relay"}

	getStatus := NewGetStatus(resolver, local, relay)
	getDiff := NewGetDiff(resolver, local, relay)
	// A History wired to an executor that errors on History(...).
	erroringExecutor := &erroringHistoryExecutor{fakeGitExecutor: &fakeGitExecutor{}}
	history := NewHistory(resolver, erroringExecutor, erroringExecutor)
	completer := &fakeAICompleter{message: "feat: no history available"}
	uc := NewGenerateCommitMessage(resolver, getStatus, getDiff, history, completer)

	got, err := uc.Execute(context.Background(), GenerateCommitMessageInput{WorktreeID: "wt1"})
	if err != nil {
		t.Fatalf("expected generation to succeed despite History failure, got error: %v", err)
	}
	if got != "feat: no history available" {
		t.Errorf("unexpected message: %q", got)
	}
	if strings.Contains(completer.gotPrompt, "Recent commits on this project") {
		t.Error("expected no recent-commit section in the prompt when History fails")
	}
}

// erroringHistoryExecutor wraps fakeGitExecutor but always fails History —
// letting TestGenerateCommitMessage_HistoryFailure_DegradesToNoStyleContext
// exercise the degrade path without adding an error field to the shared
// fakeGitExecutor for a single-test need.
type erroringHistoryExecutor struct {
	*fakeGitExecutor
}

func (e *erroringHistoryExecutor) History(ctx context.Context, repoPath, baseRef string, limit int) ([]domain.CommitRef, error) {
	return nil, errors.New("history unavailable")
}
