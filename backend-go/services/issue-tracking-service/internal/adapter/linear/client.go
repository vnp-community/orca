// Package linear implements usecase.IssueTrackerProvider against Linear's
// GraphQL API. Linear publishes no official Go SDK (design doc §4), so this
// hand-rolls a minimal GraphQL client — typed request/response structs over
// net/http, real POST requests with a GraphQL query/mutation string and
// bearer auth. genqlient/gqlgen codegen is a reasonable follow-up per the
// design doc, not required for this to work.
package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)

// endpoint is Linear's single GraphQL API endpoint — there is no
// per-tenant/per-site base URL the way Jira has (design doc §9).
const endpoint = "https://api.linear.app/graphql"

// Client is a real Linear GraphQL client — bearer token auth (personal API
// key or OAuth access token), attached per-request the same way Jira's
// Basic Auth header is (design doc §9: hand-rolling the client doesn't
// change the auth model, @linear/sdk was just a typed wrapper around this
// same bearer scheme).
type Client struct {
	httpClient *http.Client
}

// New returns a Client. Pass nil to use a sane default *http.Client with a
// bounded timeout.
func New(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{httpClient: httpClient}
}

var _ usecase.IssueTrackerProvider = (*Client)(nil)

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type graphQLResponse[T any] struct {
	Data   T              `json:"data"`
	Errors []graphQLError `json:"errors,omitempty"`
}

// do POSTs a GraphQL query/mutation to Linear's endpoint and decodes the
// response into out.
func (c *Client) do(ctx context.Context, token, query string, variables map[string]any, out any) error {
	body, err := json.Marshal(graphQLRequest{Query: query, Variables: variables})
	if err != nil {
		return fmt.Errorf("linear: marshal graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("linear: building graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("linear: graphql request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("linear: unexpected status %d: %s", resp.StatusCode, string(b))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("linear: decoding graphql response: %w", err)
	}
	return nil
}

const listIssuesQuery = `query Issues($filter: IssueFilter) {
  issues(filter: $filter) {
    nodes {
      identifier
      title
      url
      state { name }
    }
  }
}`

type linearIssueNode struct {
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	State      struct {
		Name string `json:"name"`
	} `json:"state"`
}

type linearIssuesData struct {
	Issues struct {
		Nodes []linearIssueNode `json:"nodes"`
	} `json:"issues"`
}

// ListIssues performs a real GraphQL query against Linear's API, filtered
// to the team matching projectKey when set — Linear's "team" is the
// closest concept to Jira's "project" (design doc §4).
func (c *Client) ListIssues(ctx context.Context, cred usecase.Credential, projectKey string) ([]domain.Issue, error) {
	variables := map[string]any{}
	if projectKey != "" {
		variables["filter"] = map[string]any{
			"team": map[string]any{"key": map[string]any{"eq": projectKey}},
		}
	}

	var resp graphQLResponse[linearIssuesData]
	if err := c.do(ctx, cred.Token, listIssuesQuery, variables, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("linear: list issues: %s", resp.Errors[0].Message)
	}

	issues := make([]domain.Issue, 0, len(resp.Data.Issues.Nodes))
	for _, n := range resp.Data.Issues.Nodes {
		issue, err := domain.NewIssue(n.Identifier, n.Title, n.State.Name, n.URL)
		if err != nil {
			continue // skip a malformed entry rather than failing the whole page
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

const teamByKeyQuery = `query TeamByKey($key: String!) {
  teams(filter: { key: { eq: $key } }) {
    nodes { id }
  }
}`

type linearTeamsData struct {
	Teams struct {
		Nodes []struct {
			ID string `json:"id"`
		} `json:"nodes"`
	} `json:"teams"`
}

// resolveTeamID looks up the Linear team UUID for a team key (e.g. "ENG") —
// CreateIssue's mutation needs a team ID, not a key.
func (c *Client) resolveTeamID(ctx context.Context, token, projectKey string) (string, error) {
	var resp graphQLResponse[linearTeamsData]
	if err := c.do(ctx, token, teamByKeyQuery, map[string]any{"key": projectKey}, &resp); err != nil {
		return "", err
	}
	if len(resp.Errors) > 0 {
		return "", fmt.Errorf("linear: resolve team: %s", resp.Errors[0].Message)
	}
	if len(resp.Data.Teams.Nodes) == 0 {
		return "", fmt.Errorf("linear: no team found for key %q", projectKey)
	}
	return resp.Data.Teams.Nodes[0].ID, nil
}

const createIssueMutation = `mutation IssueCreate($input: IssueCreateInput!) {
  issueCreate(input: $input) {
    success
    issue {
      identifier
      title
      url
      state { name }
    }
  }
}`

type linearIssueCreateData struct {
	IssueCreate struct {
		Success bool            `json:"success"`
		Issue   linearIssueNode `json:"issue"`
	} `json:"issueCreate"`
}

// CreateIssue performs a real GraphQL mutation against Linear's API.
// projectKey is resolved to a team ID first (Linear's issueCreate mutation
// takes teamId, not a team key).
func (c *Client) CreateIssue(ctx context.Context, cred usecase.Credential, projectKey, title, description string) (domain.Issue, error) {
	teamID, err := c.resolveTeamID(ctx, cred.Token, projectKey)
	if err != nil {
		return domain.Issue{}, err
	}

	variables := map[string]any{
		"input": map[string]any{
			"teamId":      teamID,
			"title":       title,
			"description": description,
		},
	}
	var resp graphQLResponse[linearIssueCreateData]
	if err := c.do(ctx, cred.Token, createIssueMutation, variables, &resp); err != nil {
		return domain.Issue{}, err
	}
	if len(resp.Errors) > 0 {
		return domain.Issue{}, fmt.Errorf("linear: create issue: %s", resp.Errors[0].Message)
	}
	if !resp.Data.IssueCreate.Success {
		return domain.Issue{}, fmt.Errorf("linear: issue creation reported failure")
	}

	n := resp.Data.IssueCreate.Issue
	return domain.NewIssue(n.Identifier, n.Title, n.State.Name, n.URL)
}
