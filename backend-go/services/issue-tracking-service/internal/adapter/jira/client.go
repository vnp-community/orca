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
// caller doesn't specify one. CreateIssue resolves it against the
// project's real issue types first (via listIssueTypes) and only uses this
// name if a case-insensitive match for it actually exists on the target
// project; otherwise it falls back to the first non-subtask type. Jira
// sites vary in which type names exist, which is exactly what this lookup
// guards against.
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

// ── Whoami ───────────────────────────────────────────────────────────────

// jiraMyselfResponse mirrors GET /rest/api/3/myself's JSON shape.
type jiraMyselfResponse struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
}

// Whoami calls Jira's /rest/api/3/myself to verify cred and identify the
// authenticated account — the first call Connect makes, before anything is
// persisted.
func (c *Client) Whoami(ctx context.Context, cred usecase.Credential) (domain.Viewer, error) {
	if cred.BaseURL == "" {
		return domain.Viewer{}, fmt.Errorf("jira: credential is missing a site base URL")
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/myself"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domain.Viewer{}, fmt.Errorf("jira: building whoami request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.Viewer{}, fmt.Errorf("jira: whoami request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.Viewer{}, jiraStatusError("whoami", resp)
	}
	var parsed jiraMyselfResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return domain.Viewer{}, fmt.Errorf("jira: decoding whoami response: %w", err)
	}
	return domain.Viewer{ID: parsed.AccountID, DisplayName: parsed.DisplayName, Email: parsed.EmailAddress}, nil
}

// ── SearchIssues / ListIssues / GetIssue ────────────────────────────────

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

// toRichIssue maps a jiraIssue search-result row into the extended
// domain.Issue.
func toRichIssue(baseURL string, key, summary, status string) domain.Issue {
	return domain.Issue{
		ID: key, ProviderIssueID: key, Key: key, Title: summary,
		State: status, WorkflowState: domain.WorkflowState{Name: status},
		URL: issueBrowseURL(baseURL, key),
	}
}

func (c *Client) SearchIssues(ctx context.Context, cred usecase.Credential, jql string, limit int) ([]domain.Issue, error) {
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}
	u, err := url.Parse(strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/search")
	if err != nil {
		return nil, fmt.Errorf("jira: invalid base url: %w", err)
	}
	q := u.Query()
	q.Set("jql", jql)
	if limit <= 0 {
		limit = 50
	}
	q.Set("maxResults", fmt.Sprintf("%d", limit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building search issues request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: search issues request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, jiraStatusError("search issues", resp)
	}
	var parsed jiraSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("jira: decoding search issues response: %w", err)
	}
	issues := make([]domain.Issue, 0, len(parsed.Issues))
	for _, ji := range parsed.Issues {
		issues = append(issues, toRichIssue(cred.BaseURL, ji.Key, ji.Fields.Summary, ji.Fields.Status.Name))
	}
	return issues, nil
}

// ListIssues performs a real GET against Jira's /rest/api/3/search, JQL
// filtered to projectKey when set. filterJSON (a JiraIssueFilter-shaped
// object) is not translated to JQL here — a documented gap: a follow-up
// can extend this to parse filterJSON's structured fields (status/
// assignee/labels) into additional JQL clauses once a concrete
// JiraIssueFilter shape is finalized.
func (c *Client) ListIssues(ctx context.Context, cred usecase.Credential, projectKey, filterJSON string, limit int) ([]domain.Issue, error) {
	jql := ""
	if projectKey != "" {
		jql = fmt.Sprintf("project=%q", projectKey)
	}
	_ = filterJSON
	return c.SearchIssues(ctx, cred, jql, limit)
}

func (c *Client) GetIssue(ctx context.Context, cred usecase.Credential, issueID string) (domain.Issue, error) {
	if cred.BaseURL == "" {
		return domain.Issue{}, fmt.Errorf("jira: credential is missing a site base URL")
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/issue/" + url.PathEscape(issueID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: building get issue request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: get issue request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.Issue{}, jiraStatusError("get issue", resp)
	}
	var parsed jiraIssue
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return domain.Issue{}, fmt.Errorf("jira: decoding get issue response: %w", err)
	}
	return toRichIssue(cred.BaseURL, parsed.Key, parsed.Fields.Summary, parsed.Fields.Status.Name), nil
}

