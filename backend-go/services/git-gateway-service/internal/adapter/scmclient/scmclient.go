// Package scmclient implements usecase.SCMClient against
// scm-integration-service. KNOWN GAP (see usecase.SCMClient's doc
// comment): scm-integration-service's current proto has no RPC to fetch a
// single PR/MR's base branch by number, so both methods below return a
// typed apperrors.KindInternal error until that RPC exists — this makes
// ResolvePrBase/ResolveMrBase fail cleanly (a typed, catchable error) at
// their call sites rather than blocking this task on an out-of-scope
// proto addition.
//
// Deviation from TASK-193's original sketch: apperrors.Kind has no
// KindUnimplemented value (common/apperrors/apperrors.go's Kind enum is
// KindUnknown/KindNotFound/KindAlreadyExists/KindInvalidArgument/
// KindPermissionDenied/KindFailedPrecondition/KindUnauthenticated/
// KindInternal) — using KindInternal instead, per the task's own
// documented fallback instruction.
package scmclient

import (
	"context"

	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"

	"github.com/stablyai/orca-go/common/apperrors"
)

type Client struct {
	client scmintegrationv1.ScmIntegrationServiceClient
}

func New(client scmintegrationv1.ScmIntegrationServiceClient) *Client {
	return &Client{client: client}
}

func (c *Client) GetPullRequestBase(ctx context.Context, repoID string, prNumber int32) (string, string, error) {
	return "", "", apperrors.New(apperrors.KindInternal, "WORKTREE_SCM_GET_PR_BASE_UNIMPLEMENTED",
		"scm-integration-service has no RPC to resolve a single PR's base branch yet", nil)
}

func (c *Client) GetMergeRequestBase(ctx context.Context, repoID string, mrNumber int32) (string, string, error) {
	return "", "", apperrors.New(apperrors.KindInternal, "WORKTREE_SCM_GET_MR_BASE_UNIMPLEMENTED",
		"scm-integration-service has no RPC to resolve a single MR's base branch yet", nil)
}
