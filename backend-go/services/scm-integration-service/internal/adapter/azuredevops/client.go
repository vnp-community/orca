// Package azuredevops implements usecase.ScmProvider against Azure DevOps'
// REST API — see scm-integration-service.md §7. Not implemented in this
// scaffold: satisfies the interface so the composition root and
// ProviderRegistry can register an Azure DevOps entry, every method returns
// ErrNotImplemented.
//
// TODO(scm-integration-service): implement against Azure DevOps Services
// REST API (https://learn.microsoft.com/en-us/rest/api/azure/devops/).
package azuredevops

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// ErrNotImplemented is returned by every method of this stub adapter.
var ErrNotImplemented = errors.New("azuredevops: not implemented")

// Client implements usecase.ScmProvider against Azure DevOps' REST API.
// Every method is currently a stub — see package doc.
type Client struct{}

// New returns an Azure DevOps Client.
func New() *Client {
	return &Client{}
}

var _ usecase.ScmProvider = (*Client)(nil)

func (c *Client) ListIssues(_ context.Context, _ usecase.Credential, _ string, _ usecase.IssueFilter) ([]domain.Issue, error) {
	return nil, ErrNotImplemented
}

func (c *Client) CreatePullRequest(_ context.Context, _ usecase.Credential, _ string, _ usecase.CreatePullRequestInput) (domain.PullRequest, error) {
	return domain.PullRequest{}, ErrNotImplemented
}

func (c *Client) ListPullRequests(_ context.Context, _ usecase.Credential, _ string) ([]domain.PullRequest, error) {
	return nil, ErrNotImplemented
}

func (c *Client) GetRateLimitStatus(_ context.Context, _ usecase.Credential) (domain.RateLimitStatus, error) {
	return domain.RateLimitStatus{}, ErrNotImplemented
}