// ── CreateIssue / UpdateIssue ────────────────────────────────────────────

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
// issue types (listIssueTypes) rather than blindly hardcoded, driven by
// in.IssueTypeID (falling back to defaultIssueType/resolveIssueType when
// unset) — see resolveIssueType and defaultIssueType's doc comment.
func (c *Client) CreateIssue(ctx context.Context, cred usecase.Credential, in domain.NewIssueInput) (domain.Issue, error) {
	if cred.BaseURL == "" {
		return domain.Issue{}, fmt.Errorf("jira: credential is missing a site base URL")
	}
	if in.ProjectKey == "" {
		return domain.Issue{}, fmt.Errorf("jira: project_key is required")
	}

	issueTypeName := in.IssueTypeID
	if issueTypeName == "" {
		types, err := c.listIssueTypes(ctx, cred, in.ProjectKey)
		if err != nil {
			return domain.Issue{}, fmt.Errorf("jira: resolving issue type: %w", err)
		}
		issueTypeName, err = resolveIssueType(types, defaultIssueType)
		if err != nil {
			return domain.Issue{}, fmt.Errorf("jira: resolving issue type: %w", err)
		}
	}

	fields := jiraCreateIssueFields{
		Project:   jiraProjectRef{Key: in.ProjectKey},
		Summary:   in.Title,
		IssueType: jiraIssueTypeRef{Name: issueTypeName},
	}
	if in.Description != "" {
		doc := plainTextADF(in.Description)
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
	return toRichIssue(cred.BaseURL, parsed.Key, in.Title, ""), nil
}

func (c *Client) UpdateIssue(ctx context.Context, cred usecase.Credential, in domain.IssueUpdate) (domain.Issue, error) {
	if cred.BaseURL == "" {
		return domain.Issue{}, fmt.Errorf("jira: credential is missing a site base URL")
	}
	fields := map[string]any{}
	if in.Title != "" {
		fields["summary"] = in.Title
	}
	if in.Description != "" {
		doc := plainTextADF(in.Description)
		fields["description"] = doc
	}
	body, err := json.Marshal(map[string]any{"fields": fields})
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: marshal update issue request: %w", err)
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/issue/" + url.PathEscape(in.IssueID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: building update issue request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: update issue request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return domain.Issue{}, jiraStatusError("update issue", resp)
	}
	// Jira's PUT /issue/{id} returns 204 No Content — re-fetch to return the
	// caller a current view, same convention GetIssue already uses.
	return c.GetIssue(ctx, cred, in.IssueID)
}

// ── AddIssueComment / ListIssueComments ─────────────────────────────────

func (c *Client) AddIssueComment(ctx context.Context, cred usecase.Credential, issueID, bodyMarkdown string) (domain.IssueComment, error) {
	if cred.BaseURL == "" {
		return domain.IssueComment{}, fmt.Errorf("jira: credential is missing a site base URL")
	}
	doc := plainTextADF(bodyMarkdown)
	body, err := json.Marshal(map[string]any{"body": doc})
	if err != nil {
		return domain.IssueComment{}, fmt.Errorf("jira: marshal add comment request: %w", err)
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/issue/" + url.PathEscape(issueID) + "/comment"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return domain.IssueComment{}, fmt.Errorf("jira: building add comment request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.IssueComment{}, fmt.Errorf("jira: add comment request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		return domain.IssueComment{}, jiraStatusError("add comment", resp)
	}
	var parsed struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return domain.IssueComment{}, fmt.Errorf("jira: decoding add comment response: %w", err)
	}
	return domain.IssueComment{ID: parsed.ID, BodyMarkdown: bodyMarkdown}, nil
}

