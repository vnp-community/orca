package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

type fakeCleanupProjectClient struct {
	worktrees      []WorktreeInfo
	err            error
	gotProjectID   string
	gotStatusIn    []string
	gotOlderThan   time.Time
	calledListWork bool
}

func (f *fakeCleanupProjectClient) ListWorktrees(ctx context.Context, projectID string, statusIn []string, olderThan time.Time) ([]WorktreeInfo, error) {
	f.calledListWork = true
	f.gotProjectID = projectID
	f.gotStatusIn = statusIn
	f.gotOlderThan = olderThan
	if f.err != nil {
		return nil, f.err
	}
	return f.worktrees, nil
}

type fakeCleanupGitGatewayClient struct {
	// blockedWorktreeIDs simulate a BR-AT-11/BR-AT-12 rejection
	// (ErrWorktreeRemovalBlocked) for the named worktree IDs.
	blockedWorktreeIDs map[string]bool
	// failWorktreeIDs simulate a genuine (non-safety-check) removal
	// failure.
	failWorktreeIDs map[string]bool

	calls []string // worktree IDs RemoveWorktree was called with, in order
}

func (f *fakeCleanupGitGatewayClient) RemoveWorktree(ctx context.Context, worktreeID string, force, allowOpenPR bool) error {
	f.calls = append(f.calls, worktreeID)
	if force || allowOpenPR {
		return fmt.Errorf("unexpected: cleanup_worktrees must always call with force=false, allow_open_pr=false, got force=%v allow_open_pr=%v", force, allowOpenPR)
	}
	if f.blockedWorktreeIDs[worktreeID] {
		return fmt.Errorf("%w: worktree %s has uncommitted changes or an open PR", ErrWorktreeRemovalBlocked, worktreeID)
	}
	if f.failWorktreeIDs[worktreeID] {
		return errors.New("transport error: git-gateway-service unreachable")
	}
	return nil
}

type fakeCleanupAuditWriter struct {
	calledRunID string
	calledWith  []CleanupEntry
	callCount   int
	err         error
}

func (f *fakeCleanupAuditWriter) WriteCleanupReport(ctx context.Context, runID string, entries []CleanupEntry) error {
	f.callCount++
	f.calledRunID = runID
	f.calledWith = entries
	return f.err
}

