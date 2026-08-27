// scm_client.go implements usecase.IssueSourceClient's SCM half —
// CreateWorktreeFromIssueRequest's oneof issue_source.scm_issue case
// (SOL-PI-02). Resolves an issue via scm-integration-service's ListIssues,
// filtering client-side to the requested number — that service has no
// single-issue-by-number RPC (only ListIssues(repo)), same confirmed-gap
// posture as this package's other clients.
package grpcclient

import (
	"context"
	"fmt"

	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// ScmSourceClient implements usecase.IssueSourceClient against
// scm-integration-service.
type ScmSourceClient struct {
	client scmintegrationv1.ScmIntegrationServiceClient
}

func NewScmSourceClient(client scmintegrationv1.ScmIntegrationServiceClient) *ScmSourceClient {
	return &ScmSourceClient{client: client}
}

func (c *ScmSourceClient) GetIssue(ctx context.Context, ref domain.IssueRef) (domain.Issue, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return domain.Issue{}, err
	}
	resp, err := c.client.ListIssues(ctx, &scmintegrationv1.ListIssuesRequest{
		Provider: parseScmProvider(ref.Provider), Repo: ref.Repo,
	})
	if err != nil {
		return domain.Issue{}, err
	}
	for _, iss := range resp.GetIssues() {
		if iss.GetNumber() == ref.Number {
			return domain.Issue{
				Title:    iss.GetTitle(),
				Provider: ref.Provider,
				// No description/AC/labels/comments on scmintegrationv1.Issue
				// today (id/title/state/url/number only) — buildAgentPrompt
				// degrades gracefully with empty Description/AcceptanceCriteria.
				ExternalRef: fmt.Sprintf("%s#%d", ref.Repo, ref.Number),
			}, nil
		}
	}
	return domain.Issue{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_FROM_ISSUE_ISSUE_NOT_FOUND", "issue not found", nil)
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
