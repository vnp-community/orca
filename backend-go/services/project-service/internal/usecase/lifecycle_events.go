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
	HadOpenPr           bool   `json:"had_open_pr"`
}

const (
	subjectWorktreeCreated = "orca.project.worktree.created"
	subjectWorktreeDeleted = "orca.project.worktree.deleted"
)
