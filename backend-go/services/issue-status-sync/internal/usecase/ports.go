// Package usecase holds issue-status-sync's application service and the
// ports it needs — defined here, implemented in internal/adapter/*, per
// the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"
	"time"
)

// IssueTrackerClient wraps issue-tracking-service's UpdateIssue RPC
// (workflow_state_id is that RPC's transition-target field — see its proto
// doc comment) for the Jira/Linear half of updateIssueStatus. tenantID is
// explicit (not read from ctx via common/tenant.RequireTenantID) because
// this service is an async event consumer, not an inbound gRPC handler —
// there is no validated caller identity to forward, only the tenant_id
// carried on the event itself (event.TenantID).
//
// userID is the person whose credential performs the call: the tracker
// connection is stored per (tenant, user), and the event is all this service has.
type IssueTrackerClient interface {
	// site is the tracker workspace (Jira site) the ref belongs to; "" means the connection default.
	TransitionIssue(ctx context.Context, tenantID, userID, provider, ref, site, state string) error
	// IssueStatusCategory returns the issue's current status category ("todo",
	// "in_progress", "done"), or "" when the provider does not report one.
	IssueStatusCategory(ctx context.Context, tenantID, userID, provider, ref, site string) (string, error)
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

// RequestSyncStateStore keeps the last Request status version pushed to the
// tracker (issuestatussync.request_sync_state). Advance is atomic: applied is
// true only for the single caller whose version is strictly newer than the stored one.
type RequestSyncStateStore interface {
	Advance(ctx context.Context, tenantID, requestID string, version int64, target string) (applied bool, err error)
}

// RequestLookupClient asks request-service whether an open Request owns a
// tracker issue, in which case worktree/PR sync must stay out of the way.
// site "" matches any site. The gRPC implementation waits on the
// LookupRequestBySource RPC (TASK-REQ-024-05); until then main wires nil.
type RequestLookupClient interface {
	Lookup(ctx context.Context, tenantID, provider, site, ref string) (found bool, err error)
}

// IssueCommenter posts a short comment on a tracker issue (issue-tracking-service AddIssueComment).
type IssueCommenter interface {
	AddComment(ctx context.Context, tenantID, userID, provider, ref, site, bodyMarkdown string) error
}

// SyncObserver receives metric observations. The default is a no-op.
type SyncObserver interface {
	// ObserveRequestEvent counts a handled event; result is one of applied,
	// skipped_not_jira, skipped_flag_off, skipped_stale, skipped_no_actor,
	// skipped_category, skipped_transition_unavailable, failed.
	ObserveRequestEvent(event, result string)
	// ObserveRequestOwnedSkip counts worktree/PR events skipped because a Request owns the issue (source: worktree|pr).
	ObserveRequestOwnedSkip(source string)
	ObserveJiraTransition(elapsed time.Duration)
}

type noopObserver struct{}

func (noopObserver) ObserveRequestEvent(string, string)  {}
func (noopObserver) ObserveRequestOwnedSkip(string)      {}
func (noopObserver) ObserveJiraTransition(time.Duration) {}
