// Package jira implements usecase.IssueTrackerProvider against Jira's REST
// API — a plain HTTP client, no SDK gap to work around (design doc §4:
// "Jira's adapter has no equivalent gap [to Linear]: a plain REST client
// either way").
//
// CR-JIRA-001 (2026-09-15): originally Cloud-only (hardcoded /rest/api/3/ +
// email/API-token Basic Auth). Confirmed live against a real self-hosted
// site (jr.servicehub.vn) that this always failed — Server/Data Center has
// no /rest/api/3/ at all (Cloud-only API version), so Whoami never reached
// the site's real auth check. See specs/backend-go/bugs/missing-v2/
// BUG-013-jira-adapter-cloud-only-rejects-self-hosted-jira.md.
package jira

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
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

// Client is a real Jira REST API client — API version (v2 vs v3) and auth
// scheme (Basic email:token vs Bearer PAT) are resolved per-site, see
// resolveAPIVersion/authHeaderValue below (CR-JIRA-001).
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

// ── API version resolution (CR-JIRA-001) ────────────────────────────────

type apiVersionCacheEntry struct {
	version   string
	expiresAt time.Time
}

// apiVersionCache avoids re-probing serverInfo on every single API call —
// keyed by baseURL, since deployment type never changes for a given site.
var apiVersionCache sync.Map // map[string]apiVersionCacheEntry

const apiVersionCacheTTL = 10 * time.Minute

// resolveAPIVersion detects whether cred.BaseURL is Jira Cloud (API v3
// available) or Server/Data Center (v3 doesn't exist — Cloud-only — must
// use v2) by calling the v2 serverInfo endpoint, which exists on BOTH
// deployment types, and reading its deploymentType field. Falls back to "3"
// (the pre-CR-JIRA-001 Cloud-only default) on any probe failure — an
// unreachable/unusual site degrades to the previous behavior rather than a
// new failure mode, and a real Cloud site's own requests are unaffected
// either way (this cache never returns an error itself, only a version
// string every call site can use unconditionally).
func (c *Client) resolveAPIVersion(ctx context.Context, cred usecase.Credential) string {
	if cached, ok := apiVersionCache.Load(cred.BaseURL); ok {
		if entry, ok := cached.(apiVersionCacheEntry); ok && time.Now().Before(entry.expiresAt) {
			return entry.version
		}
	}
	version := c.probeAPIVersion(ctx, cred)
	apiVersionCache.Store(cred.BaseURL, apiVersionCacheEntry{version: version, expiresAt: time.Now().Add(apiVersionCacheTTL)})
	return version
}

