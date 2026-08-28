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

// TransitionIssue calls UpdateIssue with workflow_state_id=state — that
// field is documented as "== jira.updateIssue's transition target /
// linear's stateId" (issuetracking.proto's UpdateIssueRequest doc
// comment), matching BL-PI-03's TrackerState output exactly.
func (c *IssueTrackingClient) TransitionIssue(ctx context.Context, tenantID, provider, ref, state string) error {
	ctx = withTenantMetadata(ctx, tenantID)
	_, err := c.client.UpdateIssue(ctx, &issuetrackingv1.UpdateIssueRequest{
		Provider: parseTrackerProvider(provider), IssueId: ref, WorkflowStateId: state,
	})
	return err
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
