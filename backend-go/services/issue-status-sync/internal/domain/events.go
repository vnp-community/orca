// Package domain holds issue-status-sync's entities and value objects. Per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md, this
// package has zero imports outside stdlib — no database, no gRPC, no
// NATS-specific type.
package domain

// WorktreeLifecycleEvent mirrors projectv1.WorktreeLifecycleEvent's JSON
// payload shape (published to orca.project.worktree.created/
// orca.project.worktree.deleted) — see project-service's
// internal/usecase/lifecycle_events.go for the publisher side.
//
// Deleted is NOT part of the wire payload — the payload itself is
// subject-agnostic (worktree_id/project_id/linked_issue_*/had_open_pr
// only), so the eventbus subscriber (adapter/eventbus/subscriber.go)
// stamps this field from which of the two subjects the message actually
// arrived on before handing it to SyncIssueStatus.
type WorktreeLifecycleEvent struct {
	EventID             string
	TenantID            string
	OccurredAt          string
	SchemaVersion       int32
	WorktreeID          string
	ProjectID           string
	LinkedIssueProvider string
	LinkedIssueRef      string
	HadOpenPR           bool
	Deleted             bool
}

// PullRequestLifecycleEvent mirrors scmintegrationv1.PullRequestLifecycleEvent's
// JSON payload shape (published to orca.scm.pull_request.created/
// orca.scm.pull_request.merged). Merged is stamped by the subscriber, same
// reasoning as WorktreeLifecycleEvent.Deleted above.
type PullRequestLifecycleEvent struct {
	EventID             string
	TenantID            string
	OccurredAt          string
	SchemaVersion       int32
	Provider            string
	Repo                string
	PRNumber            int32
	LinkedIssueProvider string
	LinkedIssueRef      string
	Merged              bool
}

// TargetState is BL-PI-03's mapping-table output: what to write to the
// linked issue-tracker/SCM once a worktree/PR lifecycle event resolves to
// a status change.
type TargetState struct {
	TrackerState     string // Jira/Linear transition name
	GitHubLabelPatch string // add/remove label, or "close" for Done
}