func TestCleanupWorktreesStepExecutor_MixedBatch(t *testing.T) {
	projects := &fakeCleanupProjectClient{worktrees: []WorktreeInfo{
		{ID: "wt-clean-1"}, {ID: "wt-clean-2"}, {ID: "wt-dirty"}, {ID: "wt-open-pr"}, {ID: "wt-error"},
	}}
	gitgw := &fakeCleanupGitGatewayClient{
		blockedWorktreeIDs: map[string]bool{"wt-dirty": true, "wt-open-pr": true},
		failWorktreeIDs:    map[string]bool{"wt-error": true},
	}
	audit := &fakeCleanupAuditWriter{}
	exec := NewCleanupWorktreesStepExecutor(projects, gitgw, audit)

	cfg, _ := json.Marshal(CleanupWorktreesConfig{ProjectID: "proj-1", RunID: "run-1"})
	result, err := exec.Execute(context.Background(), string(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var summary cleanupSummary
	if err := json.Unmarshal([]byte(result.OutputJSON), &summary); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if summary.Deleted != 2 {
		t.Errorf("expected deleted=2, got %d", summary.Deleted)
	}
	if summary.Skipped != 3 {
		t.Errorf("expected skipped=3, got %d", summary.Skipped)
	}
	if len(gitgw.calls) != 5 {
		t.Errorf("expected RemoveWorktree called once per candidate (5), got %d", len(gitgw.calls))
	}

	byID := map[string]CleanupEntry{}
	for _, e := range summary.Entries {
		byID[e.WorktreeID] = e
	}
	if byID["wt-clean-1"].Action != "deleted" || byID["wt-clean-2"].Action != "deleted" {
		t.Errorf("expected the two clean worktrees deleted, got %+v", byID)
	}
	if byID["wt-dirty"].Action != "skipped" || byID["wt-dirty"].Reason == "" {
		t.Errorf("expected wt-dirty skipped with a reason, got %+v", byID["wt-dirty"])
	}
	if byID["wt-open-pr"].Action != "skipped" {
		t.Errorf("expected wt-open-pr skipped, got %+v", byID["wt-open-pr"])
	}
	if byID["wt-error"].Action != "skipped" {
		t.Errorf("expected wt-error skipped, got %+v", byID["wt-error"])
	}

	if audit.callCount != 1 {
		t.Errorf("expected WriteCleanupReport called exactly once, got %d", audit.callCount)
	}
	if audit.calledRunID != "run-1" {
		t.Errorf("expected run_id=run-1, got %q", audit.calledRunID)
	}
	if len(audit.calledWith) != 5 {
		t.Errorf("expected 5 entries in the audit report, got %d", len(audit.calledWith))
	}
}

func TestCleanupWorktreesStepExecutor_DryRun_NeverCallsRemoveWorktree(t *testing.T) {
	projects := &fakeCleanupProjectClient{worktrees: []WorktreeInfo{{ID: "wt-1"}, {ID: "wt-2"}}}
	gitgw := &fakeCleanupGitGatewayClient{}
	audit := &fakeCleanupAuditWriter{}
	exec := NewCleanupWorktreesStepExecutor(projects, gitgw, audit)

	cfg, _ := json.Marshal(CleanupWorktreesConfig{ProjectID: "proj-1", DryRun: true, RunID: "run-1"})
	result, err := exec.Execute(context.Background(), string(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gitgw.calls) != 0 {
		t.Errorf("expected zero RemoveWorktree calls in dry-run mode, got %d", len(gitgw.calls))
	}

	var summary cleanupSummary
	if err := json.Unmarshal([]byte(result.OutputJSON), &summary); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(summary.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(summary.Entries))
	}
	for _, e := range summary.Entries {
		if e.Action != "would_delete" {
			t.Errorf("expected every entry to be would_delete in dry-run mode, got %+v", e)
		}
	}
}

func TestCleanupWorktreesStepExecutor_EmptyRunID_SkipsAuditWrite(t *testing.T) {
	projects := &fakeCleanupProjectClient{worktrees: []WorktreeInfo{{ID: "wt-1"}}}
	gitgw := &fakeCleanupGitGatewayClient{}
	audit := &fakeCleanupAuditWriter{}
	exec := NewCleanupWorktreesStepExecutor(projects, gitgw, audit)

	cfg, _ := json.Marshal(CleanupWorktreesConfig{ProjectID: "proj-1"})
	if _, err := exec.Execute(context.Background(), string(cfg)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if audit.callCount != 0 {
		t.Errorf("expected WriteCleanupReport never called when RunID is empty, got %d calls", audit.callCount)
	}
}

func TestCleanupWorktreesStepExecutor_AuditWriteFailure_DoesNotFailTheStep(t *testing.T) {
	projects := &fakeCleanupProjectClient{worktrees: []WorktreeInfo{{ID: "wt-1"}}}
	gitgw := &fakeCleanupGitGatewayClient{}
	audit := &fakeCleanupAuditWriter{err: errors.New("automation-service unreachable")}
	exec := NewCleanupWorktreesStepExecutor(projects, gitgw, audit)

	cfg, _ := json.Marshal(CleanupWorktreesConfig{ProjectID: "proj-1", RunID: "run-1"})
	result, err := exec.Execute(context.Background(), string(cfg))
	if err != nil {
		t.Fatalf("expected a report-write failure to be best-effort (no error), got %v", err)
	}
	if result.Status != domain.ResultStatusCompleted {
		t.Errorf("expected the step to still complete, got status=%v", result.Status)
	}
}

func TestCleanupWorktreesStepExecutor_ListWorktreesPassesFilters(t *testing.T) {
	projects := &fakeCleanupProjectClient{}
	gitgw := &fakeCleanupGitGatewayClient{}
	audit := &fakeCleanupAuditWriter{}
	exec := NewCleanupWorktreesStepExecutor(projects, gitgw, audit)

	before := time.Now().UTC()
	cfg, _ := json.Marshal(CleanupWorktreesConfig{ProjectID: "proj-1", StatusIn: []string{"completed"}, OlderThanHours: 24})
	if _, err := exec.Execute(context.Background(), string(cfg)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !projects.calledListWork {
		t.Fatal("expected ListWorktrees to be called")
	}
	if projects.gotProjectID != "proj-1" {
		t.Errorf("expected project_id=proj-1, got %q", projects.gotProjectID)
	}
	if len(projects.gotStatusIn) != 1 || projects.gotStatusIn[0] != "completed" {
		t.Errorf("expected status_in=[completed], got %v", projects.gotStatusIn)
	}
	wantOlderThan := before.Add(-24 * time.Hour)
	if projects.gotOlderThan.Sub(wantOlderThan).Abs() > time.Minute {
		t.Errorf("expected older_than around %v, got %v", wantOlderThan, projects.gotOlderThan)
	}
}
