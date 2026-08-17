// Package bitbucket implements usecase.ScmProvider against Bitbucket Cloud's
// REST API v2 (https://developer.atlassian.com/cloud/bitbucket/rest/)
// directly via net/http — no shared keychain, no shell-out, per
// scm-integration-service.md §10. Every ScmProvider method is a real REST
// call, structurally mirroring internal/adapter/github's reference
// implementation.
package bitbucket

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

// DefaultBaseURL is Bitbucket Cloud's public REST API v2 root.
const DefaultBaseURL = "https://api.bitbucket.org/2.0"

// ErrNotImplemented is kept as a sentinel for any future method added to
// usecase.ScmProvider before this adapter implements it — every method
// currently in the interface is real.
var ErrNotImplemented = errors.New("bitbucket: not implemented")

// Client implements usecase.ScmProvider against Bitbucket Cloud's REST API.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// New returns a Bitbucket Client. A nil httpClient defaults to
// http.DefaultClient; an empty baseURL defaults to DefaultBaseURL —
// overridable for tests (httptest.Server).
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

// bitbucketLinks mirrors the "links" object Bitbucket embeds on issues and
// pull requests, from which this adapter only needs the HTML link.
type bitbucketLinks struct {
	HTML struct {
		Href string `json:"href"`
	} `json:"html"`
}

// bitbucketIssue mirrors the fields this adapter needs from Bitbucket's
// GET /repositories/{workspace}/{repo_slug}/issues response
// (https://developer.atlassian.com/cloud/bitbucket/rest/api-group-issue-tracker/).
type bitbucketIssue struct {
	ID    int            `json:"id"`
	Title string         `json:"title"`
	State string         `json:"state"`
	Links bitbucketLinks `json:"links"`
}

// bitbucketIssuesResponse is Bitbucket's list-endpoint pagination envelope —
// only "values" is consumed here; "next" is left unhandled (single page
// only), see ListIssues' doc comment.
type bitbucketIssuesResponse struct {
	Values []bitbucketIssue `json:"values"`
}

// ListIssues calls Bitbucket's REST API for real: GET
// /repositories/{repo}/issues, with the resolved per-tenant token as a
// Bearer credential — per scm-integration-service.md §9, this is the only
// place that token exists, built directly into the auth header immediately
// before dispatch, never stored on the Client or logged.
//
// filter is accepted to satisfy usecase.ScmProvider but unused: Bitbucket's
// issue-state filtering uses a query language (q=state="open") that's out of
// scope for this scaffold.
//
// Pagination isn't handled yet — only the first page ("values") is read.
func (c *Client) ListIssues(ctx context.Context, cred usecase.Credential, repo string, _ usecase.IssueFilter) ([]domain.Issue, error) {
	url := fmt.Sprintf("%s/repositories/%s/issues", c.baseURL, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: build list issues request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: list issues request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bitbucket: list issues: unexpected status %d", resp.StatusCode)
	}

	var raw bitbucketIssuesResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("bitbucket: decode list issues response: %w", err)
	}

	issues := make([]domain.Issue, 0, len(raw.Values))
	for _, bi := range raw.Values {
		issue, err := domain.NewIssue(strconv.Itoa(bi.ID), domain.ScmProviderBitbucket, repo, bi.Title, bi.State, bi.Links.HTML.Href)
		if err != nil {
			return nil, fmt.Errorf("bitbucket: invalid issue in response: %w", err)
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

// bitbucketBranchName mirrors the innermost {"name": ...} object nested
// under a Bitbucket pull request's "source"/"destination" branch ref.
type bitbucketBranchName struct {
	Name string `json:"name"`
}

// bitbucketBranchRef mirrors the {"branch": {"name": ...}} shape nested
// under "source"/"destination" on a Bitbucket pull request — unlike GitHub's
// flat head/base strings, Bitbucket nests the branch name one level deeper.
type bitbucketBranchRef struct {
	Branch bitbucketBranchName `json:"branch"`
}

// bitbucketPullRequest mirrors the fields this adapter needs from
// Bitbucket's POST/GET /repositories/{workspace}/{repo_slug}/pullrequests
// response shape
// (https://developer.atlassian.com/cloud/bitbucket/rest/api-group-pullrequests/).
type bitbucketPullRequest struct {
	ID          int                `json:"id"`
	Title       string             `json:"title"`
	State       string             `json:"state"`
	Links       bitbucketLinks     `json:"links"`
	Source      bitbucketBranchRef `json:"source"`
	Destination bitbucketBranchRef `json:"destination"`
}

// bitbucketPullRequestsResponse is Bitbucket's list-endpoint pagination
// envelope for pull requests — see bitbucketIssuesResponse.
type bitbucketPullRequestsResponse struct {
	Values []bitbucketPullRequest `json:"values"`
}

func toDomainPullRequest(repo string, bp bitbucketPullRequest) (domain.PullRequest, error) {
	return domain.NewPullRequest(
		strconv.Itoa(bp.ID), domain.ScmProviderBitbucket, repo, bp.Title, bp.State, bp.Links.HTML.Href,
		bp.Source.Branch.Name, bp.Destination.Branch.Name,
	)
}

// CreatePullRequest calls Bitbucket's REST API for real: POST
// /repositories/{repo}/pullrequests, with the resolved per-tenant token as a
// Bearer credential — see ListIssues' doc comment for the token-handling
// invariant this shares.
func (c *Client) CreatePullRequest(ctx context.Context, cred usecase.Credential, repo string, input usecase.CreatePullRequestInput) (domain.PullRequest, error) {
	body, err := json.Marshal(struct {
		Title       string             `json:"title"`
		Description string             `json:"description,omitempty"`
		Source      bitbucketBranchRef `json:"source"`
		Destination bitbucketBranchRef `json:"destination"`
	}{
		Title:       input.Title,
		Description: input.Body,
		Source:      bitbucketBranchRef{Branch: bitbucketBranchName{Name: input.HeadBranch}},
		Destination: bitbucketBranchRef{Branch: bitbucketBranchName{Name: input.BaseBranch}},
	})
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("bitbucket: encode create pull request body: %w", err)
	}

	url := fmt.Sprintf("%s/repositories/%s/pullrequests", c.baseURL, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("bitbucket: build create pull request request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("bitbucket: create pull request request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return domain.PullRequest{}, fmt.Errorf("bitbucket: create pull request: unexpected status %d", resp.StatusCode)
	}

	var raw bitbucketPullRequest
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return domain.PullRequest{}, fmt.Errorf("bitbucket: decode create pull request response: %w", err)
	}
	pr, err := toDomainPullRequest(repo, raw)
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("bitbucket: invalid pull request in response: %w", err)
	}
	return pr, nil
}

