// Package postgres implements orchestration-service's repository ports
// (defined in internal/usecase) against this service's own PostgreSQL
// database — see specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule: this is the ONLY package in
// orchestration-service that knows SQL exists.
//
// UpdateStatusAndPromote, Create (gate) and Resolve (gate) are each a
// single Postgres transaction — the hard NFR from
// specs/backend-go/services/orchestration-service.md §8: a torn read
// between marking a task complete and re-scanning its dependents can
// double-dispatch a task or leave a ready task stuck pending.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/usecase"
)

// Repository implements usecase.OrchestrationTaskRepository,
// usecase.DispatchContextRepository, and usecase.GateRepository against
// Postgres via pgx — hand-written SQL (see architecture/04-tech-stack.md:
// sqlc codegen is the eventual target; this scaffold hand-writes the
// equivalent queries directly, matching usage-service's pilot precedent).
type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// ---- OrchestrationTaskRepository ---------------------------------------

func (r *Repository) Create(ctx context.Context, task domain.OrchestrationTask) (domain.OrchestrationTask, error) {
	id := task.ID
	if id == "" {
		id = uuid.NewString()
	}
	spec := task.Spec
	if spec == nil {
		spec = json.RawMessage(`{}`)
	}
	depsJSON, err := json.Marshal(task.Deps)
	if err != nil {
		return domain.OrchestrationTask{}, fmt.Errorf("postgres: marshal deps: %w", err)
	}

	// parent_id is passed as *string (nil for "no parent"), resolved in Go
	// rather than via SQL's NULLIF($n,'') — same pre-existing uuid/text bind
	// type-inference bug CreateDispatchContext's own doc comment already
	// documents and fixes for orchestration_task_id (NULLIF's result type
	// comes from its arguments, both text here, so Postgres never gets a
	// chance to coerce to parent_id's uuid column type). origin_task_id is
	// a TEXT column, so it keeps using NULLIF directly.
	var parentIDArg *string
	if task.ParentID != "" {
		parentIDArg = &task.ParentID
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO orchestration.orchestration_tasks (
			id, tenant_id, coordinator_run_id, parent_id, origin_task_id, task_title, spec, status, deps
		) VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,'pending',$8)
		RETURNING created_at
	`, id, task.TenantID, task.CoordinatorRunID, parentIDArg, task.OriginTaskID, task.TaskTitle, spec, depsJSON)

	var createdAt time.Time
	if err := row.Scan(&createdAt); err != nil {
		return domain.OrchestrationTask{}, fmt.Errorf("postgres: insert orchestration task: %w", err)
	}

	task.ID = id
	task.Status = domain.TaskStatusPending
	task.Spec = spec
	task.CreatedAt = createdAt
	return task, nil
}

func (r *Repository) Get(ctx context.Context, tenantID, id string) (domain.OrchestrationTask, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, coordinator_run_id, COALESCE(parent_id::text, ''), COALESCE(origin_task_id, ''),
		       task_title, spec, status, deps, result, created_at, completed_at
		FROM orchestration.orchestration_tasks
		WHERE id = $1 AND tenant_id = $2
	`, id, tenantID)
	task, err := scanTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OrchestrationTask{}, usecase.ErrTaskNotFound
	}
	if err != nil {
		return domain.OrchestrationTask{}, fmt.Errorf("postgres: query orchestration task: %w", err)
	}
	return task, nil
}

