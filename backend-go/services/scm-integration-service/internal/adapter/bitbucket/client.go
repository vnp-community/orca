// Package bitbucket implements usecase.ScmProvider against Bitbucket's REST
// API — see scm-integration-service.md §7. Not implemented in this
// scaffold: satisfies the interface so the composition root and
// ProviderRegistry can register a Bitbucket entry, every method returns
// ErrNotImplemented.
//
// TODO(scm-integration-service): implement against Bitbucket Cloud's REST
// API v2 (https://developer.atlassian.com/cloud/bitbucket/rest/).
package bitbucket

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// ErrNotImplemented is returned by every method of this stub adapter.
var ErrNotImplemented = errors.New("bitbucket: not implemented")

// Client implements usecase.ScmProvider against Bitbucket's REST API. Every
// method is currently a stub — see package doc.
type Client struct{}

// New returns a Bitbucket Client.
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
