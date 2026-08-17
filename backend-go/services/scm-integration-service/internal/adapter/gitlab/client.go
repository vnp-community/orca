// Package gitlab implements usecase.ScmProvider against GitLab's REST API v4
// (https://gitlab.com/api/v4) directly via net/http — no glab CLI, no shared
// keychain, no shell-out, per scm-integration-service.md §10. Every
// ScmProvider method is a real REST call, structurally mirroring
// internal/adapter/github (the reference implementation the other provider
// adapters mirror): the resolved per-tenant token is built directly into the
// auth header immediately before dispatch, never stored on the Client or
// logged.
package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// DefaultBaseURL is GitLab.com's public REST API v4 root.
const DefaultBaseURL = "https://gitlab.com/api/v4"

// ErrNotImplemented is kept as a sentinel for any future method added to
// usecase.ScmProvider before this adapter implements it — every method
// currently in the interface is real.
var ErrNotImplemented = errors.New("gitlab: not implemented")

// Client implements usecase.ScmProvider against GitLab's REST API.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// New returns a GitLab Client. A nil httpClient defaults to
// http.DefaultClient; an empty baseURL defaults to DefaultBaseURL —
// overridable for tests (httptest.Server) and self-managed GitLab
// deployments.
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

// projectPath builds the {id} path segment GitLab's v4 API expects — a
// numeric project id or a URL-encoded "namespace/project" path. repo is not
// assumed to be pre-escaped, so it's escaped here.
func projectPath(repo string) string {
	return url.PathEscape(repo)
}

// gitlabIssue mirrors the fields this adapter needs from GitLab's
// GET /projects/{id}/issues response
// (https://docs.gitlab.com/ee/api/issues.html#list-project-issues). Unlike
// GitHub, GitLab's issues endpoint never returns merge requests, so there is
// no PullRequest-shaped field to filter out here.
type gitlabIssue struct {
	IID   int    `json:"iid"`
	Title string `json:"title"`
	State string `json:"state"`
	URL   string `json:"web_url"`
}

// ListIssues calls GitLab's REST API for real: GET
// /projects/{id}/issues, with the resolved per-tenant token as a Bearer
// credential — per scm-integration-service.md §9, this is the only place
// that token exists, built directly into the auth header immediately before
// dispatch, never stored on the Client or logged.
func (c *Client) ListIssues(ctx context.Context, cred usecase.Credential, repo string, filter usecase.IssueFilter) ([]domain.Issue, error) {
	reqURL := fmt.Sprintf("%s/projects/%s/issues", c.baseURL, projectPath(repo))
	if filter.State != "" {
		reqURL += "?state=" + url.QueryEscape(filter.State)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab: build list issues request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: list issues request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitlab: list issues: unexpected status %d", resp.StatusCode)
	}

	var raw []gitlabIssue
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("gitlab: decode list issues response: %w", err)
	}

	issues := make([]domain.Issue, 0, len(raw))
	for _, gi := range raw {
		issue, err := domain.NewIssue(strconv.Itoa(gi.IID), domain.ScmProviderGitLab, repo, gi.Title, gi.State, gi.URL)
		if err != nil {
			return nil, fmt.Errorf("gitlab: invalid issue in response: %w", err)
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

// gitlabMergeRequest mirrors the fields this adapter needs from GitLab's
// POST/GET /projects/{id}/merge_requests response shape
// (https://docs.gitlab.com/ee/api/merge_requests.html) — GitLab's name for
// what GitHub calls a pull request.
type gitlabMergeRequest struct {
	IID          int    `json:"iid"`
	Title        string `json:"title"`
	State        string `json:"state"`
	URL          string `json:"web_url"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
}

func toDomainPullRequest(repo string, gm gitlabMergeRequest) (domain.PullRequest, error) {
	return domain.NewPullRequest(
		strconv.Itoa(gm.IID), domain.ScmProviderGitLab, repo, gm.Title, gm.State, gm.URL,
		gm.SourceBranch, gm.TargetBranch,
	)
}

// CreatePullRequest calls GitLab's REST API for real: POST
// /projects/{id}/merge_requests, with the resolved per-tenant token as a
// Bearer credential — see ListIssues' doc comment for the token-handling
// invariant this shares.
func (c *Client) CreatePullRequest(ctx context.Context, cred usecase.Credential, repo string, input usecase.CreatePullRequestInput) (domain.PullRequest, error) {
	body, err := json.Marshal(struct {
		Title        string `json:"title"`
		Description  string `json:"description,omitempty"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
	}{Title: input.Title, Description: input.Body, SourceBranch: input.HeadBranch, TargetBranch: input.BaseBranch})
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("gitlab: encode create pull request body: %w", err)
	}

	reqURL := fmt.Sprintf("%s/projects/%s/merge_requests", c.baseURL, projectPath(repo))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("gitlab: build create pull request request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("gitlab: create pull request request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return domain.PullRequest{}, fmt.Errorf("gitlab: create pull request: unexpected status %d", resp.StatusCode)
	}

	var raw gitlabMergeRequest
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return domain.PullRequest{}, fmt.Errorf("gitlab: decode create pull request response: %w", err)
	}
	pr, err := toDomainPullRequest(repo, raw)
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("gitlab: invalid pull request in response: %w", err)
	}
	return pr, nil
}

