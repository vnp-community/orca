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

	recordRemovedErr    error
	calledRecordRemoved bool
	gotRecordRemovedID  string
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

func (f *fakeProjectClient) RecordWorktreeCreated(ctx context.Context, projectID, repoID, path, branch string) (domain.WorktreeRecord, error) {
	f.calledRecordCreated = true
	f.gotRecordCreatedProject = projectID
	f.gotRecordCreatedRepo = repoID
	f.gotRecordCreatedPath = path
	f.gotRecordCreatedBranch = branch
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