// UpdateStatusAndPromote is the atomic promote saga (§8): BEGIN, update the
// task's status, scan pending siblings in the same coordinator_run_id
// whose deps are now all completed, promote them to ready, detect whether
// this write finalized the owning coordinator_run, COMMIT. All in one
// transaction — a crash or error partway through rolls back the whole
// thing rather than leaving a half-applied state.
func (r *Repository) UpdateStatusAndPromote(ctx context.Context, tenantID, taskID string, newStatus domain.TaskStatus, event domain.OutboxEvent) (usecase.UpdateStatusAndPromoteResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return usecase.UpdateStatusAndPromoteResult{}, fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var completedAt *time.Time
	if newStatus == domain.TaskStatusCompleted || newStatus == domain.TaskStatusFailed {
		now := time.Now().UTC()
		completedAt = &now
	}

	row := tx.QueryRow(ctx, `
		UPDATE orchestration.orchestration_tasks
		SET status = $1, completed_at = COALESCE($2, completed_at)
		WHERE id = $3 AND tenant_id = $4
		RETURNING id, tenant_id, coordinator_run_id, COALESCE(parent_id::text, ''), COALESCE(origin_task_id, ''),
		          task_title, spec, status, deps, result, created_at, completed_at
	`, string(newStatus), completedAt, taskID, tenantID)
	task, err := scanTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return usecase.UpdateStatusAndPromoteResult{}, usecase.ErrTaskNotFound
	}
	if err != nil {
		return usecase.UpdateStatusAndPromoteResult{}, fmt.Errorf("postgres: update task status: %w", err)
	}

	var promotedIDs []string
	if newStatus == domain.TaskStatusCompleted {
		promotedIDs, err = promoteReadySiblings(ctx, tx, tenantID, task.CoordinatorRunID)
		if err != nil {
			return usecase.UpdateStatusAndPromoteResult{}, err
		}
	}

	var finalized *usecase.RunFinalization
	// Only a completed/failed leaf transition can possibly finish a run —
	// skip the extra query on every other status write (ready/dispatched/
	// blocked transitions can never be the LAST event of a run).
	if newStatus == domain.TaskStatusCompleted || newStatus == domain.TaskStatusFailed {
		var nonTerminal int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM orchestration.orchestration_tasks
			WHERE coordinator_run_id = $1 AND tenant_id = $2
			  AND status NOT IN ('completed', 'failed')`,
			task.CoordinatorRunID, tenantID).Scan(&nonTerminal); err != nil {
			return usecase.UpdateStatusAndPromoteResult{}, fmt.Errorf("postgres: count non-terminal: %w", err)
		}
		if nonTerminal == 0 {
			// Every sibling in this run is now terminal. A single failed
			// leaf fails the WHOLE run (fail-closed: a partially-succeeded
			// multi-agent DAG is not a usable result for task-service's
			// ReportTaskExecutionResult, which only accepts success/
			// failure, not partial).
			var anyFailed bool
			if err := tx.QueryRow(ctx, `
				SELECT exists(SELECT 1 FROM orchestration.orchestration_tasks
					WHERE coordinator_run_id = $1 AND tenant_id = $2 AND status = 'failed')`,
				task.CoordinatorRunID, tenantID).Scan(&anyFailed); err != nil {
				return usecase.UpdateStatusAndPromoteResult{}, fmt.Errorf("postgres: check any-failed: %w", err)
			}
			runStatus := "completed"
			if anyFailed {
				runStatus = "failed"
			}
			var originTaskID string
			err := tx.QueryRow(ctx, `
				UPDATE orchestration.coordinator_runs SET status = $1, completed_at = now()
				WHERE id = $2 AND tenant_id = $3 AND status = 'running'
				RETURNING origin_task_id`,
				runStatus, task.CoordinatorRunID, tenantID).Scan(&originTaskID)
			switch {
			case errors.Is(err, pgx.ErrNoRows):
				// The run was already completed/failed by a racing
				// concurrent call (or is not 'running' for some other
				// reason) — do not double-finalize, do not error the
				// whole transaction, just skip setting `finalized`.
			case err != nil:
				return usecase.UpdateStatusAndPromoteResult{}, fmt.Errorf("postgres: finalize run: %w", err)
			default:
				finalized = &usecase.RunFinalization{
					CoordinatorRunID: task.CoordinatorRunID,
					OriginTaskID:     originTaskID,
					Success:          !anyFailed,
				}
			}
			// Reporting to task-service happens OUTSIDE this transaction
			// (see the usecase layer) — a cross-service gRPC call must
			// never hold a DB transaction open.
		}
	}

	// Outbox enqueue (BE-SOL-003/TASK-FT-003-01) — same transaction as the
	// status write above, so the event can never be observed without the
	// status change it reports on having durably committed too.
	if event.ID != "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO orchestration.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES ($1, $2, $3, $4, 1, $5::jsonb)
		`, event.ID, tenantID, event.Subject, event.OccurredAt, event.PayloadJSON); err != nil {
			return usecase.UpdateStatusAndPromoteResult{}, fmt.Errorf("postgres: insert outbox event: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return usecase.UpdateStatusAndPromoteResult{}, fmt.Errorf("postgres: commit tx: %w", err)
	}
	return usecase.UpdateStatusAndPromoteResult{Task: task, PromotedIDs: promotedIDs, RunFinalized: finalized}, nil
}

// promoteReadySiblings scans every pending task in coordinatorRunID,
// applies domain.OrchestrationTask.DepsSatisfied against the set of
// completed ids in that run, and flips any satisfied task to ready — the
// same pure rule internal/domain exposes, applied here inside the
// transaction so the SQL-level and unit-testable domain-level definitions
// of "is this task ready" can never drift apart.
func promoteReadySiblings(ctx context.Context, tx pgx.Tx, tenantID, coordinatorRunID string) ([]string, error) {
	completedRows, err := tx.Query(ctx, `
		SELECT id FROM orchestration.orchestration_tasks
		WHERE coordinator_run_id = $1 AND tenant_id = $2 AND status = 'completed'
	`, coordinatorRunID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query completed siblings: %w", err)
	}
	completed := map[string]struct{}{}
	for completedRows.Next() {
		var id string
		if err := completedRows.Scan(&id); err != nil {
			completedRows.Close()
			return nil, fmt.Errorf("postgres: scan completed sibling id: %w", err)
		}
		completed[id] = struct{}{}
	}
	completedRows.Close()
	if err := completedRows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate completed siblings: %w", err)
	}

	pendingRows, err := tx.Query(ctx, `
		SELECT id, deps FROM orchestration.orchestration_tasks
		WHERE coordinator_run_id = $1 AND tenant_id = $2 AND status = 'pending'
	`, coordinatorRunID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query pending siblings: %w", err)
	}
	type candidate struct {
		id   string
		deps []string
	}
	var candidates []candidate
	for pendingRows.Next() {
		var c candidate
		var depsJSON []byte
		if err := pendingRows.Scan(&c.id, &depsJSON); err != nil {
			pendingRows.Close()
			return nil, fmt.Errorf("postgres: scan pending sibling: %w", err)
		}
		if err := json.Unmarshal(depsJSON, &c.deps); err != nil {
			pendingRows.Close()
			return nil, fmt.Errorf("postgres: unmarshal deps: %w", err)
		}
		candidates = append(candidates, c)
	}
	pendingRows.Close()
	if err := pendingRows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate pending siblings: %w", err)
	}

	var promoted []string
	for _, c := range candidates {
		task := domain.OrchestrationTask{Deps: c.deps}
		if task.DepsSatisfied(completed) {
			promoted = append(promoted, c.id)
		}
	}
	if len(promoted) == 0 {
		return nil, nil
	}

	if _, err := tx.Exec(ctx, `
		UPDATE orchestration.orchestration_tasks SET status = 'ready'
		WHERE id = ANY($1) AND tenant_id = $2
	`, promoted, tenantID); err != nil {
		return nil, fmt.Errorf("postgres: promote ready siblings: %w", err)
	}
	return promoted, nil
}

