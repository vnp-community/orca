# TASK-AT-04-02: `project-service` — `WorktreeStatus` domain field + migration + filtered `ListWorktrees`

**From Solution:** SOL-AT-04
**Priority:** P0
**Service:** `project-service`
**File:** `backend-go/services/project-service/internal/domain/worktree.go`, `backend-go/services/project-service/internal/adapter/postgres/worktree_repository.go` (+ migration)
**Depends on:** TASK-AT-04-01
**Status:** `[ ]` TODO

---

## Context

`Worktree`'s domain model has only `Active bool` (`activation_state`) — no
`completed`/`error`/`stopped` concept. This task adds `Status` as a new,
additional, orthogonal field (not a replacement for `Active`) plus the
`ListWorktrees` filter BL-AT-04's `status: [...]`/`older_than` schema needs.

## Changes to make

In `worktree.go`, add:

```go
type WorktreeStatus string

const (
	WorktreeStatusActive    WorktreeStatus = "active"
	WorktreeStatusCompleted WorktreeStatus = "completed"
	WorktreeStatusError     WorktreeStatus = "error"
	WorktreeStatusStopped   WorktreeStatus = "stopped"
)
```

Add `Status WorktreeStatus` to the `Worktree` struct (default
`WorktreeStatusActive` for existing rows/new worktrees — set this default in
`NewWorktree` or equivalent constructor).

Add a migration:

```sql
ALTER TABLE project.worktrees
  ADD COLUMN status TEXT NOT NULL DEFAULT 'active'
  CHECK (status IN ('active', 'completed', 'error', 'stopped'));
```

In `worktree_repository.go`, update `ListWorktrees` (or add it if it
doesn't exist under that name — check the current method name for listing
worktrees by project) to accept optional `statusIn []string` and
`olderThan *time.Time` and apply:

```sql
WHERE ($2::text[] IS NULL OR status = ANY($2))
  AND ($3::timestamptz IS NULL OR created_at < $3)
```

Both filters optional — every existing caller (passing neither) sees
unchanged behavior.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/project-service/...
go test ./services/project-service/internal/adapter/postgres/... -run TestListWorktrees
```

Expected: `ListWorktrees` with `status_in`/`older_than` filters returns
exactly the matching subset; both unset → identical results to today's
unfiltered call (regression guard test).
