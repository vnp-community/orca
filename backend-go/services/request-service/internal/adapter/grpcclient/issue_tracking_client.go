package grpcclient

import (
	"context"
	"fmt"
	"time"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const issueFetchTimeout = 10 * time.Second

// IssueTrackingClient reads one Jira or Linear issue. Credentials live per (tenant, user),
// so the acting user is forwarded as well as the tenant.
type IssueTrackingClient struct {
	client issuetrackingv1.IssueTrackingServiceClient
}

var _ usecase.IssueFetcher = (*IssueTrackingClient)(nil)

func NewIssueTrackingClient(c issuetrackingv1.IssueTrackingServiceClient) *IssueTrackingClient {
	return &IssueTrackingClient{client: c}
}

func (c *IssueTrackingClient) GetIssue(ctx context.Context, provider domain.SourceProvider, ref, site string) (usecase.IssueSnapshot, error) {
	var p issuetrackingv1.IssueProvider
	switch provider {
	case domain.SourceProviderJira:
		p = issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA
	case domain.SourceProviderLinear:
		p = issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR
	default:
		return usecase.IssueSnapshot{}, fmt.Errorf("grpcclient: unsupported issue provider %q", provider)
	}
	ctx, err := withIdentityMetadata(ctx)
	if err != nil {
		return usecase.IssueSnapshot{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, issueFetchTimeout)
	defer cancel()
	issue, err := c.client.GetIssue(ctx, &issuetrackingv1.GetIssueRequest{Provider: p, IssueId: ref, WorkspaceId: site})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return usecase.IssueSnapshot{}, usecase.ErrIssueNotFound
		}
		return usecase.IssueSnapshot{}, fmt.Errorf("grpcclient: get issue: %w", err)
	}
	return usecase.IssueSnapshot{
		Title: issue.GetTitle(),
		Body:  issue.GetDescriptionMarkdown(),
		URL:   issue.GetUrl(),
		Hints: domain.SourceHints{
			IssueType: issue.GetIssueType().GetName(),
			Labels:    issue.GetLabels(),
			Priority:  issue.GetPriority().GetName(),
		},
	}, nil
}

// UnavailableIssueFetcher keeps the service startable without ISSUE_TRACKING_SERVICE_ADDR:
// title-less Jira/Linear intake then fails with REQUEST_SOURCE_NOT_FOUND.
type UnavailableIssueFetcher struct{}

func (UnavailableIssueFetcher) GetIssue(context.Context, domain.SourceProvider, string, string) (usecase.IssueSnapshot, error) {
	return usecase.IssueSnapshot{}, usecase.ErrIssueNotFound
}
