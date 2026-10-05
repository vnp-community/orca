package usecase

// worktreeLifecycleEventPayload is the JSON payload shape for
// orca.project.worktree.created/orca.project.worktree.deleted — mirrors
// projectv1.WorktreeLifecycleEvent's field names (SOL-PI-03). event_id/
// tenant_id/occurred_at/schema_version live on the outer eventbus.Event
// envelope (common/eventbus.Event), not duplicated here.
type worktreeLifecycleEventPayload struct {
	WorktreeID          string `json:"worktree_id"`
	ProjectID           string `json:"project_id"`
	LinkedIssueProvider string `json:"linked_issue_provider,omitempty"`
	LinkedIssueRef      string `json:"linked_issue_ref,omitempty"`
	// LinkedIssueSite lets issue-status-sync address the right Jira site.
	LinkedIssueSite string `json:"linked_issue_site,omitempty"`
	HadOpenPr       bool   `json:"had_open_pr"`
	// ActorUserID is who created/removed the worktree, when the caller carried a
	// user. issue-status-sync needs it to act with that person's own Jira
	// credential; without it the sync is skipped rather than guessed.
	ActorUserID string `json:"actor_user_id,omitempty"`
}

const (
	subjectWorktreeCreated = "orca.project.worktree.created"
	subjectWorktreeDeleted = "orca.project.worktree.deleted"
)
