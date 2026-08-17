// Package gitlab implements usecase.ScmProvider against GitLab's REST API
// (https://gitlab.com/api/v4) — no glab CLI, no shared keychain, per
// scm-integration-service.md §10. Fully stubbed in this scaffold; see
// this service's README "What's stubbed" section.
//
// TODO(scm-integration-service): implement all four methods for real
// against GitLab's REST API — GET /projects/{id}/issues for ListIssues,
// POST /projects/{id}/merge_requests for CreatePullRequest,
// GET /projects/{id}/merge_requests for ListPullRequests, and GitLab's
// RateLimit-* response headers (one bucket per token, unlike GitHub's
// several) for GetRateLimitStatus. See scm-integration-service.md §8 for
// the rate-limit-header-parsing requirement.
package gitlab

import (
	"context"
	"errors"
	"net/http"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// DefaultBaseURL is GitLab.com's public REST API v4 root.
const DefaultBaseURL = "https://gitlab.com/api/v4"

// ErrNotImplemented is returned by every method of this stub adapter.
var ErrNotImplemented = errors.New("gitlab: not implemented")

// Client implements usecase.ScmProvider against GitLab's REST API. Every
// method is currently a stub — see package doc.
type Client struct {
	baseURL string
}

// New returns a GitLab Client. An empty baseURL defaults to DefaultBaseURL.
// httpClient is accepted (but unused while every method is a stub) for
// signature symmetry with internal/adapter/github, so wiring in the real
// HTTP calls later doesn't change main.go's call site.
func New(httpClient *http.Client, baseURL string) *Client {
	_ = httpClient
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{baseURL: baseURL}
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
