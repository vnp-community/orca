// Package domain holds issue-tracking-service's entities and value objects.
// Per specs/backend-go/architecture/03-clean-architecture-guidelines.md,
// this package has zero imports outside stdlib — no database, no gRPC, no
// framework, no provider-specific (Jira/Linear) wire shapes.
package domain

import (
	"errors"
	"time"
)

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
	ID                  string
	ProviderIssueID     string
	Key                 string
	Title               string
	DescriptionMarkdown string
	State               string
	WorkflowState       WorkflowState
	URL                 string
	Project             ProjectRef
	IssueType           IssueTypeRef
	Labels              []string
	Assignee            UserRef
	Reporter            UserRef
	Priority            PriorityRef
	CustomFieldsJSON    string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type ProjectRef struct {
	ID          string
	Key         string
	Name        string
	WorkspaceID string
}

type IssueTypeRef struct {
	ID      string
	Name    string
	Subtask bool
}

type WorkflowState struct {
	ID       string
	Name     string
	Category string // todo|in_progress|done|cancelled
}

type UserRef struct {
	ID          string
	DisplayName string
	Email       string
	AvatarURL   string
}

type PriorityRef struct {
	ID   string
	Name string
}

// IssueComment is one comment on an Issue.
type IssueComment struct {
	ID           string
	BodyMarkdown string
	Author       UserRef
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewIssueInput is what CreateIssue passes to IssueTrackerProvider.CreateIssue
// — replaces the old (projectKey, title, description string) positional
// signature now that Jira/Linear both need issue type, assignee, priority,
// labels, and (Linear) parent-issue/team/state.
type NewIssueInput struct {
	ProjectKey       string // Jira project key; unused by Linear (TeamID instead)
	TeamID           string // Linear team id/key; unused by Jira
	StateID          string // Linear initial workflow state; unused by Jira
	Title            string
	Description      string
	IssueTypeID      string
	AssigneeID       string
	PriorityID       string
	LabelIDs         []string
	ParentIssueID    string
	CustomFieldsJSON string
}

// IssueUpdate is what UpdateIssue passes to IssueTrackerProvider.UpdateIssue.
// Every field empty/nil means "leave unchanged" — matches
// UpdateIssueRequest's proto contract.
type IssueUpdate struct {
	IssueID          string
	Title            string
	Description      string
	AssigneeID       string
	PriorityID       string
	LabelIDs         []string
	WorkflowStateID  string
	CustomFieldsJSON string
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

// ---- metadata-lookup value objects (SOL-015 scope additions) -------------

type CreateFieldOption struct {
	ID    string
	Value string
	Name  string
}

type CreateField struct {
	Key           string
	Name          string
	Required      bool
	SchemaType    string
	SchemaItems   string
	SchemaCustom  string
	AllowedValues []CreateFieldOption
}

type Transition struct {
	ID   string
	Name string
	To   WorkflowState
}

// ProjectStatusOrder is Jira's per-project Kanban column grouping — a list
// of columns, each an ordered list of status ids in that column. No Linear
// equivalent (Linear's ListWorkflowStates already returns an ordered flat
// list).
type ProjectStatusOrder struct {
	StatusIDsByColumn [][]string
}

// ---- Linear-only value objects (SOL-016) ----------------------------------

type Team struct {
	ID          string
	WorkspaceID string
	Name        string
	Key         string
}

type TeamLabel struct {
	ID    string
	Name  string
	Color string
}

type TeamMember struct {
	ID          string
	DisplayName string
	AvatarURL   string
}

type CustomView struct {
	ID          string
	WorkspaceID string
	Name        string
	Model       string // "issue" | "project"
	TeamID      string
}
