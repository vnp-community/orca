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
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{createWorktreeResult: domain.WorktreeCreateResult{Path: "/repo-feature", HeadSHA: "sha123"}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{URL: "/repo"}, recordCreatedResult: domain.WorktreeRecord{ID: "wt-1", Path: "/repo-feature", Branch: "feature"}}
	uc := NewCreateWorktree(reachability, projects, local, relay)

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

// TestCreateWorktree_UsesRepoProjectIDNotWireProjectID locks in that
// RecordWorktreeCreated is bookkept under the repo's OWN project (resolved
// via GetRepo), not the wire's CreateWorktreeInput.ProjectID — no real
// caller (wscompat's worktree.create) ever sends project_id on this RPC, so
// trusting the wire field silently recorded every worktree under an empty
// project id.
func TestCreateWorktree_UsesRepoProjectIDNotWireProjectID(t *testing.T) {
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{createWorktreeResult: domain.WorktreeCreateResult{Path: "/repo-feature", HeadSHA: "sha123"}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{
		getRepoResult:       domain.RepoInfo{URL: "/repo", ProjectID: "proj-from-repo"},
		recordCreatedResult: domain.WorktreeRecord{ID: "wt-1", Path: "/repo-feature", Branch: "feature"},
	}
	uc := NewCreateWorktree(reachability, projects, local, relay)

	// ProjectID deliberately left empty on the input, matching every real
	// caller (wscompat's worktree.create never decodes/sends it).
	_, err := uc.Execute(context.Background(), CreateWorktreeInput{RepoID: "repo-1", Branch: "feature", BaseRef: "main"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if projects.gotRecordCreatedProject != "proj-from-repo" {
		t.Errorf("expected RecordWorktreeCreated to use the repo's own project id, got %q", projects.gotRecordCreatedProject)
	}
}

func TestCreateWorktree_ThreadsLineageThroughToRecordWorktreeCreated(t *testing.T) {
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{createWorktreeResult: domain.WorktreeCreateResult{Path: "/repo-feature", HeadSHA: "sha123"}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{URL: "/repo"}, recordCreatedResult: domain.WorktreeRecord{ID: "wt-1", Path: "/repo-feature", Branch: "feature"}}
	uc := NewCreateWorktree(reachability, projects, local, relay)

	lineage := domain.WorktreeLineageCapture{ParentWorktreeID: "wt-parent", Origin: "orchestration", TaskID: "task_abc123"}
	_, err := uc.Execute(context.Background(), CreateWorktreeInput{
		ProjectID: "proj-1", RepoID: "repo-1", Branch: "feature", BaseRef: "main", Lineage: lineage,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if projects.gotRecordCreatedLineage != lineage {
		t.Errorf("expected lineage %+v to reach RecordWorktreeCreated, got %+v", lineage, projects.gotRecordCreatedLineage)
	}
}

func TestCreateWorktree_BookkeepingFails_CompensatesByRemovingWorktree(t *testing.T) {
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{createWorktreeResult: domain.WorktreeCreateResult{Path: "/repo-feature", HeadSHA: "sha123"}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{URL: "/repo"}, recordCreatedErr: errors.New("project-service unreachable")}
	uc := NewCreateWorktree(reachability, projects, local, relay)

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
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{
		createWorktreeResult: domain.WorktreeCreateResult{Path: "/repo-feature", HeadSHA: "sha123"},
		removeWorktreeErr:    errors.New("rollback failed: disk busy"),
	}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{URL: "/repo"}, recordCreatedErr: errors.New("bookkeeping unreachable")}
	uc := NewCreateWorktree(reachability, projects, local, relay)

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
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{createWorktreeErr: errors.New("git worktree add failed: branch exists")}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{URL: "/repo"}}
	uc := NewCreateWorktree(reachability, projects, local, relay)

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
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{
		findByIdempotencyKeyFound:  true,
		findByIdempotencyKeyResult: domain.WorktreeRecord{ID: "wt-existing", Path: "/repo-feature", Branch: "feature"},
	}
	uc := NewCreateWorktree(reachability, projects, local, relay)

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
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getRepoErr: errors.New("repo not found")}
	uc := NewCreateWorktree(reachability, projects, local, relay)

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

// ── SOL-WT-01: BR-WT-01/04, [A1]/[A2]/[A3] ──────────────────────────────────

func TestCreateWorktree_InvalidName_RejectsBeforeAnyExecutorCall(t *testing.T) {
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	uc := NewCreateWorktree(reachability, projects, local, relay)

	_, err := uc.Execute(context.Background(), CreateWorktreeInput{ProjectID: "proj-1", RepoID: "repo-1", Branch: "feature", BaseRef: "main", Name: "Invalid Name!"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_NAME_INVALID" {
		t.Fatalf("expected WORKTREE_NAME_INVALID, got %v", err)
	}
	if local.calledCreateWorktree || relay.calledCreateWorktree {
		t.Error("expected zero calls on either executor when the name is invalid")
	}
	if projects.calledGetRepo {
		t.Error("expected GetRepo NOT to be called when the name is invalid")
	}
}

func TestCreateWorktree_PathAlreadyExists_ReturnsSuggestedName_NoGitCallAttempted(t *testing.T) {
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{listWorktreePathsOut: []domain.WorktreeGitInfo{{Path: "/repo-feature"}}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{URL: "/repo"}}
	uc := NewCreateWorktree(reachability, projects, local, relay)

	_, err := uc.Execute(context.Background(), CreateWorktreeInput{ProjectID: "proj-1", RepoID: "repo-1", Branch: "feature", BaseRef: "main"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_PATH_EXISTS" {
		t.Fatalf("expected WORKTREE_PATH_EXISTS, got %v", err)
	}
	if local.calledCreateWorktree {
		t.Error("expected CreateWorktree to never be called when the target path already exists")
	}
}

// TestCreateWorktree_BranchAlreadyCheckedOutElsewhere_RejectsBeforeGitCall is
// the regression test for incident 2026-09-12: a real production
// WORKTREE_CREATE_FAILED / INFRA_AGENT_EXEC_FAILED round-trip when the
// caller wants a worktree for a branch that's already checked out in
// another worktree (very commonly the main repo clone itself, e.g. "main")
// — git's own "fatal: '<branch>' is already used by worktree at '<path>'"
// is now caught locally via the already-fetched onDisk listing, before ever
// calling the executor (and therefore before ever relaying to the agent).
func TestCreateWorktree_BranchAlreadyCheckedOutElsewhere_RejectsBeforeGitCall(t *testing.T) {
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{listWorktreePathsOut: []domain.WorktreeGitInfo{
		{Path: "/repo", Branch: "refs/heads/main"},
	}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{URL: "/repo"}}
	uc := NewCreateWorktree(reachability, projects, local, relay)

	_, err := uc.Execute(context.Background(), CreateWorktreeInput{ProjectID: "proj-1", RepoID: "repo-1", Branch: "main", BaseRef: "main"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_BRANCH_CHECKED_OUT_ELSEWHERE" {
		t.Fatalf("expected WORKTREE_BRANCH_CHECKED_OUT_ELSEWHERE, got %v", err)
	}
	if local.calledCreateWorktree {
		t.Error("expected CreateWorktree to never be called when the branch is already checked out elsewhere")
	}
}

func TestCreateWorktree_LimitExceeded_RejectsBeforeGitCall(t *testing.T) {
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	existing := make([]domain.WorktreeRecord, 20)
	for i := range existing {
		existing[i] = domain.WorktreeRecord{ID: "wt", RepoID: "repo-1", Active: true}
	}
	projects := &fakeProjectClient{listWorktreesResult: existing}
	uc := NewCreateWorktree(reachability, projects, local, relay)

	_, err := uc.Execute(context.Background(), CreateWorktreeInput{ProjectID: "proj-1", RepoID: "repo-1", Branch: "feature", BaseRef: "main"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_LIMIT_EXCEEDED" {
		t.Fatalf("expected WORKTREE_LIMIT_EXCEEDED, got %v", err)
	}
	if local.calledCreateWorktree {
		t.Error("expected CreateWorktree to never be called once the cap is exceeded")
	}
}

func TestCreateWorktree_LimitCheckFailsOpen_WhenListWorktreesErrors(t *testing.T) {
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{createWorktreeResult: domain.WorktreeCreateResult{Path: "/repo-feature", HeadSHA: "sha123"}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{listWorktreesErr: errors.New("project-service unreachable"), recordCreatedResult: domain.WorktreeRecord{ID: "wt-1", Path: "/repo-feature", Branch: "feature"}}
	uc := NewCreateWorktree(reachability, projects, local, relay)

	_, err := uc.Execute(context.Background(), CreateWorktreeInput{ProjectID: "proj-1", RepoID: "repo-1", Branch: "feature", BaseRef: "main"})
	if err != nil {
		t.Fatalf("expected creation to proceed despite the ListWorktrees error (fail open), got %v", err)
	}
	if !local.calledCreateWorktree {
		t.Error("expected CreateWorktree to still be called")
	}
}

func TestCreateWorktree_BaseRefNotFound_ClassifiesGitError_AttachesBranchList(t *testing.T) {
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{
		createWorktreeErr: errors.New("fatal: invalid reference: nonexistent"),
		branchInfos:       []domain.BranchInfo{{Name: "main"}, {Name: "develop"}},
	}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	uc := NewCreateWorktree(reachability, projects, local, relay)

	_, err := uc.Execute(context.Background(), CreateWorktreeInput{ProjectID: "proj-1", RepoID: "repo-1", Branch: "feature", BaseRef: "nonexistent"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_BASE_REF_NOT_FOUND" {
		t.Fatalf("expected WORKTREE_BASE_REF_NOT_FOUND, got %v", err)
	}
	if !strings.Contains(ae.Message, "main") || !strings.Contains(ae.Message, "develop") {
		t.Errorf("expected the error message to list available branches, got %q", ae.Message)
	}
}

func TestCreateWorktree_CustomNameAndPath_PassedThroughToExecutor(t *testing.T) {
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{createWorktreeResult: domain.WorktreeCreateResult{Path: "/custom/path", HeadSHA: "sha123"}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{recordCreatedResult: domain.WorktreeRecord{ID: "wt-1", Path: "/custom/path", Branch: "feature"}}
	uc := NewCreateWorktree(reachability, projects, local, relay)

	_, err := uc.Execute(context.Background(), CreateWorktreeInput{
		ProjectID: "proj-1", RepoID: "repo-1", Branch: "feature", BaseRef: "main", Name: "custom-name", Path: "/custom/path",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if local.gotCreateWorktreeTargetPath != "/custom/path" {
		t.Errorf("expected targetPath to be passed through verbatim, got %q", local.gotCreateWorktreeTargetPath)
	}
}

// TestCreateWorktree_ForwardsBaseRefToRecordWorktreeCreated is the direct
// regression guard against SOL-WT-04's confirmed silent-drop bug: BaseRef
// was received but never forwarded to RecordWorktreeCreated.
func TestCreateWorktree_ForwardsBaseRefToRecordWorktreeCreated(t *testing.T) {
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{createWorktreeResult: domain.WorktreeCreateResult{Path: "/repo-feature", HeadSHA: "sha123"}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{recordCreatedResult: domain.WorktreeRecord{ID: "wt-1", Path: "/repo-feature", Branch: "feature"}}
	uc := NewCreateWorktree(reachability, projects, local, relay)

	_, err := uc.Execute(context.Background(), CreateWorktreeInput{ProjectID: "proj-1", RepoID: "repo-1", Branch: "feature", BaseRef: "develop"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if projects.gotRecordCreatedBaseRef != "develop" {
		t.Errorf("expected in.BaseRef to be forwarded to RecordWorktreeCreated, got %q", projects.gotRecordCreatedBaseRef)
	}
}
