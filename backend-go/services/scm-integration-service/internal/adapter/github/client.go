// Package github implements usecase.ScmProvider against GitHub's REST API
// directly via net/http — no gh CLI, no shared keychain, no shell-out, per
// scm-integration-service.md §10 (the direct fix for TS Gap 1). Only
// ListIssues is a real, working REST call in this scaffold; the rest are
// typed stubs — see this service's README "What's stubbed" section.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// DefaultBaseURL is GitHub's public REST API root.
const DefaultBaseURL = "https://api.github.com"

// ErrNotImplemented is returned by the GitHub adapter methods not yet wired
// up in this scaffold.
//
// TODO(scm-integration-service): implement CreatePullRequest/
// ListPullRequests/GetRateLimitStatus against GitHub's REST API — see
// scm-integration-service.md §3 for the required method set and §8 for the
// X-RateLimit-* header parsing / secondary-rate-limit backoff every real
// GitHub call must do.
var ErrNotImplemented = errors.New("github: not implemented")

// Client implements usecase.ScmProvider against GitHub's REST API.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// New returns a GitHub Client. A nil httpClient defaults to
// http.DefaultClient; an empty baseURL defaults to DefaultBaseURL —
// overridable for tests (httptest.Server) and GitHub Enterprise deployments.
func New(httpClient *http.Client, baseURL string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{httpClient: httpClient, baseURL: baseURL}
}

var _ usecase.ScmProvider = (*Client)(nil)

// githubIssue mirrors the fields this adapter needs from GitHub's
// GET /repos/{owner}/{repo}/issues response
// (https://docs.github.com/en/rest/issues/issues#list-repository-issues).
// GitHub's issues endpoint also returns pull requests; PullRequest being
// non-nil is how the API distinguishes them.
type githubIssue struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	State       string `json:"state"`
	HTMLURL     string `json:"html_url"`
	PullRequest *struct {
		URL string `json:"url"`
	} `json:"pull_request"`
}

// ListIssues calls GitHub's REST API for real: GET /repos/{repo}/issues,
// with the resolved per-tenant token as a Bearer credential — per
// scm-integration-service.md §9, this is the only place that token exists,
// built directly into the auth header immediately before dispatch, never
// stored on the Client or logged.
func (c *Client) ListIssues(ctx context.Context, cred usecase.Credential, repo string, _ usecase.IssueFilter) ([]domain.Issue, error) {
	url := fmt.Sprintf("%s/repos/%s/issues", c.baseURL, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("github: build list issues request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: list issues request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: list issues: unexpected status %d", resp.StatusCode)
	}

	var raw []githubIssue
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("github: decode list issues response: %w", err)
	}

	issues := make([]domain.Issue, 0, len(raw))
	for _, gi := range raw {
		if gi.PullRequest != nil {
			continue // GitHub's issues endpoint also returns PRs; not this method's concern.
		}
		issue, err := domain.NewIssue(strconv.Itoa(gi.Number), domain.ScmProviderGitHub, repo, gi.Title, gi.State, gi.HTMLURL)
		if err != nil {
			return nil, fmt.Errorf("github: invalid issue in response: %w", err)
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

// CreatePullRequest is not implemented in this scaffold — see
// ErrNotImplemented's TODO.
func (c *Client) CreatePullRequest(_ context.Context, _ usecase.Credential, _ string, _ usecase.CreatePullRequestInput) (domain.PullRequest, error) {
	return domain.PullRequest{}, ErrNotImplemented
}

// ListPullRequests is not implemented in this scaffold — see
// ErrNotImplemented's TODO.
func (c *Client) ListPullRequests(_ context.Context, _ usecase.Credential, _ string) ([]domain.PullRequest, error) {
	return nil, ErrNotImplemented
}

// GetRateLimitStatus is not implemented in this scaffold — see
// ErrNotImplemented's TODO. A real implementation calls GET /rate_limit and
// parses the response body's core/graphql/search buckets, per §8.
func (c *Client) GetRateLimitStatus(_ context.Context, _ usecase.Credential) (domain.RateLimitStatus, error) {
	return domain.RateLimitStatus{}, ErrNotImplemented
}