func scanTask(row pgx.Row) (domain.OrchestrationTask, error) {
	var task domain.OrchestrationTask
	var status string
	var specJSON, depsJSON, resultJSON []byte
	var completedAt *time.Time
	if err := row.Scan(
		&task.ID, &task.TenantID, &task.CoordinatorRunID, &task.ParentID, &task.OriginTaskID,
		&task.TaskTitle, &specJSON, &status, &depsJSON, &resultJSON, &task.CreatedAt, &completedAt,
	); err != nil {
		return domain.OrchestrationTask{}, err
	}
	task.Status = domain.TaskStatus(status)
	task.Spec = specJSON
	task.Result = resultJSON
	if completedAt != nil {
		task.CompletedAt = *completedAt
	}
	if len(depsJSON) > 0 {
		if err := json.Unmarshal(depsJSON, &task.Deps); err != nil {
			return domain.OrchestrationTask{}, fmt.Errorf("postgres: unmarshal deps: %w", err)
		}
	}
	return task, nil
}

// ---- DispatchContextRepository ------------------------------------------

// Create inserts a dispatch_context row. See usecase.DispatchContextRepository's
// doc comment (Epic C, docs/execution-plan.md): orchestrationTaskID is
// persisted when the caller supplies one (NULLIF collapses "" to NULL for
// the nullable FK), and left NULL for an ad-hoc coordinator-only dispatch —
// a single INSERT, trivially atomic on its own.
func (r *Repository) CreateDispatchContext(ctx context.Context, tenantID, userID, worktreeID, handle, coordinatorRunID, orchestrationTaskID string, event domain.OutboxEvent) (domain.DispatchContext, error) {
	id := uuid.NewString()
	// orchestration_task_id is passed as *string (nil for "no task"),
	// resolved in Go rather than via SQL's NULLIF($n,'') — pgx sends a Go
	// string parameter as `text`, and NULLIF(text, '') against a UUID
	// column fails type inference at bind time ("column ... is of type
	// uuid but expression is of type text", a real, pre-existing bug this
	// fix closes: NULLIF's result type comes from its arguments, which are
	// both text here, so Postgres never gets a chance to coerce to the
	// column's uuid type the way a bare parameter reference would).
	// user_id/worktree_id are plain TEXT columns (no such coercion issue),
	// so they keep using NULLIF directly.
	var orchestrationTaskIDArg *string
	if orchestrationTaskID != "" {
		orchestrationTaskIDArg = &orchestrationTaskID
	}

	// Wrapped in a transaction (TASK-FT-003-01) — this method had none
	// before, a single INSERT was trivially atomic on its own, but the
	// outbox row must now commit atomically with the dispatch-context row.
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DispatchContext{}, fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		INSERT INTO orchestration.dispatch_contexts (id, tenant_id, user_id, worktree_id, handle, coordinator_run_id, orchestration_task_id, status)
		VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6,$7,'pending')
		RETURNING created_at
	`, id, tenantID, userID, worktreeID, handle, coordinatorRunID, orchestrationTaskIDArg)

	var createdAt time.Time
	if err := row.Scan(&createdAt); err != nil {
		return domain.DispatchContext{}, fmt.Errorf("postgres: insert dispatch context: %w", err)
	}

	if event.ID != "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO orchestration.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES ($1, $2, $3, $4, 1, $5::jsonb)
		`, event.ID, tenantID, event.Subject, event.OccurredAt, event.PayloadJSON); err != nil {
			return domain.DispatchContext{}, fmt.Errorf("postgres: insert outbox event: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.DispatchContext{}, fmt.Errorf("postgres: commit tx: %w", err)
	}

	return domain.DispatchContext{
		ID:                  id,
		TenantID:            tenantID,
		UserID:              userID,
		WorktreeID:          worktreeID,
		Handle:              handle,
		CoordinatorRunID:    coordinatorRunID,
		OrchestrationTaskID: orchestrationTaskID,
		Status:              domain.DispatchStatusPending,
		CreatedAt:           createdAt,
	}, nil
}

