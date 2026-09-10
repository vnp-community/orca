package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// fakeProjectClient is an in-memory ProjectClient — shared by
// create_worktree_test.go/remove_worktree_test.go/resolve_pr_base_test.go/
// resolve_mr_base_test.go, per specs/backend-go/standards/testing-strategy.md's
// unit-test section (never a real project-service call).
type fakeProjectClient struct {
	getRepoResult domain.RepoInfo
	getRepoErr    error
	calledGetRepo bool
	gotGetRepoID  string

	recordCreatedResult     domain.WorktreeRecord
	recordCreatedErr        error
	calledRecordCreated     bool
	gotRecordCreatedProject string
	gotRecordCreatedRepo    string
	gotRecordCreatedPath    string
	gotRecordCreatedBranch  string
	gotRecordCreatedBaseRef string
	gotRecordCreatedLineage domain.WorktreeLineageCapture

	recordRemovedErr    error
	calledRecordRemoved bool
	gotRecordRemovedID  string

	findByIdempotencyKeyResult domain.WorktreeRecord
	findByIdempotencyKeyFound  bool
	findByIdempotencyKeyErr    error
	calledFindByIdempotencyKey bool

	issueStatusSyncEnabled    bool
	issueStatusSyncEnabledErr error

	listWorktreesResult []domain.WorktreeRecord
	listWorktreesErr    error
	calledListWorktrees bool

	getWorktreeResult domain.WorktreeInfo
	getWorktreeErr    error
	calledGetWorktree bool
	gotGetWorktreeID  string
}

func (f *fakeProjectClient) GetRepo(ctx context.Context, repoID string) (domain.RepoInfo, error) {
	f.calledGetRepo = true
	f.gotGetRepoID = repoID
	if f.getRepoErr != nil {
		return domain.RepoInfo{}, f.getRepoErr
	}
	if f.getRepoResult != (domain.RepoInfo{}) {
		return f.getRepoResult, nil
	}
	return domain.RepoInfo{ID: repoID}, nil
}

func (f *fakeProjectClient) RecordWorktreeCreated(ctx context.Context, projectID, repoID, path, branch, baseRef string, lineage domain.WorktreeLineageCapture) (domain.WorktreeRecord, error) {
	f.calledRecordCreated = true
	f.gotRecordCreatedProject = projectID
	f.gotRecordCreatedRepo = repoID
	f.gotRecordCreatedPath = path
	f.gotRecordCreatedBranch = branch
	f.gotRecordCreatedBaseRef = baseRef
	f.gotRecordCreatedLineage = lineage
	if f.recordCreatedErr != nil {
		return domain.WorktreeRecord{}, f.recordCreatedErr
	}
	return f.recordCreatedResult, nil
}

func (f *fakeProjectClient) RecordWorktreeRemoved(ctx context.Context, worktreeID string) error {
	f.calledRecordRemoved = true
	f.gotRecordRemovedID = worktreeID
	return f.recordRemovedErr
}

func (f *fakeProjectClient) FindWorktreeByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (domain.WorktreeRecord, bool, error) {
	f.calledFindByIdempotencyKey = true
	if f.findByIdempotencyKeyErr != nil {
		return domain.WorktreeRecord{}, false, f.findByIdempotencyKeyErr
	}
	return f.findByIdempotencyKeyResult, f.findByIdempotencyKeyFound, nil
}

func (f *fakeProjectClient) IsIssueStatusSyncEnabled(ctx context.Context, projectID string) (bool, error) {
	if f.issueStatusSyncEnabledErr != nil {
		return false, f.issueStatusSyncEnabledErr
	}
	return f.issueStatusSyncEnabled, nil
}

func (f *fakeProjectClient) ListWorktrees(ctx context.Context, projectID string) ([]domain.WorktreeRecord, error) {
	f.calledListWorktrees = true
	if f.listWorktreesErr != nil {
		return nil, f.listWorktreesErr
	}
	return f.listWorktreesResult, nil
}

func (f *fakeProjectClient) GetWorktree(ctx context.Context, worktreeID string) (domain.WorktreeInfo, error) {
	f.calledGetWorktree = true
	f.gotGetWorktreeID = worktreeID
	if f.getWorktreeErr != nil {
		return domain.WorktreeInfo{}, f.getWorktreeErr
	}
	if f.getWorktreeResult != (domain.WorktreeInfo{}) {
		return f.getWorktreeResult, nil
	}
	return domain.WorktreeInfo{ID: worktreeID}, nil
}

// fakeScrollbackCleaner is an in-memory ScrollbackCleaner — used by
// remove_worktree_test.go to assert RemoveWorktree's best-effort cleanup
// call is made with the removed worktree's ID, and that a cleanup RPC
// failure does not fail RemoveWorktree itself.
type fakeScrollbackCleaner struct {
	err           error
	called        bool
	gotWorktreeID string
}

func (f *fakeScrollbackCleaner) DeleteTerminalScrollbackSnapshots(ctx context.Context, worktreeID string) error {
	f.called = true
	f.gotWorktreeID = worktreeID
	return f.err
}

// fakeTerminalSessionLister is an in-memory TerminalSessionLister — shared
// by remove_worktree_test.go and check_worktree_delete_safety_test.go.
type fakeTerminalSessionLister struct {
	sessions     []domain.TerminalSessionRef
	listErr      error
	killErr      error
	calledList   bool
	gotConnID    string
	killedPtyIDs []string
}

func (f *fakeTerminalSessionLister) ListSessions(ctx context.Context, connectionID string) ([]domain.TerminalSessionRef, error) {
	f.calledList = true
	f.gotConnID = connectionID
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.sessions, nil
}

func (f *fakeTerminalSessionLister) Kill(ctx context.Context, ptyID string) error {
	if f.killErr != nil {
		return f.killErr
	}
	f.killedPtyIDs = append(f.killedPtyIDs, ptyID)
	return nil
}

// fakeSCMClient is an in-memory SCMClient — shared by resolve_pr_base_test.go
// and resolve_mr_base_test.go.
type fakeSCMClient struct {
	prBaseBranch    string
	prBaseSHA       string
	prBaseErr       error
	calledGetPRBase bool

	mrBaseBranch    string
	mrBaseSHA       string
	mrBaseErr       error
	calledGetMRBase bool

	prForBranch          PullRequestInfo
	prForBranchFound     bool
	prForBranchErr       error
	calledGetPRForBranch bool
}

func (f *fakeSCMClient) GetPullRequestBase(ctx context.Context, repoID string, prNumber int32) (string, string, error) {
	f.calledGetPRBase = true
	if f.prBaseErr != nil {
		return "", "", f.prBaseErr
	}
	return f.prBaseBranch, f.prBaseSHA, nil
}

func (f *fakeSCMClient) GetMergeRequestBase(ctx context.Context, repoID string, mrNumber int32) (string, string, error) {
	f.calledGetMRBase = true
	if f.mrBaseErr != nil {
		return "", "", f.mrBaseErr
	}
	return f.mrBaseBranch, f.mrBaseSHA, nil
}

func (f *fakeSCMClient) GetPullRequestForBranch(ctx context.Context, tenantID, branch string) (PullRequestInfo, bool, error) {
	f.calledGetPRForBranch = true
	if f.prForBranchErr != nil {
		return PullRequestInfo{}, false, f.prForBranchErr
	}
	return f.prForBranch, f.prForBranchFound, nil
}
