package grpcclient

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
)

// ScmClient implements usecase.ScmClient against scm-integration-service.
type ScmClient struct {
	client scmintegrationv1.ScmIntegrationServiceClient
}

func NewScmClient(client scmintegrationv1.ScmIntegrationServiceClient) *ScmClient {
	return &ScmClient{client: client}
}

// UpdateIssue parses ref ("owner/repo#123") and labelPatch (one of
// "close", "add:<label>", "remove:<label>" — see domain.TargetState.GitHubLabelPatch's
// doc comment) into a scmintegrationv1.UpdateIssueRequest.
func (c *ScmClient) UpdateIssue(ctx context.Context, tenantID, provider, ref, labelPatch string) error {
	repo, number, err := parseIssueRef(ref)
	if err != nil {
		return err
	}
	ctx = withTenantMetadata(ctx, tenantID)

	req := &scmintegrationv1.UpdateIssueRequest{
		TenantId: tenantID, Provider: parseScmProvider(provider), Repo: repo, Number: number,
	}
	switch {
	case labelPatch == "close":
		closed := "closed"
		req.State = &closed
	case strings.HasPrefix(labelPatch, "add:"):
		req.AddLabels = []string{strings.TrimPrefix(labelPatch, "add:")}
	case strings.HasPrefix(labelPatch, "remove:"):
		req.RemoveLabels = []string{strings.TrimPrefix(labelPatch, "remove:")}
	}
	_, err = c.client.UpdateIssue(ctx, req)
	return err
}

// GetPullRequestForBranch — see usecase.ScmClient's KNOWN GAP doc comment:
// implemented for interface conformance, no caller in sync_issue_status.go
// today (WorktreeLifecycleEvent carries no branch to call this with).
func (c *ScmClient) GetPullRequestForBranch(ctx context.Context, tenantID, provider, repo, branch string) (bool, error) {
	ctx = withTenantMetadata(ctx, tenantID)
	resp, err := c.client.GetPullRequestForBranch(ctx, &scmintegrationv1.GetPullRequestForBranchRequest{
		TenantId: tenantID, Provider: parseScmProvider(provider), Repo: repo, HeadBranch: branch,
	})
	if err != nil {
		return false, err
	}
	return resp.GetFound(), nil
}

// parseIssueRef splits "owner/repo#123" into repo="owner/repo" and
// number=123 — same shape as scm-integration-service's github adapter's
// own parseItemSlug.
func parseIssueRef(ref string) (repo string, number int32, err error) {
	idx := strings.LastIndex(ref, "#")
	if idx < 0 {
		return "", 0, fmt.Errorf("grpcclient: invalid issue ref %q, want owner/repo#number", ref)
	}
	n, err := strconv.Atoi(ref[idx+1:])
	if err != nil {
		return "", 0, fmt.Errorf("grpcclient: invalid issue ref %q: %w", ref, err)
	}
	return ref[:idx], int32(n), nil
}

func parseScmProvider(provider string) scmintegrationv1.ScmProvider {
	switch provider {
	case "github":
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB
	case "gitlab":
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_GITLAB
	default:
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_UNSPECIFIED
	}
}