// ListActiveDispatchContextsForUser returns every non-terminal dispatch
// context for (tenantID, userID) — see usecase.DispatchContextRepository's
// doc comment. "Active" excludes completed/failed/circuit_broken; a
// circuit_broken dispatch is done trying, not still running, so it's
// excluded the same as failed/completed for this "what's currently running
// for me" view.
func (r *Repository) ListActiveDispatchContextsForUser(ctx context.Context, tenantID, userID string) ([]domain.DispatchContext, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, user_id, worktree_id, handle, coordinator_run_id, orchestration_task_id,
		       status, failure_count, last_failure, dispatched_at, completed_at,
		       last_heartbeat_at, created_at
		FROM orchestration.dispatch_contexts
		WHERE tenant_id = $1 AND user_id = $2
		  AND status NOT IN ('completed', 'failed', 'circuit_broken')
		ORDER BY created_at DESC
	`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query active dispatch contexts for user: %w", err)
	}
	defer rows.Close()

	out := []domain.DispatchContext{}
	for rows.Next() {
		var d domain.DispatchContext
		var worktreeID, orchestrationTaskID, lastFailure *string
		var dispatchedAt, completedAt, lastHeartbeatAt *time.Time
		if err := rows.Scan(
			&d.ID, &d.TenantID, &d.UserID, &worktreeID, &d.Handle, &d.CoordinatorRunID, &orchestrationTaskID,
			&d.Status, &d.FailureCount, &lastFailure, &dispatchedAt, &completedAt,
			&lastHeartbeatAt, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres: scan active dispatch context row: %w", err)
		}
		if worktreeID != nil {
			d.WorktreeID = *worktreeID
		}
		if orchestrationTaskID != nil {
			d.OrchestrationTaskID = *orchestrationTaskID
		}
		if lastFailure != nil {
			d.LastFailure = *lastFailure
		}
		if dispatchedAt != nil {
			d.DispatchedAt = *dispatchedAt
		}
		if completedAt != nil {
			d.CompletedAt = *completedAt
		}
		if lastHeartbeatAt != nil {
			d.LastHeartbeatAt = *lastHeartbeatAt
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate active dispatch context rows: %w", err)
	}
	return out, nil
}

// GetLatestForTask returns the most recently created dispatch_contexts row
// for orchestrationTaskID — see usecase.DispatchContextRepository's doc
// comment: a task's dispatch_contexts row is not unique (retries after
// failure create new rows), so this orders by created_at DESC and takes
// the first, matching CreateDispatchContext's own column set and
// nullable-column handling.
func (r *Repository) GetLatestForTask(ctx context.Context, tenantID, orchestrationTaskID string) (domain.DispatchContext, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, worktree_id, orchestration_task_id, handle, coordinator_run_id, status, created_at
		FROM orchestration.dispatch_contexts
		WHERE tenant_id = $1 AND orchestration_task_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, tenantID, orchestrationTaskID)

	var dc domain.DispatchContext
	var status string
	var worktreeIDCol, orchestrationTaskIDCol *string
	if err := row.Scan(&dc.ID, &dc.TenantID, &worktreeIDCol, &orchestrationTaskIDCol, &dc.Handle, &dc.CoordinatorRunID, &status, &dc.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DispatchContext{}, usecase.ErrDispatchContextNotFound
		}
		return domain.DispatchContext{}, fmt.Errorf("postgres: get latest dispatch context for task: %w", err)
	}
	if worktreeIDCol != nil {
		dc.WorktreeID = *worktreeIDCol
	}
	if orchestrationTaskIDCol != nil {
		dc.OrchestrationTaskID = *orchestrationTaskIDCol
	}
	dc.Status = domain.DispatchStatus(status)
	return dc, nil
}

// GetDispatchContext returns one dispatch_contexts row by id — see
// usecase.DispatchContextRepository's doc comment (BE-SOL-003/TASK-FT-003-02).
func (r *Repository) GetDispatchContext(ctx context.Context, tenantID, id string) (domain.DispatchContext, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, worktree_id, orchestration_task_id, handle, coordinator_run_id, status, created_at
		FROM orchestration.dispatch_contexts
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	var dc domain.DispatchContext
	var status string
	var worktreeIDCol, orchestrationTaskIDCol *string
	if err := row.Scan(&dc.ID, &dc.TenantID, &worktreeIDCol, &orchestrationTaskIDCol, &dc.Handle, &dc.CoordinatorRunID, &status, &dc.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DispatchContext{}, usecase.ErrDispatchContextNotFound
		}
		return domain.DispatchContext{}, fmt.Errorf("postgres: get dispatch context: %w", err)
	}
	if worktreeIDCol != nil {
		dc.WorktreeID = *worktreeIDCol
	}
	if orchestrationTaskIDCol != nil {
		dc.OrchestrationTaskID = *orchestrationTaskIDCol
	}
	dc.Status = domain.DispatchStatus(status)
	return dc, nil
}

// RecordDispatchFailure loads dispatch_contexts row id (locked, tenant-
// scoped), applies domain.DispatchContext.RecordFailure(reason) in Go, and
// persists the updated failure_count/status/last_failure — same
// SELECT...FOR UPDATE-then-write transaction shape as CreateGate below,
// applied to a single row instead of a cross-table update. See
// usecase.DispatchContextRepository's doc comment.
func (r *Repository) RecordDispatchFailure(ctx context.Context, tenantID, dispatchContextID, reason string) (domain.DispatchContext, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DispatchContext{}, fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var dc domain.DispatchContext
	var status string
	var orchestrationTaskID, userID *string
	err = tx.QueryRow(ctx, `
		SELECT id, tenant_id, user_id, handle, coordinator_run_id, orchestration_task_id, status, failure_count
		FROM orchestration.dispatch_contexts
		WHERE id = $1 AND tenant_id = $2
		FOR UPDATE
	`, dispatchContextID, tenantID).Scan(
		&dc.ID, &dc.TenantID, &userID, &dc.Handle, &dc.CoordinatorRunID, &orchestrationTaskID, &status, &dc.FailureCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DispatchContext{}, usecase.ErrDispatchContextNotFound
	}
	if err != nil {
		return domain.DispatchContext{}, fmt.Errorf("postgres: query dispatch context for failure record: %w", err)
	}
	if userID != nil {
		dc.UserID = *userID
	}
	if orchestrationTaskID != nil {
		dc.OrchestrationTaskID = *orchestrationTaskID
	}
	dc.Status = domain.DispatchStatus(status)

	updated := dc.RecordFailure(reason)

	if _, err := tx.Exec(ctx, `
		UPDATE orchestration.dispatch_contexts
		SET failure_count = $1, status = $2, last_failure = $3
		WHERE id = $4 AND tenant_id = $5
	`, updated.FailureCount, string(updated.Status), updated.LastFailure, dispatchContextID, tenantID); err != nil {
		return domain.DispatchContext{}, fmt.Errorf("postgres: update dispatch context failure: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.DispatchContext{}, fmt.Errorf("postgres: commit dispatch failure tx: %w", err)
	}
	return updated, nil
}

// ---- GateRepository -------------------------------------------------

// CreateGate atomically resolves dispatchContextID to its owning
// orchestration_task_id, inserts the gate row, and transitions that task to
// blocked — all in one transaction (§8). Returns
// usecase.ErrDispatchContextHasNoTask if the dispatch context has no
// orchestration_task_id yet (see the CreateDispatchContext doc comment
// above and README "Known gaps": that is the expected state for every
// dispatch context created through the current proto surface, until it is
// extended).
func (r *Repository) CreateGate(ctx context.Context, tenantID, dispatchContextID, question string, options []string, event domain.OutboxEvent) (domain.DecisionGate, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DecisionGate{}, fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var orchestrationTaskID *string
	var toHandle string
	err = tx.QueryRow(ctx, `
		SELECT orchestration_task_id, handle FROM orchestration.dispatch_contexts
		WHERE id = $1 AND tenant_id = $2
		FOR UPDATE
	`, dispatchContextID, tenantID).Scan(&orchestrationTaskID, &toHandle)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DecisionGate{}, usecase.ErrDispatchContextNotFound
	}
	if err != nil {
		return domain.DecisionGate{}, fmt.Errorf("postgres: query dispatch context: %w", err)
	}
	if orchestrationTaskID == nil || *orchestrationTaskID == "" {
		return domain.DecisionGate{}, usecase.ErrDispatchContextHasNoTask
	}

	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return domain.DecisionGate{}, fmt.Errorf("postgres: marshal options: %w", err)
	}

	id := uuid.NewString()
	var createdAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO orchestration.decision_gates (
			id, tenant_id, orchestration_task_id, dispatch_context_id, question, options, status
		) VALUES ($1,$2,$3,$4,$5,$6,'pending')
		RETURNING created_at
	`, id, tenantID, *orchestrationTaskID, dispatchContextID, question, optionsJSON).Scan(&createdAt)
	if err != nil {
		return domain.DecisionGate{}, fmt.Errorf("postgres: insert decision gate: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE orchestration.orchestration_tasks SET status = 'blocked'
		WHERE id = $1 AND tenant_id = $2
	`, *orchestrationTaskID, tenantID); err != nil {
		return domain.DecisionGate{}, fmt.Errorf("postgres: block owning task: %w", err)
	}

	// orchestration.messages' first real write (BE-SOL-003/TASK-FT-003-02) —
	// a minimal mailbox entry, not a general PostMessage usecase; see that
	// task's Context. payload carries the JSON-encoded options list so a
	// mailbox reader has the same choices the gate itself does.
	if _, err := tx.Exec(ctx, `
		INSERT INTO orchestration.messages (tenant_id, from_handle, to_handle, subject, body, type, payload)
		VALUES ($1, 'coordinator', $2, 'Decision needed', $3, 'decision_gate', $4::jsonb)
	`, tenantID, toHandle, question, optionsJSON); err != nil {
		return domain.DecisionGate{}, fmt.Errorf("postgres: insert coordinator message: %w", err)
	}

	if event.ID != "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO orchestration.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES ($1, $2, $3, $4, 1, $5::jsonb)
		`, event.ID, tenantID, event.Subject, event.OccurredAt, event.PayloadJSON); err != nil {
			return domain.DecisionGate{}, fmt.Errorf("postgres: insert outbox event: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.DecisionGate{}, fmt.Errorf("postgres: commit tx: %w", err)
	}

	return domain.DecisionGate{
		ID:                  id,
		TenantID:            tenantID,
		OrchestrationTaskID: *orchestrationTaskID,
		DispatchContextID:   dispatchContextID,
		Question:            question,
		Options:             options,
		Status:              domain.GateStatusPending,
		CreatedAt:           createdAt,
	}, nil
}

// ResolveGate atomically transitions the gate to resolved and unblocks its
// owning task — all in one transaction (§8). The row is locked
// (SELECT ... FOR UPDATE) before the pending check so two concurrent
// ResolveGate calls for the same gate can never both observe "pending" —
// enforcing domain.ErrGateAlreadyResolved's invariant even under a race
// (defense in depth alongside the usecase-level HandleSerializer keying).
func (r *Repository) ResolveGate(ctx context.Context, tenantID, gateID, resolution string) (domain.DecisionGate, []string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DecisionGate{}, nil, fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var gate domain.DecisionGate
	var status string
	var dispatchContextID *string
	var fromHandle *string
	var optionsJSON []byte
	var existingResolution *string
	var resolvedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT g.id, g.tenant_id, g.orchestration_task_id, g.dispatch_context_id, g.question, g.options,
		       g.status, g.resolution, g.created_at, g.resolved_at, dc.handle
		FROM orchestration.decision_gates g
		LEFT JOIN orchestration.dispatch_contexts dc ON dc.id = g.dispatch_context_id
		WHERE g.id = $1 AND g.tenant_id = $2
		FOR UPDATE OF g
	`, gateID, tenantID).Scan(
		&gate.ID, &gate.TenantID, &gate.OrchestrationTaskID, &dispatchContextID, &gate.Question, &optionsJSON,
		&status, &existingResolution, &gate.CreatedAt, &resolvedAt, &fromHandle,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DecisionGate{}, nil, usecase.ErrGateNotFound
	}
	if err != nil {
		return domain.DecisionGate{}, nil, fmt.Errorf("postgres: query decision gate: %w", err)
	}
	// resolution is NULL until a gate is resolved — a still-pending gate
	// (the only state this function is ever called against, per its own
	// name) would otherwise fail to scan into a non-pointer string.
	if existingResolution != nil {
		gate.Resolution = *existingResolution
	}
	if status != string(domain.GateStatusPending) {
		return domain.DecisionGate{}, nil, usecase.ErrGateNotPending
	}
	if dispatchContextID != nil {
		gate.DispatchContextID = *dispatchContextID
	}
	if len(optionsJSON) > 0 {
		if err := json.Unmarshal(optionsJSON, &gate.Options); err != nil {
			return domain.DecisionGate{}, nil, fmt.Errorf("postgres: unmarshal gate options: %w", err)
		}
	}

	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `
		UPDATE orchestration.decision_gates SET status = 'resolved', resolution = $1, resolved_at = $2
		WHERE id = $3
	`, resolution, now, gateID); err != nil {
		return domain.DecisionGate{}, nil, fmt.Errorf("postgres: resolve decision gate: %w", err)
	}

	// Resolving a gate unblocks the task it was gating; that task moves
	// straight to ready rather than pending because a blocked task's deps
	// (if any) were already satisfied at the point it became blocked — a
	// gate blocks dispatch, not dependency completion. There is no further
	// "promotion pass" over siblings here (unlike UpdateStatusAndPromote):
	// promotion is triggered by a task reaching *completed*, and resolving
	// a gate doesn't complete anything.
	if _, err := tx.Exec(ctx, `
		UPDATE orchestration.orchestration_tasks SET status = 'ready'
		WHERE id = $1 AND tenant_id = $2 AND status = 'blocked'
	`, gate.OrchestrationTaskID, tenantID); err != nil {
		return domain.DecisionGate{}, nil, fmt.Errorf("postgres: unblock owning task: %w", err)
	}

	// A minimal orchestration.messages row for the resolution, mirroring
	// CreateGate's own first-write (TASK-FT-003-02) — direction reversed
	// (the resolving handle replying to the coordinator's mailbox) since
	// this is a reply to the question CreateGate posted. No outbox event
	// here: the CR names no `.resolved` subject, only `.opened` — see
	// GateRepository.ResolveGate's doc comment.
	replyFromHandle := "coordinator"
	if fromHandle != nil && *fromHandle != "" {
		replyFromHandle = *fromHandle
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO orchestration.messages (tenant_id, from_handle, to_handle, subject, body, type)
		VALUES ($1, $2, 'coordinator', 'Decision resolved', $3, 'decision_gate')
	`, tenantID, replyFromHandle, resolution); err != nil {
		return domain.DecisionGate{}, nil, fmt.Errorf("postgres: insert coordinator message: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.DecisionGate{}, nil, fmt.Errorf("postgres: commit tx: %w", err)
	}

	gate.Status = domain.GateStatusResolved
	gate.Resolution = resolution
	gate.ResolvedAt = now
	return gate, []string{gate.OrchestrationTaskID}, nil
}

func (r *Repository) ListPending(ctx context.Context, tenantID string) ([]domain.DecisionGate, error) {
	// Reuses idx_gates_pending (0001_init.up.sql:91,
	// WHERE status = 'pending') — this query's WHERE clause matches that
	// partial index verbatim so the planner can actually use it.
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, orchestration_task_id, dispatch_context_id, question, options, status, resolution, created_at, resolved_at
		FROM orchestration.decision_gates
		WHERE tenant_id = $1 AND status = 'pending'
		ORDER BY created_at ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query pending gates: %w", err)
	}
	defer rows.Close()

	out := []domain.DecisionGate{}
	for rows.Next() {
		var g domain.DecisionGate
		var status string
		var dispatchContextID, resolution *string
		var optionsJSON []byte
		var resolvedAt *time.Time
		if err := rows.Scan(&g.ID, &g.TenantID, &g.OrchestrationTaskID, &dispatchContextID, &g.Question, &optionsJSON, &status, &resolution, &g.CreatedAt, &resolvedAt); err != nil {
			return nil, fmt.Errorf("postgres: scan pending gate: %w", err)
		}
		g.Status = domain.GateStatus(status)
		if dispatchContextID != nil {
			g.DispatchContextID = *dispatchContextID
		}
		if resolution != nil {
			g.Resolution = *resolution
		}
		if resolvedAt != nil {
			g.ResolvedAt = *resolvedAt
		}
		if len(optionsJSON) > 0 {
			if err := json.Unmarshal(optionsJSON, &g.Options); err != nil {
				return nil, fmt.Errorf("postgres: unmarshal gate options: %w", err)
			}
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate pending gates: %w", err)
	}
	return out, nil
}

// ---- CoordinatorRunRepository -------------------------------------

func (r *Repository) CreateWithTasks(ctx context.Context, tenantID string, run domain.CoordinatorRun) (domain.CoordinatorRun, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	id := run.ID
	if id == "" {
		id = uuid.NewString()
	}
	var worktreeIDArg *string
	if run.WorktreeID != "" {
		worktreeIDArg = &run.WorktreeID
	}
	var createdAt time.Time
	if err := tx.QueryRow(ctx, `
		INSERT INTO orchestration.coordinator_runs (id, tenant_id, origin_task_id, spec, status, coordinator_handle, poll_interval_ms, worktree_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING created_at
	`, id, tenantID, run.OriginTaskID, run.Spec, string(run.Status), run.CoordinatorHandle, run.PollIntervalMs, worktreeIDArg).Scan(&createdAt); err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("postgres: insert coordinator run: %w", err)
	}

	// ExpandSpec resolves each node's own tempId->real-id NOW that the
	// run's real id (id, minted above) exists — mirrors
	// domain.ExpandSpec's own doc comment: "real IDs are NOT minted here."
	tasks, err := domain.ExpandSpec(tenantID, id, run.OriginTaskID, run.Spec)
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("postgres: expand spec: %w", err)
	}

	// tempID -> freshly-minted real UUID, built before any INSERT so every
	// task's Deps (still tempIDs at this point) can be resolved to real
	// ids in the SAME loop that inserts them — a torn resolve here would
	// leave a task with a dep pointing at a tempId string instead of a
	// real row, permanently stuck (never satisfied by DepsSatisfied).
	realIDs := make(map[string]string, len(tasks))
	var nodes []domain.SpecNode
	if err := json.Unmarshal(run.Spec, &nodes); err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("postgres: re-parse spec for id mapping: %w", err)
	}
	for _, n := range nodes {
		realIDs[n.TempID] = uuid.NewString()
	}

	for i, t := range tasks {
		taskID := realIDs[nodes[i].TempID]
		resolvedDeps := make([]string, 0, len(t.Deps))
		for _, tempDep := range t.Deps {
			resolvedDeps = append(resolvedDeps, realIDs[tempDep])
		}
		depsJSON, err := json.Marshal(resolvedDeps)
		if err != nil {
			return domain.CoordinatorRun{}, fmt.Errorf("postgres: marshal resolved deps: %w", err)
		}
		spec := t.Spec
		if spec == nil {
			spec = json.RawMessage(`{}`)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO orchestration.orchestration_tasks (
				id, tenant_id, coordinator_run_id, origin_task_id, task_title, spec, status, deps
			) VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8)
		`, taskID, tenantID, id, t.OriginTaskID, t.TaskTitle, spec, string(t.Status), depsJSON); err != nil {
			return domain.CoordinatorRun{}, fmt.Errorf("postgres: insert orchestration task %q: %w", t.TaskTitle, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("postgres: commit create-with-tasks tx: %w", err)
	}

	run.ID = id
	run.CreatedAt = createdAt
	return run, nil
}

func (r *Repository) GetRun(ctx context.Context, tenantID, id string) (domain.CoordinatorRun, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, origin_task_id, spec, status, coordinator_handle, poll_interval_ms,
		       worktree_id, result, error_message, created_at, completed_at, reported_at
		FROM orchestration.coordinator_runs
		WHERE id = $1 AND tenant_id = $2
	`, id, tenantID)
	run, err := scanCoordinatorRun(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CoordinatorRun{}, usecase.ErrRunNotFound
	}
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("postgres: query coordinator run: %w", err)
	}
	return run, nil
}