// ListPullRequests calls GitLab's REST API for real: GET
// /projects/{id}/merge_requests.
func (c *Client) ListPullRequests(ctx context.Context, cred usecase.Credential, repo string) ([]domain.PullRequest, error) {
	reqURL := fmt.Sprintf("%s/projects/%s/merge_requests", c.baseURL, projectPath(repo))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab: build list pull requests request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: list pull requests request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitlab: list pull requests: unexpected status %d", resp.StatusCode)
	}

	var raw []gitlabMergeRequest
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("gitlab: decode list pull requests response: %w", err)
	}
	prs := make([]domain.PullRequest, 0, len(raw))
	for _, gm := range raw {
		pr, err := toDomainPullRequest(repo, gm)
		if err != nil {
			return nil, fmt.Errorf("gitlab: invalid pull request in response: %w", err)
		}
		prs = append(prs, pr)
	}
	return prs, nil
}

// GetRateLimitStatus calls GitLab's REST API for real: GET /user — GitLab
// has no dedicated rate-limit-status endpoint (§8: "one bucket per token");
// it reports remaining quota via RateLimit-* response headers on every API
// call instead, so this uses the cheapest always-available authenticated
// endpoint purely to read those headers.
func (c *Client) GetRateLimitStatus(ctx context.Context, cred usecase.Credential) (domain.RateLimitStatus, error) {
	reqURL := fmt.Sprintf("%s/user", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return domain.RateLimitStatus{}, fmt.Errorf("gitlab: build rate limit request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.RateLimitStatus{}, fmt.Errorf("gitlab: rate limit request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return domain.RateLimitStatus{}, fmt.Errorf("gitlab: rate limit: unexpected status %d", resp.StatusCode)
	}

	// A missing header is not a request failure — some GitLab plans/deployments
	// don't send RateLimit-*, so absence just means "no signal", not an error.
	limit, _ := strconv.Atoi(resp.Header.Get("RateLimit-Limit"))
	remaining, _ := strconv.Atoi(resp.Header.Get("RateLimit-Remaining"))
	var resetAt time.Time
	if reset, err := strconv.ParseInt(resp.Header.Get("RateLimit-Reset"), 10, 64); err == nil {
		resetAt = time.Unix(reset, 0)
	}

	return domain.RateLimitStatus{
		Provider:  domain.ScmProviderGitLab,
		Remaining: remaining,
		Limit:     limit,
		ResetAt:   resetAt,
	}, nil
}