func (c *Client) ListIssueComments(ctx context.Context, cred usecase.Credential, issueID string) ([]domain.IssueComment, error) {
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/issue/" + url.PathEscape(issueID) + "/comment"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list comments request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: list comments request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, jiraStatusError("list comments", resp)
	}
	var parsed struct {
		Comments []struct {
			ID   string `json:"id"`
			Body struct {
				Content []struct {
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
				} `json:"content"`
			} `json:"body"`
		} `json:"comments"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("jira: decoding list comments response: %w", err)
	}
	out := make([]domain.IssueComment, 0, len(parsed.Comments))
	for _, cm := range parsed.Comments {
		text := ""
		if len(cm.Body.Content) > 0 && len(cm.Body.Content[0].Content) > 0 {
			text = cm.Body.Content[0].Content[0].Text
		}
		out = append(out, domain.IssueComment{ID: cm.ID, BodyMarkdown: text})
	}
	return out, nil
}

// ── metadata lookups (ListProjects / ListIssueTypes / ListCreateFields /
// ListAssignableUsers / ListPriorities / ListTransitions /
// GetProjectStatusOrder) ────────────────────────────────────────────────

func (c *Client) ListProjects(ctx context.Context, cred usecase.Credential, workspaceID string) ([]domain.ProjectRef, error) {
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/project/search"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list projects request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: list projects request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, jiraStatusError("list projects", resp)
	}
	var parsed struct {
		Values []struct {
			ID   string `json:"id"`
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("jira: decoding list projects response: %w", err)
	}
	out := make([]domain.ProjectRef, 0, len(parsed.Values))
	for _, p := range parsed.Values {
		out = append(out, domain.ProjectRef{ID: p.ID, Key: p.Key, Name: p.Name, WorkspaceID: workspaceID})
	}
	return out, nil
}

// jiraIssueTypeMeta mirrors one entry of Jira Cloud's
// GET /rest/api/3/issue/createmeta/{projectIdOrKey}/issuetypes JSON
// response — the current (non-deprecated) endpoint for discovering which
// issue types are actually creatable on a project.
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
// issue types Jira actually allows creating on projectKey. CreateIssue
// calls this internally; ListIssueTypes (below) is the exported wrapper
// reachable from the gRPC surface.
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

// ListIssueTypes is the exported wrapper reachable from the gRPC surface —
// thin wrapper over the internal listIssueTypes CreateIssue already used.
func (c *Client) ListIssueTypes(ctx context.Context, cred usecase.Credential, projectIDOrKey string) ([]domain.IssueTypeRef, error) {
	types, err := c.listIssueTypes(ctx, cred, projectIDOrKey)
	if err != nil {
		return nil, err
	}
	out := make([]domain.IssueTypeRef, 0, len(types))
	for _, t := range types {
		out = append(out, domain.IssueTypeRef{ID: t.ID, Name: t.Name, Subtask: t.Subtask})
	}
	return out, nil
}

func (c *Client) ListCreateFields(ctx context.Context, cred usecase.Credential, projectIDOrKey, issueTypeID string) ([]domain.CreateField, error) {
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/issue/createmeta/" +
		url.PathEscape(projectIDOrKey) + "/issuetypes/" + url.PathEscape(issueTypeID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list create fields request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: list create fields request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, jiraStatusError("list create fields", resp)
	}
	var parsed struct {
		Fields []struct {
			FieldID  string `json:"fieldId"`
			Name     string `json:"name"`
			Required bool   `json:"required"`
			Schema   struct {
				Type   string `json:"type"`
				Items  string `json:"items"`
				Custom string `json:"custom"`
			} `json:"schema"`
			AllowedValues []struct {
				ID    string `json:"id"`
				Value string `json:"value"`
				Name  string `json:"name"`
			} `json:"allowedValues"`
		} `json:"fields"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("jira: decoding list create fields response: %w", err)
	}
	out := make([]domain.CreateField, 0, len(parsed.Fields))
	for _, f := range parsed.Fields {
		cf := domain.CreateField{
			Key: f.FieldID, Name: f.Name, Required: f.Required,
			SchemaType: f.Schema.Type, SchemaItems: f.Schema.Items, SchemaCustom: f.Schema.Custom,
		}
		for _, av := range f.AllowedValues {
			cf.AllowedValues = append(cf.AllowedValues, domain.CreateFieldOption{ID: av.ID, Value: av.Value, Name: av.Name})
		}
		out = append(out, cf)
	}
	return out, nil
}

func (c *Client) ListAssignableUsers(ctx context.Context, cred usecase.Credential, projectIDOrKey, issueID string) ([]domain.UserRef, error) {
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}
	u, err := url.Parse(strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/user/assignable/search")
	if err != nil {
		return nil, fmt.Errorf("jira: invalid base url: %w", err)
	}
	q := u.Query()
	if issueID != "" {
		q.Set("issueKey", issueID)
	} else {
		q.Set("project", projectIDOrKey)
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list assignable users request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: list assignable users request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, jiraStatusError("list assignable users", resp)
	}
	var parsed []struct {
		AccountID    string `json:"accountId"`
		DisplayName  string `json:"displayName"`
		EmailAddress string `json:"emailAddress"`
		AvatarUrls   struct {
			Size32 string `json:"32x32"`
		} `json:"avatarUrls"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("jira: decoding list assignable users response: %w", err)
	}
	out := make([]domain.UserRef, 0, len(parsed))
	for _, u := range parsed {
		out = append(out, domain.UserRef{ID: u.AccountID, DisplayName: u.DisplayName, Email: u.EmailAddress, AvatarURL: u.AvatarUrls.Size32})
	}
	return out, nil
}

func (c *Client) ListPriorities(ctx context.Context, cred usecase.Credential) ([]domain.PriorityRef, error) {
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/priority"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list priorities request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: list priorities request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, jiraStatusError("list priorities", resp)
	}
	var parsed []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("jira: decoding list priorities response: %w", err)
	}
	out := make([]domain.PriorityRef, 0, len(parsed))
	for _, p := range parsed {
		out = append(out, domain.PriorityRef{ID: p.ID, Name: p.Name})
	}
	return out, nil
}

func (c *Client) ListTransitions(ctx context.Context, cred usecase.Credential, issueID string) ([]domain.Transition, error) {
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/issue/" + url.PathEscape(issueID) + "/transitions"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list transitions request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: list transitions request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, jiraStatusError("list transitions", resp)
	}
	var parsed struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			To   struct {
				ID             string `json:"id"`
				Name           string `json:"name"`
				StatusCategory struct {
					Key string `json:"key"`
				} `json:"statusCategory"`
			} `json:"to"`
		} `json:"transitions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("jira: decoding list transitions response: %w", err)
	}
	out := make([]domain.Transition, 0, len(parsed.Transitions))
	for _, t := range parsed.Transitions {
		out = append(out, domain.Transition{
			ID: t.ID, Name: t.Name,
			To: domain.WorkflowState{ID: t.To.ID, Name: t.To.Name, Category: t.To.StatusCategory.Key},
		})
	}
	return out, nil
}