func (r *Repository) Complete(ctx context.Context, tenantID, id string, result json.RawMessage) (domain.CoordinatorRun, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE orchestration.coordinator_runs
		SET status = 'completed', result = $1, completed_at = now()
		WHERE id = $2 AND tenant_id = $3 AND status = 'running'
		RETURNING id, tenant_id, origin_task_id, spec, status, coordinator_handle, poll_interval_ms,
		          worktree_id, result, error_message, created_at, completed_at, reported_at
	`, result, id, tenantID)
	run, err := scanCoordinatorRun(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CoordinatorRun{}, usecase.ErrRunNotFound
	}
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("postgres: complete coordinator run: %w", err)
	}
	return run, nil
}

func (r *Repository) Fail(ctx context.Context, tenantID, id, errMsg string) (domain.CoordinatorRun, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE orchestration.coordinator_runs
		SET status = 'failed', error_message = $1, completed_at = now()
		WHERE id = $2 AND tenant_id = $3 AND status IN ('running', 'idle')
		RETURNING id, tenant_id, origin_task_id, spec, status, coordinator_handle, poll_interval_ms,
		          worktree_id, result, error_message, created_at, completed_at, reported_at
	`, errMsg, id, tenantID)
	run, err := scanCoordinatorRun(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CoordinatorRun{}, usecase.ErrRunNotFound
	}
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("postgres: fail coordinator run: %w", err)
	}
	return run, nil
}

