package grpcclient

import (
	"context"
	"errors"
	"time"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// reviewDiffTimeout bounds one BranchCompare: over SSH git can be slow, and a person is waiting for the review.
const reviewDiffTimeout = 20 * time.Second

// GitReviewDiffReader answers usecase.ReviewDiffReader with git-gateway's BranchCompare, which gives the changed files with the
// lines added and removed against the merge-base of the worktree's branch and the base ref.
type GitReviewDiffReader struct {
	git     gitgatewayv1.GitGatewayServiceClient
	timeout time.Duration
}

var _ usecase.ReviewDiffReader = (*GitReviewDiffReader)(nil)

func NewGitReviewDiffReader(git gitgatewayv1.GitGatewayServiceClient) *GitReviewDiffReader {
	return &GitReviewDiffReader{git: git, timeout: reviewDiffTimeout}
}

func (r *GitReviewDiffReader) BranchCompare(ctx context.Context, worktreeID, baseRef string) (usecase.ReviewDiff, error) {
	if r == nil || r.git == nil {
		return usecase.ReviewDiff{}, errors.New("git-gateway is not configured")
	}
	cctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	gctx, err := withTenantMetadata(cctx)
	if err != nil {
		return usecase.ReviewDiff{}, err
	}
	resp, err := r.git.BranchCompare(gctx, &gitgatewayv1.BranchCompareRequest{WorktreeId: worktreeID, BaseRef: baseRef})
	if err != nil {
		return usecase.ReviewDiff{}, err
	}
	out := usecase.ReviewDiff{
		Status: resp.GetStatus(), BaseOID: resp.GetBaseOid(), HeadOID: resp.GetHeadOid(), CompareRef: resp.GetCompareRef(), MergeBase: resp.GetMergeBase(),
	}
	for _, e := range resp.GetEntries() {
		out.Entries = append(out.Entries, domain.ChangedPath{Path: e.GetPath(), OldPath: e.GetOldPath(), Status: e.GetStatus(), Added: int(e.GetAdded()), Removed: int(e.GetRemoved())})
	}
	if out.Status != "ready" && resp.GetErrorMessage() != "" {
		return out, errors.New(resp.GetErrorMessage())
	}
	return out, nil
}