// ListPullRequests calls Bitbucket's REST API for real: GET
// /repositories/{repo}/pullrequests.
func (c *Client) ListPullRequests(ctx context.Context, cred usecase.Credential, repo string) ([]domain.PullRequest, error) {
	url := fmt.Sprintf("%s/repositories/%s/pullrequests", c.baseURL, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: build list pull requests request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: list pull requests request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bitbucket: list pull requests: unexpected status %d", resp.StatusCode)
	}

	var raw bitbucketPullRequestsResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("bitbucket: decode list pull requests response: %w", err)
	}
	prs := make([]domain.PullRequest, 0, len(raw.Values))
	for _, bp := range raw.Values {
		pr, err := toDomainPullRequest(repo, bp)
		if err != nil {
			return nil, fmt.Errorf("bitbucket: invalid pull request in response: %w", err)
		}
		prs = append(prs, pr)
	}
	return prs, nil
}

// GetRateLimitStatus calls Bitbucket's REST API for real: GET /user, a
// lightweight authenticated endpoint — Bitbucket Cloud has no dedicated
// rate-limit-status endpoint comparable to GitHub's /rate_limit. Header
// exposure of X-RateLimit-* is inconsistent across Bitbucket plans, so a
// missing header is treated as "unknown" (zero values), not a failure.
func (c *Client) GetRateLimitStatus(ctx context.Context, cred usecase.Credential) (domain.RateLimitStatus, error) {
	url := fmt.Sprintf("%s/user", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return domain.RateLimitStatus{}, fmt.Errorf("bitbucket: build rate limit request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.RateLimitStatus{}, fmt.Errorf("bitbucket: rate limit request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return domain.RateLimitStatus{}, fmt.Errorf("bitbucket: rate limit: unexpected status %d", resp.StatusCode)
	}

	status := domain.RateLimitStatus{Provider: domain.ScmProviderBitbucket}
	if limit, err := strconv.Atoi(resp.Header.Get("X-RateLimit-Limit")); err == nil {
		status.Limit = limit
	}
	if remaining, err := strconv.Atoi(resp.Header.Get("X-RateLimit-Remaining")); err == nil {
		status.Remaining = remaining
	}
	if reset, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		status.ResetAt = time.Unix(reset, 0)
	}
	return status, nil
}
