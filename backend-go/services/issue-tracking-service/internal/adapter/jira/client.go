// Package jira implements usecase.IssueTrackerProvider against Jira Cloud's
// REST API v3 — a plain HTTP client, no SDK gap to work around (design doc
// §4: "Jira's adapter has no equivalent gap [to Linear]: a plain REST
// client either way").
package jira

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)

// defaultIssueType is the issue-type name CreateIssue prefers when the
// caller doesn't specify one — the RPC surface (issuetracking.proto) has no
// issue-type field yet, so there is no caller-requested name to read. This
// is no longer sent blindly: CreateIssue resolves it against the project's
// real issue types first (via listIssueTypes) and only uses this name if a
// case-insensitive match for it actually exists on the target project;
// otherwise it falls back to the first non-subtask type. Jira sites vary in
// which type names exist, which is exactly what this lookup guards against.
// TODO: thread a caller-requested issue-type name through CreateIssueRequest
// once the design doc's ListIssueTypes RPC (§3) is exposed over gRPC.
const defaultIssueType = "Task"

// Client is a real Jira Cloud REST API v3 client — Basic Auth,
// base64(email:apiToken), unchanged from the TS jira/client.ts scheme
// (design doc §9).
type Client struct {
	httpClient *http.Client
}

// New returns a Client. Pass nil to use a sane default *http.Client with a
// bounded timeout — every outbound call also carries the inbound gRPC
// context's deadline (design doc §8), the client timeout is just a backstop.
func New(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{httpClient: httpClient}
}

var _ usecase.IssueTrackerProvider = (*Client)(nil)

// jiraSearchResponse mirrors the subset of Jira Cloud's
// GET /rest/api/3/search JSON response shape this adapter needs.
type jiraSearchResponse struct {
	Issues []jiraIssue `json:"issues"`
}

type jiraIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Status  struct {
			Name string `json:"name"`
		} `json:"status"`
	} `json:"fields"`
}

// ListIssues performs a real GET against Jira's /rest/api/3/search, JQL
// filtered to projectKey when set.
func (c *Client) ListIssues(ctx context.Context, cred usecase.Credential, projectKey string) ([]domain.Issue, error) {
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}

	u, err := url.Parse(strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/search")
	if err != nil {
		return nil, fmt.Errorf("jira: invalid base url: %w", err)
	}
	q := u.Query()
	if projectKey != "" {
		q.Set("jql", fmt.Sprintf("project=%q", projectKey))
	}
	q.Set("maxResults", "50")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list issues request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: list issues request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, jiraStatusError("list issues", resp)
	}

	var parsed jiraSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("jira: decoding list issues response: %w", err)
	}

	issues := make([]domain.Issue, 0, len(parsed.Issues))
	for _, ji := range parsed.Issues {
		issue, err := domain.NewIssue(ji.Key, ji.Fields.Summary, ji.Fields.Status.Name, issueBrowseURL(cred.BaseURL, ji.Key))
		if err != nil {
			continue // skip a malformed entry rather than failing the whole page
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

// jiraCreateIssueRequest mirrors POST /rest/api/3/issue's request body.
type jiraCreateIssueRequest struct {
	Fields jiraCreateIssueFields `json:"fields"`
}

type jiraCreateIssueFields struct {
	Project     jiraProjectRef   `json:"project"`
	Summary     string           `json:"summary"`
	Description *adfDoc          `json:"description,omitempty"`
	IssueType   jiraIssueTypeRef `json:"issuetype"`
}

type jiraProjectRef struct {
	Key string `json:"key"`
}

type jiraIssueTypeRef struct {
	Name string `json:"name"`
}

type jiraCreateIssueResponse struct {
	Key string `json:"key"`
}

// CreateIssue performs a real POST against Jira's /rest/api/3/issue.
// Description is wrapped in a minimal Atlassian Document Format (ADF)
// document — Jira Cloud REST API v3 requires ADF, not plain text, for rich
// text fields. The issue type sent is resolved against the project's real
// issue types (listIssueTypes) rather than blindly hardcoded — see
// resolveIssueType and defaultIssueType's doc comment.
func (c *Client) CreateIssue(ctx context.Context, cred usecase.Credential, projectKey, title, description string) (domain.Issue, error) {
	if cred.BaseURL == "" {
		return domain.Issue{}, fmt.Errorf("jira: credential is missing a site base URL")
	}

	types, err := c.listIssueTypes(ctx, cred, projectKey)
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: resolving issue type: %w", err)
	}
	issueTypeName, err := resolveIssueType(types, defaultIssueType)
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: resolving issue type: %w", err)
	}

	fields := jiraCreateIssueFields{
		Project:   jiraProjectRef{Key: projectKey},
		Summary:   title,
		IssueType: jiraIssueTypeRef{Name: issueTypeName},
	}
	if description != "" {
		doc := plainTextADF(description)
		fields.Description = &doc
	}
	body, err := json.Marshal(jiraCreateIssueRequest{Fields: fields})
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: marshal create issue request: %w", err)
	}

	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/issue"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: building create issue request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: create issue request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return domain.Issue{}, jiraStatusError("create issue", resp)
	}

	var parsed jiraCreateIssueResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return domain.Issue{}, fmt.Errorf("jira: decoding create issue response: %w", err)
	}

	return domain.NewIssue(parsed.Key, title, "", issueBrowseURL(cred.BaseURL, parsed.Key))
}

