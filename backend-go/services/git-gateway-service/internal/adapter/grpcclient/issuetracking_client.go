// issuetracking_client.go implements usecase.IssueSourceClient's tracker
// half — CreateWorktreeFromIssueRequest's oneof issue_source.tracker_issue
// case (SOL-PI-02), against issue-tracking-service's real GetIssue RPC
// (proto/orca/issuetracking/v1/issuetracking.proto).
package grpcclient

import (
	"context"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// IssueTrackingSourceClient implements usecase.IssueSourceClient against
// issue-tracking-service.
type IssueTrackingSourceClient struct {
	client issuetrackingv1.IssueTrackingServiceClient
}

func NewIssueTrackingSourceClient(client issuetrackingv1.IssueTrackingServiceClient) *IssueTrackingSourceClient {
	return &IssueTrackingSourceClient{client: client}
}

func (c *IssueTrackingSourceClient) GetIssue(ctx context.Context, ref domain.IssueRef) (domain.Issue, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return domain.Issue{}, err
	}
	resp, err := c.client.GetIssue(ctx, &issuetrackingv1.GetIssueRequest{
		Provider: parseTrackerProvider(ref.Provider), IssueId: ref.TrackerRef,
	})
	if err != nil {
		return domain.Issue{}, err
	}
	comments := make([]string, 0) // issue-tracking-service's GetIssue doesn't inline comments; ListIssueComments is a separate RPC, out of this saga's scope
	return domain.Issue{
		Title:              resp.GetTitle(),
		Description:        resp.GetDescriptionMarkdown(),
		AcceptanceCriteria: "", // no dedicated field on issuetrackingv1.Issue
		Labels:             resp.GetLabels(),
		Comments:           comments,
		Provider:           ref.Provider,
		ExternalRef:        issueExternalRef(resp),
	}, nil
}

// issueExternalRef prefers the provider-native key ("ENG-123") over the
// raw provider_issue_id — matches Worktree.linked_issue_ref's documented
// shape ("ENG-123" for tracker issues).
func issueExternalRef(iss *issuetrackingv1.Issue) string {
	if iss.GetKey() != "" {
		return iss.GetKey()
	}
	return iss.GetProviderIssueId()
}

func parseTrackerProvider(provider string) issuetrackingv1.IssueProvider {
	switch provider {
	case "jira":
		return issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA
	case "linear":
		return issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR
	default:
		return issuetrackingv1.IssueProvider_ISSUE_PROVIDER_UNSPECIFIED
	}
}
