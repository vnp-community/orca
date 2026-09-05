package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

var _ usecase.WorkItemProvider = (*Client)(nil)

// githubWorkItemIssue/githubWorkItemPR are separate, richer wire shapes
// from githubIssue/githubPullRequest above — kept independent (not
// extending those structs) so ListIssues/ListPullRequests/
// CreatePullRequest's existing decode paths are untouched by this feature.
type githubWorkItemIssue struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	State       string    `json:"state"`
	HTMLURL     string    `json:"html_url"`
	UpdatedAt   time.Time `json:"updated_at"`
	User        *ghUser   `json:"user"`
	Labels      []ghLabel `json:"labels"`
	PullRequest *struct{} `json:"pull_request"`
}

type githubWorkItemPR struct {
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	State     string     `json:"state"`
	Draft     bool       `json:"draft"`
	HTMLURL   string     `json:"html_url"`
	UpdatedAt time.Time  `json:"updated_at"`
	MergedAt  *time.Time `json:"merged_at"`
	User      *ghUser    `json:"user"`
	Labels    []ghLabel  `json:"labels"`
}

type ghUser struct {
	Login string `json:"login"`
}

type ghLabel struct {
	Name string `json:"name"`
}

// ListWorkItems implements usecase.WorkItemProvider for GitHub — fetches
// open/closed issues and pull requests (per filter.Scope) via GitHub's REST
// API and maps both into the shared domain.WorkItem shape. Ports the
// legacy desktop backend's listWorkItems (backend/src/main/github/client.ts)
// "recent" mode only — see WorkItemFilter's doc comment for what's
// deliberately not ported (queried-mode's full search-syntax, pagination,
// caching).
//
// Partial-failure semantics mirror the legacy backend's asymmetric
// handling (client.ts's documented `// Why:` block): an issues-side
// failure is swallowed and PRs still return; a PR-side failure fails the
// whole call. This keeps the common case (a repo with issues disabled,
// which GitHub reports as a 410 on the issues endpoint) from blanking the
// PR list too.
func (c *Client) ListWorkItems(ctx context.Context, cred usecase.Credential, repo string, filter usecase.WorkItemFilter) ([]domain.WorkItem, error) {
	var items []domain.WorkItem

	if filter.Scope != "pr" {
		issues, err := c.listWorkItemIssues(ctx, cred, repo, filter)
		if err == nil {
			items = append(items, issues...)
		}
		// Issues-side failure is intentionally swallowed here (see doc
		// comment) — e.g. issues disabled on this repo shouldn't blank PRs.
	}

	if filter.Scope != "issue" {
		prs, err := c.listWorkItemPullRequests(ctx, cred, repo, filter)
		if err != nil {
			return nil, fmt.Errorf("github: list work items (pull requests): %w", err)
		}
		items = append(items, prs...)
	}

	return items, nil
}

func (c *Client) listWorkItemIssues(ctx context.Context, cred usecase.Credential, repo string, filter usecase.WorkItemFilter) ([]domain.WorkItem, error) {
	q := url.Values{}
	q.Set("state", issuesAPIState(filter.State))
	q.Set("sort", "updated")
	q.Set("direction", "desc")
	q.Set("per_page", strconv.Itoa(clampPerPage(filter.Limit)))
	if len(filter.Labels) > 0 {
		q.Set("labels", strings.Join(filter.Labels, ","))
	}
	if filter.Assignee != "" {
		q.Set("assignee", filter.Assignee)
	}
	if filter.Author != "" {
		q.Set("creator", filter.Author)
	}

	reqURL := fmt.Sprintf("%s/repos/%s/issues?%s", c.baseURL, repo, q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("github: build list work items (issues) request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: list work items (issues) request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: list work items (issues): unexpected status %d", resp.StatusCode)
	}

	var raw []githubWorkItemIssue
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("github: decode list work items (issues) response: %w", err)
	}

	items := make([]domain.WorkItem, 0, len(raw))
	for _, gi := range raw {
		if gi.PullRequest != nil {
			continue // GitHub's issues endpoint also returns PRs; listWorkItemPullRequests covers those.
		}
		item, err := domain.NewWorkItem(
			"issue:"+strconv.Itoa(gi.Number), "issue", int32(gi.Number), gi.Title, gi.State, gi.HTMLURL,
			labelNames(gi.Labels), gi.UpdatedAt, userLogin(gi.User),
		)
		if err != nil {
			continue // skip a malformed row rather than failing the whole list
		}
		items = append(items, item)
	}
	return items, nil
}