func (r *Repository) ListRunning(ctx context.Context) ([]domain.CoordinatorRun, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, origin_task_id, spec, status, coordinator_handle, poll_interval_ms,
		       worktree_id, result, error_message, created_at, completed_at, reported_at
		FROM orchestration.coordinator_runs
		WHERE status = 'running'
	`)
	if err != nil {
		return nil, fmt.Errorf("postgres: query running coordinator runs: %w", err)
	}
	defer rows.Close()

	out := []domain.CoordinatorRun{}
	for rows.Next() {
		run, err := scanCoordinatorRun(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan running coordinator run: %w", err)
		}
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate running coordinator runs: %w", err)
	}
	return out, nil
}

func (r *Repository) MarkReported(ctx context.Context, tenantID, id string) error {
	if _, err := r.pool.Exec(ctx, `
		UPDATE orchestration.coordinator_runs SET reported_at = now()
		WHERE id = $1 AND tenant_id = $2
	`, id, tenantID); err != nil {
		return fmt.Errorf("postgres: mark coordinator run reported: %w", err)
	}
	return nil
}

func (r *Repository) ListUnreportedTerminal(ctx context.Context) ([]domain.CoordinatorRun, error) {
	// Reuses idx_coordinator_runs_unreported (TASK-TASKV1-005-01) — this
	// WHERE clause matches that partial index verbatim.
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, origin_task_id, spec, status, coordinator_handle, poll_interval_ms,
		       worktree_id, result, error_message, created_at, completed_at, reported_at
		FROM orchestration.coordinator_runs
		WHERE status IN ('completed', 'failed') AND reported_at IS NULL
	`)
	if err != nil {
		return nil, fmt.Errorf("postgres: query unreported terminal runs: %w", err)
	}
	defer rows.Close()

	out := []domain.CoordinatorRun{}
	for rows.Next() {
		run, err := scanCoordinatorRun(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan unreported terminal run: %w", err)
		}
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate unreported terminal runs: %w", err)
	}
	return out, nil
}

