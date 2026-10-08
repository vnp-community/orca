// Package grpcclient implements issue-status-sync's outbound ports against
// issue-tracking-service, scm-integration-service, and project-service —
// same withTenantMetadata-style pattern every other grpcclient package in
// this codebase already uses (see e.g. git-gateway-service's
// internal/adapter/grpcclient).
package grpcclient

import (
	"context"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
)

// IssueTrackingClient implements usecase.IssueTrackerClient against
// issue-tracking-service's UpdateIssue RPC.
type IssueTrackingClient struct {
	client issuetrackingv1.IssueTrackingServiceClient
}

func NewIssueTrackingClient(client issuetrackingv1.IssueTrackingServiceClient) *IssueTrackingClient {
	return &IssueTrackingClient{client: client}
}

// TransitionIssue calls UpdateIssue with workflow_state_id=state, the target
// status name — issue-tracking-service's Jira adapter resolves it to one of
// the issue's currently available transitions and errors if there is none.
func (c *IssueTrackingClient) TransitionIssue(ctx context.Context, tenantID, userID, provider, ref, site, state string) error {
	ctx = withIdentityMetadata(ctx, tenantID, userID)
	_, err := c.client.UpdateIssue(ctx, &issuetrackingv1.UpdateIssueRequest{
		Provider: parseTrackerProvider(provider), IssueId: ref, WorkflowStateId: state, WorkspaceId: site,
	})
	return err
}

// IssueStatusCategory reads the issue's current status category through GetIssue.
func (c *IssueTrackingClient) IssueStatusCategory(ctx context.Context, tenantID, userID, provider, ref, site string) (string, error) {
	ctx = withIdentityMetadata(ctx, tenantID, userID)
	issue, err := c.client.GetIssue(ctx, &issuetrackingv1.GetIssueRequest{
		Provider: parseTrackerProvider(provider), IssueId: ref, WorkspaceId: site,
	})
	if err != nil {
		return "", err
	}
	return issue.GetWorkflowState().GetCategory(), nil
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

// AddComment implements usecase.IssueCommenter through AddIssueComment.
func (c *IssueTrackingClient) AddComment(ctx context.Context, tenantID, userID, provider, ref, site, bodyMarkdown string) error {
	ctx = withIdentityMetadata(ctx, tenantID, userID)
	_, err := c.client.AddIssueComment(ctx, &issuetrackingv1.AddIssueCommentRequest{
		Provider: parseTrackerProvider(provider), IssueId: ref, BodyMarkdown: bodyMarkdown, WorkspaceId: site,
	})
	return err
}
