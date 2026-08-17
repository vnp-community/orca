// Package domain holds issue-tracking-service's entities and value objects.
// Per specs/backend-go/architecture/03-clean-architecture-guidelines.md,
// this package has zero imports outside stdlib — no database, no gRPC, no
// framework, no provider-specific (Jira/Linear) wire shapes.
package domain

import "errors"

// Provider distinguishes which external issue tracker an Issue/operation
// targets — Jira and Linear, per issue-tracking-service.md §4's
// adapter-per-provider pattern (same shape as scm-integration-service's
// GitHub/GitLab/Bitbucket split, applied to Jira/Linear).
type Provider string

const (
	ProviderJira   Provider = "jira"
	ProviderLinear Provider = "linear"
)

// Valid reports whether p is one of the known provider enum values.
func (p Provider) Valid() bool {
	switch p {
	case ProviderJira, ProviderLinear:
		return true
	default:
		return false
	}
}

var (
	// ErrInvalidProvider is returned when Provider isn't one of the known
	// enum values.
	ErrInvalidProvider = errors.New("domain: invalid issue provider")
	// ErrEmptyID guards against an issue with no provider-native
	// identifier — meaningless for round-tripping mutations back to
	// Jira/Linear (design doc §4: "raw provider ID for round-tripping").
	ErrEmptyID = errors.New("domain: issue id is required")
	// ErrEmptyTitle guards against a title-less issue, which every
	// provider's own API rejects too.
	ErrEmptyTitle = errors.New("domain: issue title is required")
)

// Issue is issue-tracking-service's provider-agnostic value object — mirrors
// the wire shape of orca.issuetracking.v1.Issue (see
// proto/orca/issuetracking/v1/issuetracking.proto): a provider-native ID
// (Jira key like "PROJ-1" or Linear issue identifier), title, workflow
// state name, and a browsable URL back to the provider. issue-tracking-service
// never persists this — every read is live against Jira/Linear (design doc §2).
type Issue struct {
	ID    string
	Title string
	State string
	URL   string
}

// NewIssue constructs an Issue, enforcing the minimal invariants every
// provider adapter's response (or create-issue input) must satisfy to be a
// meaningful domain value — this is where "issue-tracking-service owns this
// data's shape correctness" lives, not scattered validation in adapters.
func NewIssue(id, title, state, url string) (Issue, error) {
	if id == "" {
		return Issue{}, ErrEmptyID
	}
	if title == "" {
		return Issue{}, ErrEmptyTitle
	}
	return Issue{ID: id, Title: title, State: state, URL: url}, nil
}