// scanCoordinatorRun mirrors scanTask's nullable-column-handling
// convention (repository.go:220-243), shared by GetRun/Complete/Fail/ListRunning
// so the mapping can't drift between them.
func scanCoordinatorRun(row pgx.Row) (domain.CoordinatorRun, error) {
	var run domain.CoordinatorRun
	var status string
	var specJSON, resultJSON []byte
	var worktreeID, errorMessage *string
	var completedAt, reportedAt *time.Time
	if err := row.Scan(
		&run.ID, &run.TenantID, &run.OriginTaskID, &specJSON, &status, &run.CoordinatorHandle, &run.PollIntervalMs,
		&worktreeID, &resultJSON, &errorMessage, &run.CreatedAt, &completedAt, &reportedAt,
	); err != nil {
		return domain.CoordinatorRun{}, err
	}
	run.Status = domain.RunStatus(status)
	run.Spec = specJSON
	run.Result = resultJSON
	if worktreeID != nil {
		run.WorktreeID = *worktreeID
	}
	if errorMessage != nil {
		run.ErrorMessage = *errorMessage
	}
	if completedAt != nil {
		run.CompletedAt = *completedAt
	}
	if reportedAt != nil {
		run.ReportedAt = *reportedAt
	}
	return run, nil
}

// ---- OrchestrationTaskRepository extensions ----------------------------

