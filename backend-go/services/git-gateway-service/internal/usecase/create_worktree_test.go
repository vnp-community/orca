package usecase

import (
	"errors"
	"strings"
	"testing"

	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func TestCreateWorktree_HappyPath(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{createWorktreeResult: domain.WorktreeCreateResult{Path: "/repo-feature", HeadSHA: "sha123"}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{recordCreatedResult: domain.WorktreeRecord{ID: "wt-1", Path: "/repo-feature", Branch: "feature"}}
	uc := NewCreateWorktree(resolver, projects, local, relay)

	got, err := uc.Execute(context.Background(), CreateWorktreeInput{ProjectID: "proj-1", RepoID: "repo-1", Branch: "feature", BaseRef: "main"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WorktreeID != "wt-1" || got.Path != "/repo-feature" || got.HeadSHA != "sha123" {
		t.Errorf("unexpected result: %+v", got)
	}
	if !projects.calledRecordCreated {
		t.Error("expected RecordWorktreeCreated to be called")
	}
	if local.calledRemoveWorktree {
		t.Error("expected no compensation on the happy path")
	}
}

func TestCreateWorktree_BookkeepingFails_CompensatesByRemovingWorktree(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{createWorktreeResult: domain.WorktreeCreateResult{Path: "/repo-feature", HeadSHA: "sha123"}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{recordCreatedErr: errors.New("project-service unreachable")}
	uc := NewCreateWorktree(resolver, projects, local, relay)

	_, err := uc.Execute(context.Background(), CreateWorktreeInput{ProjectID: "proj-1", RepoID: "repo-1", Branch: "feature", BaseRef: "main"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_BOOKKEEPING_FAILED" {
		t.Fatalf("expected WORKTREE_BOOKKEEPING_FAILED, got %v", err)
	}
	if local.removeWorktreeCallCount != 1 {
		t.Fatalf("expected exactly one compensating RemoveWorktree call, got %d", local.removeWorktreeCallCount)
	}
	if local.gotRemoveWorktreePath != "/repo-feature" {
		t.Errorf("expected compensation to target the created path, got %q", local.gotRemoveWorktreePath)
	}
	if !local.gotRemoveWorktreeForce {
		t.Error("expected compensation to force-remove the worktree")
	}
}

func TestCreateWorktree_BookkeepingFailsAndCompensationFails_ReportsBothFailures(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{
		createWorktreeResult: domain.WorktreeCreateResult{Path: "/repo-feature", HeadSHA: "sha123"},
		removeWorktreeErr:    errors.New("rollback failed: disk busy"),
	}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{recordCreatedErr: errors.New("bookkeeping unreachable")}
	uc := NewCreateWorktree(resolver, projects, local, relay)

	_, err := uc.Execute(context.Background(), CreateWorktreeInput{ProjectID: "proj-1", RepoID: "repo-1", Branch: "feature", BaseRef: "main"})
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "bookkeeping unreachable") {
		t.Errorf("expected error to name the bookkeeping failure, got %q", msg)
	}
	if !strings.Contains(msg, "rollback failed: disk busy") {
		t.Errorf("expected error to name the rollback failure, got %q", msg)
	}
	if !strings.Contains(msg, "/repo-feature") {
		t.Errorf("expected error to name the orphaned path, got %q", msg)
	}
}

func TestCreateWorktree_GitCreateFails_NoBookkeepingOrCompensationAttempted(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{createWorktreeErr: errors.New("git worktree add failed: branch exists")}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	uc := NewCreateWorktree(resolver, projects, local, relay)

	_, err := uc.Execute(context.Background(), CreateWorktreeInput{ProjectID: "proj-1", RepoID: "repo-1", Branch: "feature", BaseRef: "main"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_CREATE_FAILED" {
		t.Fatalf("expected WORKTREE_CREATE_FAILED, got %v", err)
	}
	if projects.calledRecordCreated {
		t.Error("expected RecordWorktreeCreated NOT to be called when git worktree add itself fails")
	}
	if local.calledRemoveWorktree {
		t.Error("expected no compensation attempt when there was nothing to compensate")
	}
}

func TestCreateWorktree_IdempotencyKeyMatch_ReturnsExistingWithoutExecutorCall(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{
		findByIdempotencyKeyFound:  true,
		findByIdempotencyKeyResult: domain.WorktreeRecord{ID: "wt-existing", Path: "/repo-feature", Branch: "feature"},
	}
	uc := NewCreateWorktree(resolver, projects, local, relay)

	got, err := uc.Execute(context.Background(), CreateWorktreeInput{
		ProjectID: "proj-1", RepoID: "repo-1", Branch: "feature", BaseRef: "main", IdempotencyKey: "dedupe-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WorktreeID != "wt-existing" || got.Path != "/repo-feature" {
		t.Errorf("expected the existing worktree to be returned, got %+v", got)
	}
	if !projects.calledFindByIdempotencyKey {
		t.Error("expected FindWorktreeByIdempotencyKey to be called")
	}
	if projects.calledGetRepo {
		t.Error("expected GetRepo NOT to be called on an idempotency match")
	}
	if local.calledCreateWorktree || relay.calledCreateWorktree {
		t.Error("expected no GitExecutor method to be called on an idempotency match")
	}
	if projects.calledRecordCreated {
		t.Error("expected RecordWorktreeCreated NOT to be called on an idempotency match")
	}
}

func TestCreateWorktree_RepoNotFound_NoExecutorCallAtAll(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getRepoErr: errors.New("repo not found")}
	uc := NewCreateWorktree(resolver, projects, local, relay)

	_, err := uc.Execute(context.Background(), CreateWorktreeInput{ProjectID: "proj-1", RepoID: "repo-1", Branch: "feature", BaseRef: "main"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindNotFound || ae.Code != "WORKTREE_REPO_NOT_FOUND" {
		t.Fatalf("expected WORKTREE_REPO_NOT_FOUND (KindNotFound), got %v", err)
	}
	if local.calledCreateWorktree || relay.calledCreateWorktree {
		t.Error("expected no GitExecutor method to be called when the repo lookup fails")
	}
}