// GetProjectStatusOrder: Jira Cloud has no single "column order" REST
// endpoint — this would need the project's board configuration via the
// Agile API (/rest/agile/1.0/board?projectKeyOrId=... then
// /rest/agile/1.0/board/{id}/configuration's columnConfig.columns, each
// column's statuses list giving one entry of StatusIdsByColumn). Left as a
// documented gap rather than guessed at: no live Jira Cloud site is
// available in this environment to confirm the exact response shape.
func (c *Client) GetProjectStatusOrder(ctx context.Context, cred usecase.Credential, projectIDOrKey string) (domain.ProjectStatusOrder, error) {
	return domain.ProjectStatusOrder{}, fmt.Errorf("jira: GetProjectStatusOrder not yet implemented — see doc comment in this method")
}

// ── CreateProject / GetProject (Linear-shared concept, not wired to any
// jira.* channel — BUG-015's method list has none) ──────────────────────

// CreateProject/GetProject are not wired to any jira.* channel but must
// still satisfy the widened IssueTrackerProvider interface; return a clear
// unsupported error rather than a silent no-op.
func (c *Client) CreateProject(ctx context.Context, cred usecase.Credential, workspaceID, teamID, name, description string) (domain.ProjectRef, error) {
	return domain.ProjectRef{}, fmt.Errorf("jira: CreateProject is not supported — use listProjects/an existing Jira project")
}

func (c *Client) GetProject(ctx context.Context, cred usecase.Credential, projectID, workspaceID string) (domain.ProjectRef, error) {
	return domain.ProjectRef{}, fmt.Errorf("jira: GetProject is not implemented — see listProjects")
}

// ── ListTeams/ListTeamLabels/ListTeamMembers/GetCustomView/
// ListWorkflowStates are Linear-only concepts — implemented only to
// satisfy IssueTrackerProvider, never reached by any jira.* channel. ────

func (c *Client) ListTeams(ctx context.Context, cred usecase.Credential, workspaceID string) ([]domain.Team, error) {
	return nil, fmt.Errorf("jira: ListTeams is not applicable to jira — use listProjects")
}

func (c *Client) ListTeamLabels(ctx context.Context, cred usecase.Credential, teamID string) ([]domain.TeamLabel, error) {
	return nil, fmt.Errorf("jira: ListTeamLabels is not applicable to jira")
}

func (c *Client) ListTeamMembers(ctx context.Context, cred usecase.Credential, teamID string) ([]domain.TeamMember, error) {
	return nil, fmt.Errorf("jira: ListTeamMembers is not applicable to jira — use listAssignableUsers")
}

func (c *Client) GetCustomView(ctx context.Context, cred usecase.Credential, viewID, model string) (domain.CustomView, error) {
	return domain.CustomView{}, fmt.Errorf("jira: GetCustomView is not applicable to jira")
}

func (c *Client) ListWorkflowStates(ctx context.Context, cred usecase.Credential, teamID string) ([]domain.WorkflowState, error) {
	return nil, fmt.Errorf("jira: ListWorkflowStates is not applicable to jira — use getProjectStatusOrder")
}

// ── shared helpers ───────────────────────────────────────────────────────

// resolveIssueType picks which real Jira issue-type name to send on
// CreateIssue, given the project's actual issue types and a preferred name
// (defaultIssueType, "Task", until a caller-requested one is supplied via
// NewIssueInput.IssueTypeID). Preference order: an exact case-insensitive
// match on preferredName; else the first non-subtask type (subtasks
// require a parent issue and can't be the target of a bare top-level
// CreateIssue); else an error — a project with no usable issue type can't
// have an issue created on it, and silently guessing would just trade one
// hardcoded string for another.
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
