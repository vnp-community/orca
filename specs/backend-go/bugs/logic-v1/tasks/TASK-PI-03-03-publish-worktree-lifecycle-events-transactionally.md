# TASK-PI-03-03: `RecordWorktreeCreated`/`RecordWorktreeRemoved` publish outbox events in the same transaction

**From Solution:** SOL-PI-03
**Priority:** P0
**Service:** `project-service`
**File:** `backend-go/services/project-service/internal/usecase/record_worktree_created.go`, `backend-go/services/project-service/internal/usecase/record_worktree_removed.go` (new usecase — none exists today per BUG-PI-03), `backend-go/services/project-service/internal/adapter/postgres/worktree_repository.go`, `backend-go/services/project-service/internal/usecase/ports.go`
**Depends on:** TASK-PI-02-02, TASK-PI-02-06, TASK-PI-03-02
**Status:** `[x] DONE — CreateWorktreeWithEvent/RemoveWorktreeWithEvent (both-or-neither tx), record_worktree_created.go/record_worktree_removed.go publish via outbox; migrations/0010 adds linked_issue_provider/ref to project.worktrees.`

---

## Context

BUG-PI-03: `CreateWorktree` calls `RecordWorktreeCreated`/deletion RPCs
directly and neither publishes an outbox event. `git-gateway-service` owns
no database and cannot participate in the outbox pattern, so `project-service`
— already the durable writer of worktree existence — is where the outbox
row belongs, in the same Postgres transaction as the `worktrees` write
(`05-data-architecture.md:86-88`). `had_open_pr` is deliberately always
published `false` by this side: resolving it needs a live SCM call, which
must not happen inside a DB transaction — the **consumer**
(`TASK-PI-03-07`) resolves it at processing time via `GetPullRequestForBranch`.

## Changes to make

### 1. `WorktreeRepository` port — new transactional method (`ports.go`)

```go
type WorktreeRepository interface {
	// ... existing methods ...
	// CreateWorktreeWithEvent inserts worktree and event in one transaction
	// — see usage-service's SaveSession(ctx, session, event) for the
	// precedent this follows (record_usage_session.go:89-95).
	CreateWorktreeWithEvent(ctx context.Context, worktree domain.Worktree, event domain.OutboxEvent) (domain.Worktree, error)
	// RemoveWorktreeWithEvent deletes the row and enqueues worktree.deleted
	// in the same transaction.
	RemoveWorktreeWithEvent(ctx context.Context, worktreeID string, event domain.OutboxEvent) error
}
```

### 2. `worktree_repository.go` — implement both against a shared `pool.Begin`

```go
func (r *WorktreeRepository) CreateWorktreeWithEvent(ctx context.Context, wt domain.Worktree, event domain.OutboxEvent) (domain.Worktree, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after Commit

	row := tx.QueryRow(ctx, `
		INSERT INTO project.worktrees (
			id, project_id, repo_id, path, branch, active,
			parent_worktree_id, origin, capture_source, capture_confidence, task_id,
			orchestration_run_id, coordinator_handle, created_by_terminal_handle,
			linked_issue_provider, linked_issue_ref
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING `+worktreeColumns,
		wt.ID, wt.ProjectID, wt.RepoID, wt.Path, wt.Branch, wt.Active,
		wt.ParentWorktreeID, wt.Origin, wt.CaptureSource, wt.CaptureConfidence, wt.TaskID,
		wt.OrchestrationRunID, wt.CoordinatorHandle, wt.CreatedByTerminalHandle,
		wt.LinkedIssueProvider, wt.LinkedIssueRef,
	)
	out, err := scanWorktree(row)
	if err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: insert worktree: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO project.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES ($1, $2, $3, $4, 1, $5)
	`, event.ID, event.TenantID, event.Subject, event.OccurredAt, event.PayloadJSON); err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: insert outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: commit worktree+event tx: %w", err)
	}
	return out, nil
}
```

`RemoveWorktreeWithEvent` follows the identical begin/exec-delete/exec-insert/commit
shape. Extend `worktreeColumns` to include `linked_issue_provider, linked_issue_ref`
and `scanWorktree` to scan them (both new `*string` fields on
`domain.Worktree` — add via `0011` or a preceding migration alongside
`TASK-PI-02-02`'s proto fields; add the matching
`ALTER TABLE project.worktrees ADD COLUMN linked_issue_provider TEXT, ADD COLUMN linked_issue_ref TEXT;`
here if not already covered by another task in this set).

### 3. `record_worktree_created.go` — build and pass the event

```go
func (uc *RecordWorktreeCreated) Execute(ctx context.Context, in RecordWorktreeCreatedInput) (domain.Worktree, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	wt, err := domain.NewWorktree(uuid.NewString(), in.ProjectID, in.RepoID, in.Path, in.Branch, in.Lineage)
	if err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_WORKTREE_INVALID", err.Error(), err)
	}

	payload, _ := json.Marshal(worktreeLifecycleEventPayload{
		WorktreeID: wt.ID, ProjectID: in.ProjectID,
		LinkedIssueProvider: in.Lineage.LinkedIssueProvider, LinkedIssueRef: in.Lineage.LinkedIssueRef,
	})
	event := domain.OutboxEvent{
		ID: uuid.NewString(), TenantID: tenantID,
		Subject: "orca.project.worktree.created", OccurredAt: time.Now().UTC(), PayloadJSON: payload,
	}

	created, err := uc.repo.CreateWorktreeWithEvent(ctx, wt, event)
	if err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindInternal, "PROJECT_RECORD_WORKTREE_FAILED", "failed to persist worktree", err)
	}
	return created, nil
}
```

### 4. `record_worktree_removed.go` (new) — same shape, `had_open_pr` always `false`

```go
// RecordWorktreeRemoved always publishes had_open_pr=false — resolving the
// real value needs a live scm-integration-service call, which must not
// happen inside this transaction (05-data-architecture.md never sanctions
// a cross-service call from inside a DB tx). The consumer
// (issue-status-sync) resolves it at processing time instead.
func (uc *RecordWorktreeRemoved) Execute(ctx context.Context, worktreeID string) error {
	// fetch existing worktree for its linked-issue fields before delete,
	// build worktreeLifecycleEventPayload{..., HadOpenPr: false}, call
	// uc.repo.RemoveWorktreeWithEvent(ctx, worktreeID, event)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/project-service/...
go vet ./services/project-service/...
```
