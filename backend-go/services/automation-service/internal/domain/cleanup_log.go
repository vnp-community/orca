package domain

// CleanupLogEntry is one worktree's outcome in a
// workflow-service.CleanupWorktreesStepExecutor run — BR-AT-14's
// per-worktree, per-reason audit trail, persisted via WriteCleanupReport.
type CleanupLogEntry struct {
	WorktreeID string
	Action     string // "deleted" | "skipped" | "would_delete"
	Reason     string
}
