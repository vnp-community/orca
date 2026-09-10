// Package usecase holds issue-status-sync's application service and the
// ports it needs — defined here, implemented in internal/adapter/*, per
// the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import "context"

// IssueTrackerClient wraps issue-tracking-service's UpdateIssue RPC
// (workflow_state_id is that RPC's transition-target field — see its proto
// doc comment) for the Jira/Linear half of updateIssueStatus. tenantID is
// explicit (not read from ctx via common/tenant.RequireTenantID) because
// this service is an async event consumer, not an inbound gRPC handler —
// there is no validated caller identity to forward, only the tenant_id
// carried on the event itself (event.TenantID).
type IssueTrackerClient interface {
	TransitionIssue(ctx context.Context, tenantID, provider, ref, state string) error
}

// ScmClient wraps scm-integration-service for the GitHub half of
// updateIssueStatus (label-based status, no native workflow states) plus
// had_open_pr resolution. See IssueTrackerClient's doc comment for why
// tenantID is explicit here too.
type ScmClient interface {
	UpdateIssue(ctx context.Context, tenantID, provider, ref, labelPatch string) error
	// GetPullRequestForBranch resolves had_open_pr at processing time — see
	// record_worktree_removed.go's doc comment (project-service,
	// TASK-PI-03-03) for why the publisher never resolves this itself.
	//
	// KNOWN GAP: WorktreeLifecycleEvent carries no branch/repo, so this
	// method has no caller yet in sync_issue_status.go — HandleWorktreeLifecycle
	// uses the event's own (always-false-today) HadOpenPR field directly.
	// Kept on this port so a future event-schema extension (branch/repo
	// fields) has a real implementation ready to call, rather than
	// inventing the RPC shape later under time pressure.
	GetPullRequestForBranch(ctx context.Context, tenantID, provider, repo, branch string) (found bool, err error)
}

// ProjectSettingsClient wraps project-service's GetProject RPC for the
// BR-PI-07 belt-and-braces re-check: the publisher already gates recording
// the link when sync is off, but a project's flag can flip off AFTER the
// link was recorded and BEFORE this event is processed.
type ProjectSettingsClient interface {
	IsIssueStatusSyncEnabled(ctx context.Context, tenantID, projectID string) (bool, error)
}

// ProcessedEventStore is the dedup cache backing BR-PI-09's
// at-least-once-but-idempotent consumption — issuestatussync.processed_events.
type ProcessedEventStore interface {
	Seen(ctx context.Context, eventID string) (bool, error)
	MarkSeen(ctx context.Context, eventID string) error
}