func (c *Client) probeAPIVersion(ctx context.Context, cred usecase.Credential) string {
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/2/serverInfo"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "3"
	}
	req.Header.Set("Authorization", authHeaderValue(cred))
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "3"
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "3"
	}
	var parsed struct {
		DeploymentType string `json:"deploymentType"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "3"
	}
	if parsed.DeploymentType == "Server" || parsed.DeploymentType == "Data Center" {
		return "2"
	}
	return "3" // "Cloud", or empty/unrecognized — same safe default as before this CR.
}

// authHeaderValue picks Basic (Cloud's email:api-token convention) when an
// email is present, else Bearer (Server/Data Center's Personal Access Token
// convention — CR-JIRA-001) — a caller with only a token and no email is
// assumed to be authenticating with a PAT against a self-hosted instance.
// Does not change behavior for any existing Cloud caller (all of which
// already send both email and token).
func authHeaderValue(cred usecase.Credential) string {
	if cred.Email == "" {
		return "Bearer " + cred.Token
	}
	return "Basic " + basicAuth(cred.Email, cred.Token)
}

func apiURL(baseURL, apiVersion, path string) string {
	return strings.TrimRight(baseURL, "/") + "/rest/api/" + apiVersion + path
}

// ── Whoami ───────────────────────────────────────────────────────────────

// jiraMyselfResponse mirrors GET .../myself's JSON shape — identical on v2
// and v3.
type jiraMyselfResponse struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
}

// Whoami calls Jira's .../myself to verify cred and identify the
// authenticated account — the first call Connect makes, before anything is
// persisted.
func (c *Client) Whoami(ctx context.Context, cred usecase.Credential) (domain.Viewer, error) {
	if cred.BaseURL == "" {
		return domain.Viewer{}, fmt.Errorf("jira: credential is missing a site base URL")
	}
	apiVersion := c.resolveAPIVersion(ctx, cred)
	u := apiURL(cred.BaseURL, apiVersion, "/myself")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domain.Viewer{}, fmt.Errorf("jira: building whoami request: %w", err)
	}
	req.Header.Set("Authorization", authHeaderValue(cred))
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

// jiraSearchResponse mirrors the subset of Jira's GET .../search JSON
// response shape this adapter needs — identical on v2 and v3.
type jiraSearchResponse struct {
	Issues []jiraIssue `json:"issues"`
}

type jiraIssue struct {
	Key    string          `json:"key"`
	Fields jiraIssueFields `json:"fields"`
}

// jiraIssueFields is the subset of Jira's real `fields` object this adapter
// reads — identical shape on v2 (Server/Data Center) and v3 (Cloud).
//
// BUG-016: this used to only declare Summary/Status — Project/IssueType/
// Assignee/Reporter/Priority/Labels were silently dropped for every issue
// SearchIssues/GetIssue ever returned, even though domain.Issue (and the
// wire proto) has always had fields for all of them. Project/IssueType are
// non-optional on the frontend's JiraIssue type — a live-confirmed crash
// ("Cannot read properties of undefined (reading 'key')" on issue.project.key)
// is what surfaced this, once CR-TSRC-001's capability fix let a real Jira
// fetch reach the frontend for the first time.
type jiraIssueFields struct {
	Summary string `json:"summary"`
	Status  struct {
		Name           string `json:"name"`
		StatusCategory struct {
			Key string `json:"key"`
		} `json:"statusCategory"`
	} `json:"status"`
	Project   jiraProjectField   `json:"project"`
	IssueType jiraIssueTypeField `json:"issuetype"`
	Assignee  *jiraUserField     `json:"assignee"`
	Reporter  *jiraUserField     `json:"reporter"`
	Priority  jiraPriorityField  `json:"priority"`
	Labels    []string           `json:"labels"`
}

type jiraProjectField struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type jiraIssueTypeField struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

// jiraUserField covers both Jira Cloud's accountId-keyed user identity and
// Jira Server/Data Center's name/key-keyed one (CR-JIRA-001 already
// established this exact Cloud-vs-Server/DC duality for auth; the same
// duality applies to user references inside issue payloads). Pointer in
// jiraIssueFields (not a value) because Jira omits assignee/reporter
// entirely — not an empty object — when unset (an unassigned issue), and a
// nil *jiraUserField round-trips that distinction; toUserRef below then
// maps nil to domain.UserRef{}'s zero value, same as any other unset ref.
type jiraUserField struct {
	AccountID    string `json:"accountId"`
	Name         string `json:"name"`
	Key          string `json:"key"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	AvatarUrls   struct {
		Large string `json:"48x48"`
	} `json:"avatarUrls"`
}

func (u *jiraUserField) toUserRef() domain.UserRef {
	if u == nil {
		return domain.UserRef{}
	}
	id := u.AccountID
	if id == "" {
		id = u.Key
	}
	if id == "" {
		id = u.Name
	}
	return domain.UserRef{ID: id, DisplayName: u.DisplayName, Email: u.EmailAddress, AvatarURL: u.AvatarUrls.Large}
}

