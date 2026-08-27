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

	recordRemovedErr    error
	calledRecordRemoved bool
	gotRecordRemovedID  string

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

func (f *fakeProjectClient) RecordWorktreeCreated(ctx context.Context, projectID, repoID, path, branch, baseRef string) (domain.WorktreeRecord, error) {
	f.calledRecordCreated = true
	f.gotRecordCreatedProject = projectID
	f.gotRecordCreatedRepo = repoID
	f.gotRecordCreatedPath = path
	f.gotRecordCreatedBranch = branch
	f.gotRecordCreatedBaseRef = baseRef
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

// fakeTerminalSessionLister is an in-memory TerminalSessionLister — shared
// by remove_worktree_test.go and check_worktree_delete_safety_test.go.
type fakeTerminalSessionLister struct {
	sessions       []domain.TerminalSessionRef
	listErr        error
	killErr        error
	calledList     bool
	gotConnID      string
	killedPtyIDs   []string
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
