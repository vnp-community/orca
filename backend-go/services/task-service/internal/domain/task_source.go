package domain

import (
	"errors"
	"strings"
)

// SourceProvider names the external work-item system a task was started from.
type SourceProvider string

const (
	SourceProviderJira   SourceProvider = "jira"
	SourceProviderLinear SourceProvider = "linear"
	SourceProviderGitHub SourceProvider = "github"
	SourceProviderGitLab SourceProvider = "gitlab"
)

var (
	ErrInvalidSourceProvider = errors.New("task source: provider must be one of jira/linear/github/gitlab")
	ErrEmptySourceRef        = errors.New("task source: ref is required")
)

// TaskSource is the link from a task to the external issue it was started
// from. Ref is provider-native ("ENG-123" for Jira/Linear, "owner/repo#12"
// for GitHub/GitLab) and is the idempotency key together with the project.
type TaskSource struct {
	TaskID    string
	TenantID  string
	ProjectID string
	Provider  SourceProvider
	Ref       string
	URL       string
	// Site is the connection workspace id (Jira: site base URL) that makes Ref
	// unambiguous across sites; "" means unknown (pre-site rows, other providers).
	Site string
}

// NewTaskSource validates and normalizes a source link.
func NewTaskSource(tenantID, projectID string, provider SourceProvider, ref, url, site string) (TaskSource, error) {
	switch provider {
	case SourceProviderJira, SourceProviderLinear, SourceProviderGitHub, SourceProviderGitLab:
	default:
		return TaskSource{}, ErrInvalidSourceProvider
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return TaskSource{}, ErrEmptySourceRef
	}
	if tenantID == "" {
		return TaskSource{}, ErrEmptyTenant
	}
	return TaskSource{TenantID: tenantID, ProjectID: projectID, Provider: provider, Ref: ref, URL: strings.TrimSpace(url), Site: strings.TrimSpace(site)}, nil
}

// ErrSourceAlreadyLinked is returned by a repository when a different task
// already holds the same (tenant, project, provider, site, ref) — the caller
// resolves the race by re-reading the winner instead of failing the user.
var ErrSourceAlreadyLinked = errors.New("task source: already linked to another task")