// jiraIssueTypeMeta mirrors one entry of Jira Cloud's
// GET /rest/api/3/issue/createmeta/{projectIdOrKey}/issuetypes JSON
// response — the current (non-deprecated) endpoint for discovering which
// issue types are actually creatable on a project. The older
// GET /rest/api/3/issue/createmeta?expand=projects.issuetypes sketch from
// the design doc's §3 API sketch is deprecated by Atlassian in favor of
// this per-project endpoint.
type jiraIssueTypeMeta struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

// jiraIssueTypesResponse mirrors the paginated envelope
// GET .../issuetypes returns. This adapter doesn't page through it — Jira
// Cloud projects have a small, bounded number of issue types, well under a
// single page's maxResults.
type jiraIssueTypesResponse struct {
	Values []jiraIssueTypeMeta `json:"values"`
}

// listIssueTypes performs a real GET against Jira's
// /rest/api/3/issue/createmeta/{projectIdOrKey}/issuetypes, returning the
// issue types Jira actually allows creating on projectKey. This is an
// internal capability CreateIssue calls before every create — the design
// doc's ListIssueTypes RPC (§3) isn't exposed over gRPC (issue-tracking-service
// only implements the RPCs the generated proto defines today), so this
// stays unexported rather than becoming a new public method. Not cached:
// this service resolves credentials fresh per request with no
// per-connection session state (design doc §8), and a plain per-call HTTP
// request matches that same no-session-state convention — there is no
// existing caching layer in this adapter to reuse.
func (c *Client) listIssueTypes(ctx context.Context, cred usecase.Credential, projectKey string) ([]jiraIssueTypeMeta, error) {
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}

	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/issue/createmeta/" + url.PathEscape(projectKey) + "/issuetypes"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list issue types request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: list issue types request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, jiraStatusError("list issue types", resp)
	}

	var parsed jiraIssueTypesResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("jira: decoding list issue types response: %w", err)
	}
	return parsed.Values, nil
}

// resolveIssueType picks which real Jira issue-type name to send on
// CreateIssue, given the project's actual issue types and a preferred name
// (defaultIssueType, "Task", until CreateIssueRequest can carry a
// caller-requested one). Preference order: an exact case-insensitive match
// on preferredName; else the first non-subtask type (subtasks require a
// parent issue and can't be the target of a bare top-level CreateIssue);
// else an error — a project with no usable issue type can't have an issue
// created on it, and silently guessing would just trade one hardcoded
// string for another.
func resolveIssueType(types []jiraIssueTypeMeta, preferredName string) (string, error) {
	if len(types) == 0 {
		return "", fmt.Errorf("jira: project has no issue types available")
	}
	for _, t := range types {
		if strings.EqualFold(t.Name, preferredName) {
			return t.Name, nil
		}
	}
	for _, t := range types {
		if !t.Subtask {
			return t.Name, nil
		}
	}
	return "", fmt.Errorf("jira: project has no non-subtask issue type available")
}

func jiraStatusError(op string, resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("jira: %s: unexpected status %d: %s", op, resp.StatusCode, string(b))
}

func issueBrowseURL(baseURL, key string) string {
	if key == "" {
		return ""
	}
	return strings.TrimRight(baseURL, "/") + "/browse/" + key
}

func basicAuth(email, token string) string {
	return base64.StdEncoding.EncodeToString([]byte(email + ":" + token))
}

// adfDoc is a minimal Atlassian Document Format document — just enough to
// carry a plain-text description, not the full ADF node-type surface.
type adfDoc struct {
	Type    string    `json:"type"`
	Version int       `json:"version"`
	Content []adfNode `json:"content"`
}

type adfNode struct {
	Type    string    `json:"type"`
	Content []adfNode `json:"content,omitempty"`
	Text    string    `json:"text,omitempty"`
}

func plainTextADF(text string) adfDoc {
	return adfDoc{
		Type:    "doc",
		Version: 1,
		Content: []adfNode{{
			Type:    "paragraph",
			Content: []adfNode{{Type: "text", Text: text}},
		}},
	}
}