func (c *Client) listWorkItemPullRequests(ctx context.Context, cred usecase.Credential, repo string, filter usecase.WorkItemFilter) ([]domain.WorkItem, error) {
	// GitHub's pulls endpoint only accepts state=open|closed|all (no
	// "merged" literal) — state:merged is applied as a client-side filter
	// below on merged_at instead. It also has no assignee/creator/labels
	// query params (unlike the issues endpoint) — Assignee/Author/Labels
	// are likewise applied client-side here.
	apiState := filter.State
	if apiState == "merged" || apiState == "" {
		apiState = "all"
	}
	q := url.Values{}
	q.Set("state", apiState)
	q.Set("sort", "updated")
	q.Set("direction", "desc")
	q.Set("per_page", strconv.Itoa(clampPerPage(filter.Limit)))

	reqURL := fmt.Sprintf("%s/repos/%s/pulls?%s", c.baseURL, repo, q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build list work items (pull requests) request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list work items (pull requests) request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list work items (pull requests): unexpected status %d", resp.StatusCode)
	}

	var raw []githubWorkItemPR
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode list work items (pull requests) response: %w", err)
	}

	items := make([]domain.WorkItem, 0, len(raw))
	for _, gp := range raw {
		merged := gp.MergedAt != nil
		if filter.State == "merged" && !merged {
			continue
		}
		if filter.Assignee != "" && filter.Assignee != userLogin(gp.User) {
			continue
		}
		if filter.Author != "" && filter.Author != userLogin(gp.User) {
			continue
		}
		if len(filter.Labels) > 0 && !hasAnyLabel(gp.Labels, filter.Labels) {
			continue
		}
		state := prWorkItemState(gp.State, gp.Draft, merged)
		item, err := domain.NewWorkItem(
			"pr:"+strconv.Itoa(gp.Number), "pr", int32(gp.Number), gp.Title, state, gp.HTMLURL,
			labelNames(gp.Labels), gp.UpdatedAt, userLogin(gp.User),
		)
		if err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

// prWorkItemState mirrors the legacy backend's mapPullRequestWorkItem
// precedence exactly: merged > closed > draft > open.
func prWorkItemState(rawState string, draft, merged bool) string {
	switch {
	case merged:
		return "merged"
	case rawState == "closed":
		return "closed"
	case draft:
		return "draft"
	default:
		return "open"
	}
}

// issuesAPIState maps this feature's filter vocabulary ("merged" is
// PR-only and never reaches here) onto GitHub's issues-endpoint state
// values; empty/unrecognized defaults to "open", the legacy backend's own
// default for its "recent" mode.
func issuesAPIState(state string) string {
	switch state {
	case "closed", "all":
		return state
	default:
		return "open"
	}
}

func clampPerPage(limit int) int {
	if limit <= 0 {
		return defaultWorkItemLimitFallback
	}
	if limit > 100 { // GitHub's own REST per_page cap
		return 100
	}
	return limit
}

// defaultWorkItemLimitFallback mirrors usecase.defaultWorkItemLimit — kept
// as a separate constant since this package can't import the usecase
// package's unexported constant (and shouldn't depend on usecase beyond
// the WorkItemProvider/WorkItemFilter/Credential types it already does).
const defaultWorkItemLimitFallback = 24

func userLogin(u *ghUser) string {
	if u == nil {
		return ""
	}
	return u.Login
}

func labelNames(labels []ghLabel) []string {
	if len(labels) == 0 {
		return nil
	}
	names := make([]string, 0, len(labels))
	for _, l := range labels {
		names = append(names, l.Name)
	}
	return names
}

func hasAnyLabel(labels []ghLabel, want []string) bool {
	for _, l := range labels {
		for _, w := range want {
			if l.Name == w {
				return true
			}
		}
	}
	return false
}
