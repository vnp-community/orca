# TASK-WT-04-02: `domain.Worktree.BaseRef` + repository insert/scan/`GetWorktree`

**From Solution:** SOL-WT-04
**Priority:** P0
**Service:** `project-service`
**File:** `backend-go/services/project-service/internal/domain/worktree.go`
**Depends on:** TASK-WT-04-01
**Status:** `[x]` DONE — domain.Worktree.BaseRef + NewWorktree(...,baseRef); worktree_repository.go base_ref column + GetWorktree; ports.go WorktreeRepository.GetWorktree. go build+test clean (integration test against real Postgres passes).

---

## Context

Threads the new `base_ref` column through `project-service`'s domain type and Postgres repository, and adds the single-worktree lookup `CompareWorktrees` ([TASK-WT-04-06](./TASK-WT-04-06-usecase-compare-worktrees.md)) needs.

## Changes to make

`backend-go/services/project-service/internal/domain/worktree.go` — add `BaseRef` to the `Worktree` struct and thread it through `NewWorktree`:

```go
type Worktree struct {
	ID        string
	ProjectID string
	RepoID    string
	Path      string
	Branch    string
	Active    bool
	CreatedAt time.Time
	BaseRef   *string // NEW — the branch/tag/sha this worktree was created from; nil for worktrees created before this backfill

	ParentWorktreeID        *string
	Origin                  *string
	CaptureSource           *string
	CaptureConfidence       *string
	TaskID                  *string
	OrchestrationRunID      *string
	CoordinatorHandle       *string
	CreatedByTerminalHandle *string
}
```

```go
func NewWorktree(id, projectID, repoID, path, branch, baseRef string, lineage WorktreeLineageCapture) (Worktree, error) {
	if projectID == "" {
		return Worktree{}, ErrEmptyProjectID
	}
	if repoID == "" {
		return Worktree{}, ErrEmptyRepoID
	}
	if path == "" {
		return Worktree{}, ErrEmptyWorktreePath
	}
	if branch == "" {
		return Worktree{}, ErrEmptyWorktreeBranch
	}
	wt := Worktree{
		ID: id, ProjectID: projectID, RepoID: repoID, Path: path, Branch: branch, Active: true,
		BaseRef:                 nonEmptyPtr(baseRef),
		ParentWorktreeID:        nonEmptyPtr(lineage.ParentWorktreeID),
		Origin:                  nonEmptyPtr(lineage.Origin),
		CaptureSource:           nonEmptyPtr(lineage.CaptureSource),
		TaskID:                  nonEmptyPtr(lineage.TaskID),
		OrchestrationRunID:      nonEmptyPtr(lineage.OrchestrationRunID),
		CoordinatorHandle:       nonEmptyPtr(lineage.CoordinatorHandle),
		CreatedByTerminalHandle: nonEmptyPtr(lineage.CreatedByTerminalHandle),
	}
	if wt.ParentWorktreeID != nil || wt.Origin != nil || wt.TaskID != nil || wt.OrchestrationRunID != nil {
		explicit := "explicit"
		wt.CaptureConfidence = &explicit
	}
	return wt, nil
}
```

`backend-go/services/project-service/internal/adapter/postgres/worktree_repository.go` — add `base_ref` to `worktreeColumns`, the `INSERT`, and `scanWorktree`:

```go
const worktreeColumns = `id, project_id, repo_id, path, branch, active, created_at,
	parent_worktree_id, origin, capture_source, capture_confidence, task_id,
	orchestration_run_id, coordinator_handle, created_by_terminal_handle, base_ref`
```

Update `RecordWorktreeCreated`'s `INSERT` (columns, `VALUES`, and the arg list) and `scanWorktree`'s `Scan` call to include `wt.BaseRef`/`&wt.BaseRef` as the 15th column, matching this file's existing positional-arg convention — see [TASK-WT-01-06](./TASK-WT-01-06-project-service-outbox-event.md) if that task already changed this method's signature to add an `event domain.OutboxEvent` param; apply both changes together if so, `base_ref` as one more column in the same `INSERT`.

Add `GetWorktree` to the same file:

```go
func (r *WorktreeRepository) GetWorktree(ctx context.Context, worktreeID string) (domain.Worktree, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+worktreeColumns+` FROM project.worktrees WHERE id = $1`, worktreeID)
	out, err := scanWorktree(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Worktree{}, domain.ErrWorktreeNotFound
	}
	if err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: get worktree: %w", err)
	}
	return out, nil
}
```

Add `GetWorktree(ctx context.Context, worktreeID string) (domain.Worktree, error)` to the `WorktreeRepository` interface in `backend-go/services/project-service/internal/usecase/ports.go` (next to `ListWorktrees`, `ports.go:141`).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/project-service/...
```

Expected: build fails only on call sites not yet updated for `NewWorktree`'s new `baseRef` param and `WorktreeRepository`'s new `GetWorktree` method — fix those (including test fakes) as part of this task so the package compiles. Behavior tests land in [TASK-WT-04-07](./TASK-WT-04-07-tests-project-service.md).