func (r *Repository) ListReadyUnclaimed(ctx context.Context) ([]domain.OrchestrationTask, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ot.id, ot.tenant_id, ot.coordinator_run_id, COALESCE(ot.parent_id::text, ''), COALESCE(ot.origin_task_id, ''),
		       ot.task_title, ot.spec, ot.status, ot.deps, ot.result, ot.created_at, ot.completed_at
		FROM orchestration.orchestration_tasks ot
		WHERE ot.status = 'ready'
		  AND NOT EXISTS (
		    SELECT 1 FROM orchestration.dispatch_contexts dc WHERE dc.orchestration_task_id = ot.id
		  )
	`)
	if err != nil {
		return nil, fmt.Errorf("postgres: query ready unclaimed tasks: %w", err)
	}
	defer rows.Close()

	out := []domain.OrchestrationTask{}
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan ready unclaimed task: %w", err)
		}
		out = append(out, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate ready unclaimed tasks: %w", err)
	}
	return out, nil
}

// ClaimReady is the CAS the tick loop relies on for "exactly one instance
// dispatches this task" — a plain UPDATE...WHERE status='ready' RETURNING,
// no explicit transaction needed since a single-row UPDATE is already
// atomic in Postgres.
func (r *Repository) ClaimReady(ctx context.Context, tenantID, taskID string) (domain.OrchestrationTask, bool, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE orchestration.orchestration_tasks
		SET status = 'dispatched'
		WHERE id = $1 AND tenant_id = $2 AND status = 'ready'
		RETURNING id, tenant_id, coordinator_run_id, COALESCE(parent_id::text, ''), COALESCE(origin_task_id, ''),
		          task_title, spec, status, deps, result, created_at, completed_at
	`, taskID, tenantID)
	task, err := scanTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OrchestrationTask{}, false, nil // lost the race — not an error
	}
	if err != nil {
		return domain.OrchestrationTask{}, false, fmt.Errorf("postgres: claim ready task: %w", err)
	}
	return task, true, nil
}

func (r *Repository) CountNonTerminalByRun(ctx context.Context, tenantID, coordinatorRunID string) (int, error) {
	var count int
	if err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM orchestration.orchestration_tasks
		WHERE coordinator_run_id = $1 AND tenant_id = $2
		  AND status NOT IN ('completed', 'failed')
	`, coordinatorRunID, tenantID).Scan(&count); err != nil {
		return 0, fmt.Errorf("postgres: count non-terminal tasks: %w", err)
	}
	return count, nil
}

// ---- DispatchContextRepository.RecordHeartbeat -------------------------

func (r *Repository) RecordHeartbeat(ctx context.Context, tenantID, dispatchContextID string) (domain.DispatchContext, error) {
	if _, err := uuid.Parse(dispatchContextID); err != nil {
		// id is UUID-typed in the DB — a malformed id can never match a
		// real row, so treat it the same as "not found" rather than
		// leaking Postgres's raw invalid-input-syntax error to the caller.
		return domain.DispatchContext{}, usecase.ErrDispatchContextNotFound
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE orchestration.dispatch_contexts
		SET last_heartbeat_at = now()
		WHERE id = $1 AND tenant_id = $2
		RETURNING id, tenant_id, worktree_id, orchestration_task_id, handle, coordinator_run_id, status, created_at, last_heartbeat_at
	`, dispatchContextID, tenantID)

	var dc domain.DispatchContext
	var status string
	var worktreeID, orchestrationTaskID *string
	if err := row.Scan(&dc.ID, &dc.TenantID, &worktreeID, &orchestrationTaskID, &dc.Handle, &dc.CoordinatorRunID, &status, &dc.CreatedAt, &dc.LastHeartbeatAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DispatchContext{}, usecase.ErrDispatchContextNotFound
		}
		return domain.DispatchContext{}, fmt.Errorf("postgres: record heartbeat: %w", err)
	}
	if worktreeID != nil {
		dc.WorktreeID = *worktreeID
	}
	if orchestrationTaskID != nil {
		dc.OrchestrationTaskID = *orchestrationTaskID
	}
	dc.Status = domain.DispatchStatus(status)
	return dc, nil
}

// ---- common/outbox.Store -------------------------------------------------
//
// FetchUnpublished and MarkPublished implement common/outbox.Store — see
// cmd/server/main.go for where the relay is wired (BE-SOL-003/TASK-FT-003-01),
// same shape as usage-service's identically-named methods
// (internal/adapter/postgres/repository.go:99-140 there) against
// orchestration.outbox_events instead of usage.outbox_events.

func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM orchestration.outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: query unpublished outbox events: %w", err)
	}
	defer rows.Close()

	var out []outbox.Record
	for rows.Next() {
		var rec outbox.Record
		if err := rows.Scan(&rec.ID, &rec.Event.TenantID, &rec.Subject, &rec.Event.OccurredAt, &rec.Event.Version, &rec.Event.Payload); err != nil {
			return nil, fmt.Errorf("postgres: scan outbox event row: %w", err)
		}
		rec.Event.ID = rec.ID
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate outbox event rows: %w", err)
	}
	return out, nil
}

func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `UPDATE orchestration.outbox_events SET published_at = now() WHERE id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("postgres: mark outbox events published: %w", err)
	}
	return nil
}