type jiraPriorityField struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// toRichIssue maps a jiraIssue search/get-result row into the extended
// domain.Issue.
func toRichIssue(baseURL string, ji jiraIssue) domain.Issue {
	return domain.Issue{
		ID: ji.Key, ProviderIssueID: ji.Key, Key: ji.Key, Title: ji.Fields.Summary,
		State:         ji.Fields.Status.Name,
		WorkflowState: domain.WorkflowState{Name: ji.Fields.Status.Name, Category: normalizeStatusCategory(ji.Fields.Status.StatusCategory.Key)},
		URL:           issueBrowseURL(baseURL, ji.Key),
		Project:       domain.ProjectRef{ID: ji.Fields.Project.ID, Key: ji.Fields.Project.Key, Name: ji.Fields.Project.Name},
		IssueType:     domain.IssueTypeRef{ID: ji.Fields.IssueType.ID, Name: ji.Fields.IssueType.Name, Subtask: ji.Fields.IssueType.Subtask},
		Assignee:      ji.Fields.Assignee.toUserRef(),
		Reporter:      ji.Fields.Reporter.toUserRef(),
		Priority:      domain.PriorityRef{ID: ji.Fields.Priority.ID, Name: ji.Fields.Priority.Name},
		Labels:        ji.Fields.Labels,
	}
}

func (c *Client) SearchIssues(ctx context.Context, cred usecase.Credential, jql string, limit int) ([]domain.Issue, error) {
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}
	apiVersion := c.resolveAPIVersion(ctx, cred)
	u, err := url.Parse(apiURL(cred.BaseURL, apiVersion, "/search"))
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
	req.Header.Set("Authorization", authHeaderValue(cred))
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
		issues = append(issues, toRichIssue(cred.BaseURL, ji))
	}
	return issues, nil
}

// ListIssues performs a real GET against Jira's .../search, JQL filtered to
// projectKey when set. filterJSON (a JiraIssueFilter-shaped object) is not
// translated to JQL here — a documented gap: a follow-up can extend this to
// parse filterJSON's structured fields (status/assignee/labels) into
// additional JQL clauses once a concrete JiraIssueFilter shape is finalized.
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
	apiVersion := c.resolveAPIVersion(ctx, cred)
	u := apiURL(cred.BaseURL, apiVersion, "/issue/"+url.PathEscape(issueID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: building get issue request: %w", err)
	}
	req.Header.Set("Authorization", authHeaderValue(cred))
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
	return toRichIssue(cred.BaseURL, parsed), nil
}

// ── CreateIssue / UpdateIssue ────────────────────────────────────────────

// jiraCreateIssueRequest mirrors POST .../issue's request body.
type jiraCreateIssueRequest struct {
	Fields jiraCreateIssueFields `json:"fields"`
}

type jiraCreateIssueFields struct {
	Project jiraProjectRef `json:"project"`
	Summary string         `json:"summary"`
	// Description is `any`, not `*adfDoc`: API v3 (Cloud) requires ADF for
	// rich text; API v2 (Server/Data Center) takes a plain string instead
	// (CR-JIRA-001) — see CreateIssue's version branch below.
	Description any              `json:"description,omitempty"`
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

// CreateIssue performs a real POST against Jira's .../issue. On API v3
// (Cloud), Description is wrapped in a minimal Atlassian Document Format
// (ADF) document, which v3 requires for rich text fields; on v2 (Server/
// Data Center — CR-JIRA-001), Description is sent as a plain string, which
// is what v2 expects instead. The issue type sent is resolved against the
// project's real issue types (listIssueTypes) rather than blindly
// hardcoded, driven by in.IssueTypeID (falling back to defaultIssueType/
// resolveIssueType when unset) — see resolveIssueType and
// defaultIssueType's doc comment.
func (c *Client) CreateIssue(ctx context.Context, cred usecase.Credential, in domain.NewIssueInput) (domain.Issue, error) {
	if cred.BaseURL == "" {
		return domain.Issue{}, fmt.Errorf("jira: credential is missing a site base URL")
	}
	if in.ProjectKey == "" {
		return domain.Issue{}, fmt.Errorf("jira: project_key is required")
	}
	apiVersion := c.resolveAPIVersion(ctx, cred)

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
		if apiVersion == "2" {
			fields.Description = in.Description
		} else {
			fields.Description = plainTextADF(in.Description)
		}
	}
	body, err := json.Marshal(jiraCreateIssueRequest{Fields: fields})
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: marshal create issue request: %w", err)
	}
	u := apiURL(cred.BaseURL, apiVersion, "/issue")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: building create issue request: %w", err)
	}
	req.Header.Set("Authorization", authHeaderValue(cred))
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
	// Jira's create response only carries {id, key, self} — no fields — so
	// Project/IssueType are synthesized from what this call already knows
	// (the values just sent), same as Title/Summary already was before
	// BUG-016. Everything else (assignee, priority, labels...) legitimately
	// has no value yet for a freshly created issue.
	return toRichIssue(cred.BaseURL, jiraIssue{
		Key: parsed.Key,
		Fields: jiraIssueFields{
			Summary:   in.Title,
			Project:   jiraProjectField{Key: in.ProjectKey},
			IssueType: jiraIssueTypeField{Name: issueTypeName},
		},
	}), nil
}

