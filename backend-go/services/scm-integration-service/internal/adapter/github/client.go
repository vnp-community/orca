// Package github implements usecase.ScmProvider against GitHub's REST API
// directly via net/http — no gh CLI, no shared keychain, no shell-out, per
// scm-integration-service.md §10 (the direct fix for TS Gap 1). Every
// ScmProvider method is a real REST call as of Phase 3
// (docs/execution-plan.md §3) — this is the reference implementation the
// other four provider adapters mirror.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// DefaultBaseURL is GitHub's public REST API root.
const DefaultBaseURL = "https://api.github.com"

// ErrNotImplemented is kept as a sentinel for any future method added to
// usecase.ScmProvider before this adapter implements it — every method
// currently in the interface is real.
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

// githubPullRequest mirrors the fields this adapter needs from GitHub's
// POST/GET /repos/{owner}/{repo}/pulls response shape
// (https://docs.github.com/en/rest/pulls/pulls).
type githubPullRequest struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	State   string `json:"state"`
	HTMLURL string `json:"html_url"`
	Head    struct {
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

func toDomainPullRequest(repo string, gp githubPullRequest) (domain.PullRequest, error) {
	return domain.NewPullRequest(
		strconv.Itoa(gp.Number), domain.ScmProviderGitHub, repo, gp.Title, gp.State, gp.HTMLURL,
		gp.Head.Ref, gp.Base.Ref,
	)
}

// CreatePullRequest calls GitHub's REST API for real: POST
// /repos/{repo}/pulls, with the resolved per-tenant token as a Bearer
// credential — see ListIssues' doc comment for the token-handling
// invariant this shares.
func (c *Client) CreatePullRequest(ctx context.Context, cred usecase.Credential, repo string, input usecase.CreatePullRequestInput) (domain.PullRequest, error) {
	body, err := json.Marshal(struct {
		Title string `json:"title"`
		Body  string `json:"body,omitempty"`
		Head  string `json:"head"`
		Base  string `json:"base"`
	}{Title: input.Title, Body: input.Body, Head: input.HeadBranch, Base: input.BaseBranch})
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("github: encode create pull request body: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/pulls", c.baseURL, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("github: build create pull request request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("github: create pull request request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return domain.PullRequest{}, fmt.Errorf("github: create pull request: unexpected status %d", resp.StatusCode)
	}

	var raw githubPullRequest
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return domain.PullRequest{}, fmt.Errorf("github: decode create pull request response: %w", err)
	}
	pr, err := toDomainPullRequest(repo, raw)
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("github: invalid pull request in response: %w", err)
	}
	return pr, nil
}

// ListPullRequests calls GitHub's REST API for real: GET
// /repos/{repo}/pulls.
func (c *Client) ListPullRequests(ctx context.Context, cred usecase.Credential, repo string) ([]domain.PullRequest, error) {
	url := fmt.Sprintf("%s/repos/%s/pulls", c.baseURL, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("github: build list pull requests request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: list pull requests request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: list pull requests: unexpected status %d", resp.StatusCode)
	}

	var raw []githubPullRequest
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("github: decode list pull requests response: %w", err)
	}
	prs := make([]domain.PullRequest, 0, len(raw))
	for _, gp := range raw {
		pr, err := toDomainPullRequest(repo, gp)
		if err != nil {
			return nil, fmt.Errorf("github: invalid pull request in response: %w", err)
		}
		prs = append(prs, pr)
	}
	return prs, nil
}

// githubRateLimitResponse mirrors GET /rate_limit
// (https://docs.github.com/en/rest/rate-limit) — this adapter reports the
// "core" REST bucket, since that's the bucket every other method in this
// client consumes against; GraphQL/search buckets are a separate future
// RateLimitStatus.Bucket dimension per §8, not modeled yet.
type githubRateLimitResponse struct {
	Resources struct {
		Core struct {
			Limit     int   `json:"limit"`
			Remaining int   `json:"remaining"`
			Reset     int64 `json:"reset"`
		} `json:"core"`
	} `json:"resources"`
}

// GetRateLimitStatus calls GitHub's REST API for real: GET /rate_limit.
func (c *Client) GetRateLimitStatus(ctx context.Context, cred usecase.Credential) (domain.RateLimitStatus, error) {
	url := fmt.Sprintf("%s/rate_limit", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return domain.RateLimitStatus{}, fmt.Errorf("github: build rate limit request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.RateLimitStatus{}, fmt.Errorf("github: rate limit request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return domain.RateLimitStatus{}, fmt.Errorf("github: rate limit: unexpected status %d", resp.StatusCode)
	}

	var raw githubRateLimitResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return domain.RateLimitStatus{}, fmt.Errorf("github: decode rate limit response: %w", err)
	}
	return domain.RateLimitStatus{
		Provider:  domain.ScmProviderGitHub,
		Remaining: raw.Resources.Core.Remaining,
		Limit:     raw.Resources.Core.Limit,
		ResetAt:   time.Unix(raw.Resources.Core.Reset, 0),
	}, nil
}