func (c *Client) UpdateIssue(ctx context.Context, cred usecase.Credential, in domain.IssueUpdate) (domain.Issue, error) {
	if cred.BaseURL == "" {
		return domain.Issue{}, fmt.Errorf("jira: credential is missing a site base URL")
	}
	apiVersion := c.resolveAPIVersion(ctx, cred)
	fields := map[string]any{}
	if in.Title != "" {
		fields["summary"] = in.Title
	}
	if in.Description != "" {
		// Same v2-plain-string / v3-ADF split as CreateIssue — see its doc comment.
		if apiVersion == "2" {
			fields["description"] = in.Description
		} else {
			fields["description"] = plainTextADF(in.Description)
		}
	}
	if len(fields) == 0 && in.WorkflowStateID != "" {
		// Status change only: nothing to PUT (an empty fields update is a no-op
		// that would also report success).
		if err := c.transitionIssue(ctx, cred, apiVersion, in.IssueID, in.WorkflowStateID); err != nil {
			return domain.Issue{}, err
		}
		return c.GetIssue(ctx, cred, in.IssueID)
	}
	body, err := json.Marshal(map[string]any{"fields": fields})
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: marshal update issue request: %w", err)
	}
	u := apiURL(cred.BaseURL, apiVersion, "/issue/"+url.PathEscape(in.IssueID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: building update issue request: %w", err)
	}
	req.Header.Set("Authorization", authHeaderValue(cred))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: update issue request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return domain.Issue{}, jiraStatusError("update issue", resp)
	}
	if in.WorkflowStateID != "" {
		if err := c.transitionIssue(ctx, cred, apiVersion, in.IssueID, in.WorkflowStateID); err != nil {
			return domain.Issue{}, err
		}
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
	apiVersion := c.resolveAPIVersion(ctx, cred)
	// Comment body: same v2-plain-string / v3-ADF split as CreateIssue.
	var bodyField any = bodyMarkdown
	if apiVersion != "2" {
		bodyField = plainTextADF(bodyMarkdown)
	}
	body, err := json.Marshal(map[string]any{"body": bodyField})
	if err != nil {
		return domain.IssueComment{}, fmt.Errorf("jira: marshal add comment request: %w", err)
	}
	u := apiURL(cred.BaseURL, apiVersion, "/issue/"+url.PathEscape(issueID)+"/comment")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return domain.IssueComment{}, fmt.Errorf("jira: building add comment request: %w", err)
	}
	req.Header.Set("Authorization", authHeaderValue(cred))
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
	apiVersion := c.resolveAPIVersion(ctx, cred)
	u := apiURL(cred.BaseURL, apiVersion, "/issue/"+url.PathEscape(issueID)+"/comment")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list comments request: %w", err)
	}
	req.Header.Set("Authorization", authHeaderValue(cred))
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

// jiraProjectListItem is the {id,key,name} shape shared by both of
// ListProjects' endpoints — only the response envelope around it differs
// (a flat array on v2 vs a paginated {values:[...]} object on v3).
type jiraProjectListItem struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// ListProjects. BUG-017: Jira Server/Data Center below version 8.4 has no
// paginated GET /project/search endpoint at all — that path segment isn't
// recognized as a distinct sub-resource, so Jira's router falls through to
// the older single-project-lookup route (GET /project/{projectIdOrKey}),
// treating the literal string "search" as a project key. Live-confirmed via
// direct curl against jr.servicehub.vn: /project/search → 404 "No project
// could be found with key 'search'"; /project → 200, a flat JSON array (not
// {values:[...]}). v2 (Server/Data Center) must use the older /project
// endpoint and parse its flat-array shape; v3 (Cloud) keeps using
// /project/search's paginated {values:[...]} shape unchanged — same
// Cloud-vs-Server/DC duality CR-JIRA-001 already established for this
// adapter's auth/API-version handling, just not yet applied to this
// specific endpoint.
func (c *Client) ListProjects(ctx context.Context, cred usecase.Credential, workspaceID string) ([]domain.ProjectRef, error) {
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}
	apiVersion := c.resolveAPIVersion(ctx, cred)
	path := "/project/search"
	if apiVersion == "2" {
		path = "/project"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL(cred.BaseURL, apiVersion, path), nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list projects request: %w", err)
	}
	req.Header.Set("Authorization", authHeaderValue(cred))
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: list projects request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, jiraStatusError("list projects", resp)
	}

	var items []jiraProjectListItem
	if apiVersion == "2" {
		if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
			return nil, fmt.Errorf("jira: decoding list projects response: %w", err)
		}
	} else {
		var parsed struct {
			Values []jiraProjectListItem `json:"values"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
			return nil, fmt.Errorf("jira: decoding list projects response: %w", err)
		}
		items = parsed.Values
	}

	out := make([]domain.ProjectRef, 0, len(items))
	for _, p := range items {
		out = append(out, domain.ProjectRef{ID: p.ID, Key: p.Key, Name: p.Name, WorkspaceID: workspaceID})
	}
	return out, nil
}

// jiraIssueTypeMeta mirrors one entry of Jira's
// GET .../issue/createmeta/{projectIdOrKey}/issuetypes JSON response — the
// current (non-deprecated) endpoint for discovering which issue types are
// actually creatable on a project, present on both v2 and v3.
type jiraIssueTypeMeta struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

// jiraIssueTypesResponse mirrors the paginated envelope
// GET .../issuetypes returns. This adapter doesn't page through it — Jira
// projects have a small, bounded number of issue types, well under a
// single page's maxResults.
type jiraIssueTypesResponse struct {
	Values []jiraIssueTypeMeta `json:"values"`
}

// listIssueTypes performs a real GET against Jira's
// .../issue/createmeta/{projectIdOrKey}/issuetypes, returning the issue
// types Jira actually allows creating on projectKey. CreateIssue calls this
// internally; ListIssueTypes (below) is the exported wrapper reachable from
// the gRPC surface.
func (c *Client) listIssueTypes(ctx context.Context, cred usecase.Credential, projectKey string) ([]jiraIssueTypeMeta, error) {
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}
	apiVersion := c.resolveAPIVersion(ctx, cred)
	u := apiURL(cred.BaseURL, apiVersion, "/issue/createmeta/"+url.PathEscape(projectKey)+"/issuetypes")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list issue types request: %w", err)
	}
	req.Header.Set("Authorization", authHeaderValue(cred))
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
	apiVersion := c.resolveAPIVersion(ctx, cred)
	u := apiURL(cred.BaseURL, apiVersion, "/issue/createmeta/"+url.PathEscape(projectIDOrKey)+"/issuetypes/"+url.PathEscape(issueTypeID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list create fields request: %w", err)
	}
	req.Header.Set("Authorization", authHeaderValue(cred))
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
	apiVersion := c.resolveAPIVersion(ctx, cred)
	u, err := url.Parse(apiURL(cred.BaseURL, apiVersion, "/user/assignable/search"))
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
	req.Header.Set("Authorization", authHeaderValue(cred))
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
	apiVersion := c.resolveAPIVersion(ctx, cred)
	u := apiURL(cred.BaseURL, apiVersion, "/priority")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list priorities request: %w", err)
	}
	req.Header.Set("Authorization", authHeaderValue(cred))
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
	apiVersion := c.resolveAPIVersion(ctx, cred)
	u := apiURL(cred.BaseURL, apiVersion, "/issue/"+url.PathEscape(issueID)+"/transitions")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list transitions request: %w", err)
	}
	req.Header.Set("Authorization", authHeaderValue(cred))
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

// GetProjectStatusOrder: Jira has no single "column order" REST endpoint on
// either API version — this would need the project's board configuration
// via the Agile API (/rest/agile/1.0/board?projectKeyOrId=... then
// /rest/agile/1.0/board/{id}/configuration's columnConfig.columns, each
// column's statuses list giving one entry of StatusIdsByColumn). Left as a
// documented gap rather than guessed at: no live Jira site was available in
// this environment to confirm the exact response shape.
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
// carry a plain-text description, not the full ADF node-type surface. Only
// used for API v3 (Cloud) — v2 (Server/Data Center, CR-JIRA-001) takes a
// plain string instead.
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

// ErrTransitionUnavailable means the issue has no transition to the requested
// status from where it is now. Returned instead of pretending success: a caller
// asking to move an issue must be able to tell it did not move.
var ErrTransitionUnavailable = errors.New("jira: transition unavailable")

// transitionIssue moves the issue to the status named target (a status name, or
// a transition id). Jira only offers the transitions its workflow allows from
// the current status, so this never forces an illegal move: an issue already in
// the target status is left alone, and one with no way there is an error.
func (c *Client) transitionIssue(ctx context.Context, cred usecase.Credential, apiVersion, issueID, target string) error {
	if current, err := c.GetIssue(ctx, cred, issueID); err == nil && strings.EqualFold(strings.TrimSpace(current.State), strings.TrimSpace(target)) {
		return nil
	}
	transitions, err := c.ListTransitions(ctx, cred, issueID)
	if err != nil {
		return err
	}
	chosen, ok := pickTransition(transitions, target)
	if !ok {
		names := make([]string, 0, len(transitions))
		for _, t := range transitions {
			names = append(names, t.To.Name)
		}
		return fmt.Errorf("%w: %s has no transition to %q (available: %s)", ErrTransitionUnavailable, issueID, target, strings.Join(names, ", "))
	}
	body, err := json.Marshal(map[string]any{"transition": map[string]any{"id": chosen.ID}})
	if err != nil {
		return fmt.Errorf("jira: marshal transition request: %w", err)
	}
	u := apiURL(cred.BaseURL, apiVersion, "/issue/"+url.PathEscape(issueID)+"/transitions")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("jira: building transition request: %w", err)
	}
	req.Header.Set("Authorization", authHeaderValue(cred))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("jira: transition request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return jiraStatusError("transition issue", resp)
	}
	return nil
}

// pickTransition resolves target to one of the available transitions: an exact
// transition id first, then the destination status name, then the transition's
// own name — the order that keeps a caller-supplied id from being shadowed by a
// status that happens to share its text.
func pickTransition(transitions []domain.Transition, target string) (domain.Transition, bool) {
	target = strings.TrimSpace(target)
	for _, t := range transitions {
		if t.ID == target {
			return t, true
		}
	}
	for _, t := range transitions {
		if strings.EqualFold(strings.TrimSpace(t.To.Name), target) {
			return t, true
		}
	}
	for _, t := range transitions {
		if strings.EqualFold(strings.TrimSpace(t.Name), target) {
			return t, true
		}
	}
	return domain.Transition{}, false
}

// normalizeStatusCategory maps Jira's statusCategory.key (new/indeterminate/
// done) to the domain's todo/in_progress/done vocabulary (issuetracking.proto's
// WorkflowState.category). Unknown keys stay empty so callers treat the
// category as unknown rather than guessing.
func normalizeStatusCategory(key string) string {
	switch key {
	case "new":
		return "todo"
	case "indeterminate":
		return "in_progress"
	case "done":
		return "done"
	default:
		return ""
	}
}
