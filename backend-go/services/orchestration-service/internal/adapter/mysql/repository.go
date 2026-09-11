// Package mysql implements orchestration-service's repository ports
// (usecase.OrchestrationTaskRepository, usecase.DispatchContextRepository,
// usecase.GateRepository, usecase.CoordinatorRunRepository) plus
// common/outbox.Store against MySQL/TiDB via database/sql +
// github.com/go-sql-driver/mysql — this service's multi-dialect rollout
// adapter for CR-DB-002/CR-DB-003 (batch 2), mirroring
// internal/adapter/postgres's behavior 1:1 against the dialect-safe schema
// in migrations/mysql/{0001_init,...,0006_coordinator_run_heartbeat}.up.sql
// (tables `coordinator_runs`, `orchestration_tasks`, `dispatch_contexts`,
// `decision_gates`, `messages`, `outbox_events`, no schema/database prefix
// — same naming decision as usage-service's pilot adapter, see
// specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-002.md).
//
// UpdateStatusAndPromote, CreateGate and ResolveGate are each one MySQL
// transaction, same hard NFR as the Postgres adapter (orchestration-service.md
// §8): a torn read between marking a task complete and re-scanning its
// dependents can double-dispatch a task or leave a ready task stuck
// pending.
package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/usecase"
)

// Repository implements usecase.OrchestrationTaskRepository,
// usecase.DispatchContextRepository, usecase.GateRepository,
// usecase.CoordinatorRunRepository, and common/outbox.Store against
// MySQL/TiDB via database/sql. No RLS equivalent exists in MySQL — every
// query below filters by tenant_id explicitly, which is the ONLY
// tenant-isolation enforcement for this adapter (see BE-DB-SOL-001 §4: this
// was already true for the Postgres adapter too, since RLS never actually
// activated there — this doesn't lower the bar, it just doesn't add a
// backstop that was never real).
type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// scannable is satisfied by both *sql.Row and *sql.Rows, letting scanTask/
// scanCoordinatorRun be shared between QueryRowContext and QueryContext
// call sites — same trick internal/adapter/postgres's own scanTask/
// scanCoordinatorRun play against pgx.Row/pgx.Rows.
type scannable interface {
	Scan(dest ...any) error
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
		return domain.OrchestrationTask{}, fmt.Errorf("mysql: marshal deps: %w", err)
	}

	createdAt := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO orchestration_tasks (
			id, tenant_id, coordinator_run_id, parent_id, origin_task_id, task_title, spec, status, deps, created_at
		) VALUES (?,?,?,?,?,?,?,'pending',?,?)
	`, id, task.TenantID, task.CoordinatorRunID, nullableString(task.ParentID), nullableString(task.OriginTaskID), task.TaskTitle, []byte(spec), depsJSON, createdAt)
	if err != nil {
		return domain.OrchestrationTask{}, fmt.Errorf("mysql: insert orchestration task: %w", err)
	}

	task.ID = id
	task.Status = domain.TaskStatusPending
	task.Spec = spec
	task.CreatedAt = createdAt
	return task, nil
}

func (r *Repository) Get(ctx context.Context, tenantID, id string) (domain.OrchestrationTask, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, coordinator_run_id, COALESCE(parent_id, ''), COALESCE(origin_task_id, ''),
		       task_title, spec, status, deps, result, created_at, completed_at
		FROM orchestration_tasks
		WHERE id = ? AND tenant_id = ?
	`, id, tenantID)
	task, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OrchestrationTask{}, usecase.ErrTaskNotFound
	}
	if err != nil {
		return domain.OrchestrationTask{}, fmt.Errorf("mysql: query orchestration task: %w", err)
	}
	return task, nil
}

// UpdateStatusAndPromote is the atomic promote saga (§8) — see
// internal/adapter/postgres's identically-named method's doc comment for
// the full rationale; this is the same sequence of steps, one MySQL
// transaction, translated statement by statement.
func (r *Repository) UpdateStatusAndPromote(ctx context.Context, tenantID, taskID string, newStatus domain.TaskStatus, event domain.OutboxEvent) (usecase.UpdateStatusAndPromoteResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return usecase.UpdateStatusAndPromoteResult{}, fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var completedAt *time.Time
	if newStatus == domain.TaskStatusCompleted || newStatus == domain.TaskStatusFailed {
		now := time.Now().UTC()
		completedAt = &now
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE orchestration_tasks
		SET status = ?, completed_at = COALESCE(?, completed_at)
		WHERE id = ? AND tenant_id = ?
	`, string(newStatus), completedAt, taskID, tenantID)
	if err != nil {
		return usecase.UpdateStatusAndPromoteResult{}, fmt.Errorf("mysql: update task status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return usecase.UpdateStatusAndPromoteResult{}, fmt.Errorf("mysql: update task status rows affected: %w", err)
	}
	if affected == 0 {
		return usecase.UpdateStatusAndPromoteResult{}, usecase.ErrTaskNotFound
	}

	task, err := scanTask(tx.QueryRowContext(ctx, `
		SELECT id, tenant_id, coordinator_run_id, COALESCE(parent_id, ''), COALESCE(origin_task_id, ''),
		       task_title, spec, status, deps, result, created_at, completed_at
		FROM orchestration_tasks
		WHERE id = ? AND tenant_id = ?
	`, taskID, tenantID))
	if err != nil {
		return usecase.UpdateStatusAndPromoteResult{}, fmt.Errorf("mysql: reselect updated task: %w", err)
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
	// skip the extra queries on every other status write.
	if newStatus == domain.TaskStatusCompleted || newStatus == domain.TaskStatusFailed {
		var nonTerminal int
		if err := tx.QueryRowContext(ctx, `
			SELECT count(*) FROM orchestration_tasks
			WHERE coordinator_run_id = ? AND tenant_id = ?
			  AND status NOT IN ('completed', 'failed')`,
			task.CoordinatorRunID, tenantID).Scan(&nonTerminal); err != nil {
			return usecase.UpdateStatusAndPromoteResult{}, fmt.Errorf("mysql: count non-terminal: %w", err)
		}
		if nonTerminal == 0 {
			// Every sibling in this run is now terminal — fail-closed: a
			// single failed leaf fails the WHOLE run (see postgres
			// adapter's identical comment).
			var anyFailed bool
			if err := tx.QueryRowContext(ctx, `
				SELECT EXISTS(SELECT 1 FROM orchestration_tasks
					WHERE coordinator_run_id = ? AND tenant_id = ? AND status = 'failed')`,
				task.CoordinatorRunID, tenantID).Scan(&anyFailed); err != nil {
				return usecase.UpdateStatusAndPromoteResult{}, fmt.Errorf("mysql: check any-failed: %w", err)
			}
			runStatus := "completed"
			if anyFailed {
				runStatus = "failed"
			}

			// No RETURNING in MySQL: fetch origin_task_id BEFORE the CAS
			// UPDATE (it never changes), then use the UPDATE's affected-row
			// count as the "did I win the race to finalize" signal — same
			// semantics as Postgres's `WHERE status = 'running'
			// RETURNING origin_task_id` + pgx.ErrNoRows-means-raced.
			var originTaskID string
			if err := tx.QueryRowContext(ctx, `
				SELECT origin_task_id FROM coordinator_runs WHERE id = ? AND tenant_id = ?
			`, task.CoordinatorRunID, tenantID).Scan(&originTaskID); err != nil {
				return usecase.UpdateStatusAndPromoteResult{}, fmt.Errorf("mysql: fetch origin_task_id for finalize: %w", err)
			}
			finalizeRes, err := tx.ExecContext(ctx, `
				UPDATE coordinator_runs SET status = ?, completed_at = ?
				WHERE id = ? AND tenant_id = ? AND status = 'running'
			`, runStatus, time.Now().UTC(), task.CoordinatorRunID, tenantID)
			if err != nil {
				return usecase.UpdateStatusAndPromoteResult{}, fmt.Errorf("mysql: finalize run: %w", err)
			}
			finalizeAffected, err := finalizeRes.RowsAffected()
			if err != nil {
				return usecase.UpdateStatusAndPromoteResult{}, fmt.Errorf("mysql: finalize run rows affected: %w", err)
			}
			if finalizeAffected == 1 {
				finalized = &usecase.RunFinalization{
					CoordinatorRunID: task.CoordinatorRunID,
					OriginTaskID:     originTaskID,
					Success:          !anyFailed,
				}
			}
			// finalizeAffected == 0: the run was already completed/failed
			// by a racing concurrent call — do not double-finalize, do not
			// error the whole transaction, just skip setting `finalized`.
		}
	}

	if event.ID != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES (?, ?, ?, ?, 1, ?)
		`, event.ID, tenantID, event.Subject, event.OccurredAt, event.PayloadJSON); err != nil {
			return usecase.UpdateStatusAndPromoteResult{}, fmt.Errorf("mysql: insert outbox event: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return usecase.UpdateStatusAndPromoteResult{}, fmt.Errorf("mysql: commit tx: %w", err)
	}
	return usecase.UpdateStatusAndPromoteResult{Task: task, PromotedIDs: promotedIDs, RunFinalized: finalized}, nil
}

// promoteReadySiblings mirrors internal/adapter/postgres's function of the
// same name — see its doc comment.
func promoteReadySiblings(ctx context.Context, tx *sql.Tx, tenantID, coordinatorRunID string) ([]string, error) {
	completedRows, err := tx.QueryContext(ctx, `
		SELECT id FROM orchestration_tasks
		WHERE coordinator_run_id = ? AND tenant_id = ? AND status = 'completed'
	`, coordinatorRunID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query completed siblings: %w", err)
	}
	completed := map[string]struct{}{}
	for completedRows.Next() {
		var id string
		if err := completedRows.Scan(&id); err != nil {
			completedRows.Close()
			return nil, fmt.Errorf("mysql: scan completed sibling id: %w", err)
		}
		completed[id] = struct{}{}
	}
	completedRows.Close()
	if err := completedRows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate completed siblings: %w", err)
	}

	pendingRows, err := tx.QueryContext(ctx, `
		SELECT id, deps FROM orchestration_tasks
		WHERE coordinator_run_id = ? AND tenant_id = ? AND status = 'pending'
	`, coordinatorRunID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query pending siblings: %w", err)
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
			return nil, fmt.Errorf("mysql: scan pending sibling: %w", err)
		}
		if err := json.Unmarshal(depsJSON, &c.deps); err != nil {
			pendingRows.Close()
			return nil, fmt.Errorf("mysql: unmarshal deps: %w", err)
		}
		candidates = append(candidates, c)
	}
	pendingRows.Close()
	if err := pendingRows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate pending siblings: %w", err)
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

	// MySQL has no `id = ANY($1)` array predicate — build IN (?,?,...)
	// dynamically, same technique usage-service's MarkPublished already
	// uses for outbox_events ids.
	placeholders, args := inClausePlaceholders(promoted)
	args = append(args, tenantID)
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE orchestration_tasks SET status = 'ready'
		WHERE id IN (%s) AND tenant_id = ?
	`, placeholders), args...); err != nil {
		return nil, fmt.Errorf("mysql: promote ready siblings: %w", err)
	}
	return promoted, nil
}

func scanTask(row scannable) (domain.OrchestrationTask, error) {
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
			return domain.OrchestrationTask{}, fmt.Errorf("mysql: unmarshal deps: %w", err)
		}
	}
	return task, nil
}

// ---- DispatchContextRepository ------------------------------------------

func (r *Repository) CreateDispatchContext(ctx context.Context, tenantID, userID, worktreeID, handle, coordinatorRunID, orchestrationTaskID string, event domain.OutboxEvent) (domain.DispatchContext, error) {
	id := uuid.NewString()
	createdAt := time.Now().UTC()

	// Wrapped in a transaction (mirrors postgres adapter) — the outbox row
	// must commit atomically with the dispatch-context row.
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.DispatchContext{}, fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO dispatch_contexts (id, tenant_id, user_id, worktree_id, handle, coordinator_run_id, orchestration_task_id, status, created_at)
		VALUES (?,?,?,?,?,?,?,'pending',?)
	`, id, tenantID, nullableString(userID), nullableString(worktreeID), handle, coordinatorRunID, nullableString(orchestrationTaskID), createdAt); err != nil {
		return domain.DispatchContext{}, fmt.Errorf("mysql: insert dispatch context: %w", err)
	}

	if event.ID != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES (?, ?, ?, ?, 1, ?)
		`, event.ID, tenantID, event.Subject, event.OccurredAt, event.PayloadJSON); err != nil {
			return domain.DispatchContext{}, fmt.Errorf("mysql: insert outbox event: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return domain.DispatchContext{}, fmt.Errorf("mysql: commit tx: %w", err)
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

func (r *Repository) ListActiveDispatchContextsForUser(ctx context.Context, tenantID, userID string) ([]domain.DispatchContext, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, tenant_id, user_id, worktree_id, handle, coordinator_run_id, orchestration_task_id,
		       status, failure_count, last_failure, dispatched_at, completed_at,
		       last_heartbeat_at, created_at
		FROM dispatch_contexts
		WHERE tenant_id = ? AND user_id = ?
		  AND status NOT IN ('completed', 'failed', 'circuit_broken')
		ORDER BY created_at DESC
	`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query active dispatch contexts for user: %w", err)
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
			return nil, fmt.Errorf("mysql: scan active dispatch context row: %w", err)
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
		return nil, fmt.Errorf("mysql: iterate active dispatch context rows: %w", err)
	}
	return out, nil
}

func (r *Repository) GetLatestForTask(ctx context.Context, tenantID, orchestrationTaskID string) (domain.DispatchContext, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, worktree_id, orchestration_task_id, handle, coordinator_run_id, status, created_at
		FROM dispatch_contexts
		WHERE tenant_id = ? AND orchestration_task_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, tenantID, orchestrationTaskID)

	var dc domain.DispatchContext
	var status string
	var worktreeIDCol, orchestrationTaskIDCol *string
	if err := row.Scan(&dc.ID, &dc.TenantID, &worktreeIDCol, &orchestrationTaskIDCol, &dc.Handle, &dc.CoordinatorRunID, &status, &dc.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.DispatchContext{}, usecase.ErrDispatchContextNotFound
		}
		return domain.DispatchContext{}, fmt.Errorf("mysql: get latest dispatch context for task: %w", err)
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

func (r *Repository) GetDispatchContext(ctx context.Context, tenantID, id string) (domain.DispatchContext, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, worktree_id, orchestration_task_id, handle, coordinator_run_id, status, created_at
		FROM dispatch_contexts
		WHERE tenant_id = ? AND id = ?
	`, tenantID, id)

	var dc domain.DispatchContext
	var status string
	var worktreeIDCol, orchestrationTaskIDCol *string
	if err := row.Scan(&dc.ID, &dc.TenantID, &worktreeIDCol, &orchestrationTaskIDCol, &dc.Handle, &dc.CoordinatorRunID, &status, &dc.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.DispatchContext{}, usecase.ErrDispatchContextNotFound
		}
		return domain.DispatchContext{}, fmt.Errorf("mysql: get dispatch context: %w", err)
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

func (r *Repository) RecordDispatchFailure(ctx context.Context, tenantID, dispatchContextID, reason string) (domain.DispatchContext, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.DispatchContext{}, fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var dc domain.DispatchContext
	var status string
	var orchestrationTaskID, userID *string
	err = tx.QueryRowContext(ctx, `
		SELECT id, tenant_id, user_id, handle, coordinator_run_id, orchestration_task_id, status, failure_count
		FROM dispatch_contexts
		WHERE id = ? AND tenant_id = ?
		FOR UPDATE
	`, dispatchContextID, tenantID).Scan(
		&dc.ID, &dc.TenantID, &userID, &dc.Handle, &dc.CoordinatorRunID, &orchestrationTaskID, &status, &dc.FailureCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DispatchContext{}, usecase.ErrDispatchContextNotFound
	}
	if err != nil {
		return domain.DispatchContext{}, fmt.Errorf("mysql: query dispatch context for failure record: %w", err)
	}
	if userID != nil {
		dc.UserID = *userID
	}
	if orchestrationTaskID != nil {
		dc.OrchestrationTaskID = *orchestrationTaskID
	}
	dc.Status = domain.DispatchStatus(status)

	updated := dc.RecordFailure(reason)

	if _, err := tx.ExecContext(ctx, `
		UPDATE dispatch_contexts
		SET failure_count = ?, status = ?, last_failure = ?
		WHERE id = ? AND tenant_id = ?
	`, updated.FailureCount, string(updated.Status), updated.LastFailure, dispatchContextID, tenantID); err != nil {
		return domain.DispatchContext{}, fmt.Errorf("mysql: update dispatch context failure: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return domain.DispatchContext{}, fmt.Errorf("mysql: commit dispatch failure tx: %w", err)
	}
	return updated, nil
}

func (r *Repository) RecordHeartbeat(ctx context.Context, tenantID, dispatchContextID string) (domain.DispatchContext, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.DispatchContext{}, fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var dc domain.DispatchContext
	var status string
	var worktreeID, orchestrationTaskID *string
	err = tx.QueryRowContext(ctx, `
		SELECT id, tenant_id, worktree_id, orchestration_task_id, handle, coordinator_run_id, status, created_at
		FROM dispatch_contexts
		WHERE id = ? AND tenant_id = ?
		FOR UPDATE
	`, dispatchContextID, tenantID).Scan(&dc.ID, &dc.TenantID, &worktreeID, &orchestrationTaskID, &dc.Handle, &dc.CoordinatorRunID, &status, &dc.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DispatchContext{}, usecase.ErrDispatchContextNotFound
	}
	if err != nil {
		return domain.DispatchContext{}, fmt.Errorf("mysql: query dispatch context for heartbeat: %w", err)
	}
	if worktreeID != nil {
		dc.WorktreeID = *worktreeID
	}
	if orchestrationTaskID != nil {
		dc.OrchestrationTaskID = *orchestrationTaskID
	}
	dc.Status = domain.DispatchStatus(status)

	hb := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		UPDATE dispatch_contexts SET last_heartbeat_at = ? WHERE id = ? AND tenant_id = ?
	`, hb, dispatchContextID, tenantID); err != nil {
		return domain.DispatchContext{}, fmt.Errorf("mysql: record heartbeat: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return domain.DispatchContext{}, fmt.Errorf("mysql: commit heartbeat tx: %w", err)
	}
	dc.LastHeartbeatAt = hb
	return dc, nil
}

// ---- GateRepository -------------------------------------------------

func (r *Repository) CreateGate(ctx context.Context, tenantID, dispatchContextID, question string, options []string, event domain.OutboxEvent) (domain.DecisionGate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.DecisionGate{}, fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var orchestrationTaskID *string
	var toHandle string
	err = tx.QueryRowContext(ctx, `
		SELECT orchestration_task_id, handle FROM dispatch_contexts
		WHERE id = ? AND tenant_id = ?
		FOR UPDATE
	`, dispatchContextID, tenantID).Scan(&orchestrationTaskID, &toHandle)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DecisionGate{}, usecase.ErrDispatchContextNotFound
	}
	if err != nil {
		return domain.DecisionGate{}, fmt.Errorf("mysql: query dispatch context: %w", err)
	}
	if orchestrationTaskID == nil || *orchestrationTaskID == "" {
		return domain.DecisionGate{}, usecase.ErrDispatchContextHasNoTask
	}

	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return domain.DecisionGate{}, fmt.Errorf("mysql: marshal options: %w", err)
	}

	id := uuid.NewString()
	createdAt := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO decision_gates (
			id, tenant_id, orchestration_task_id, dispatch_context_id, question, options, status, created_at
		) VALUES (?,?,?,?,?,?,'pending',?)
	`, id, tenantID, *orchestrationTaskID, dispatchContextID, question, optionsJSON, createdAt); err != nil {
		return domain.DecisionGate{}, fmt.Errorf("mysql: insert decision gate: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE orchestration_tasks SET status = 'blocked'
		WHERE id = ? AND tenant_id = ?
	`, *orchestrationTaskID, tenantID); err != nil {
		return domain.DecisionGate{}, fmt.Errorf("mysql: block owning task: %w", err)
	}

	// messages' first real write, mirroring postgres adapter's CreateGate —
	// `read` is backtick-quoted (MySQL reserved word) even though this
	// INSERT relies on its column default rather than setting it.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO messages (tenant_id, from_handle, to_handle, subject, body, type, payload)
		VALUES (?, 'coordinator', ?, 'Decision needed', ?, 'decision_gate', ?)
	`, tenantID, toHandle, question, optionsJSON); err != nil {
		return domain.DecisionGate{}, fmt.Errorf("mysql: insert coordinator message: %w", err)
	}

	if event.ID != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES (?, ?, ?, ?, 1, ?)
		`, event.ID, tenantID, event.Subject, event.OccurredAt, event.PayloadJSON); err != nil {
			return domain.DecisionGate{}, fmt.Errorf("mysql: insert outbox event: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return domain.DecisionGate{}, fmt.Errorf("mysql: commit tx: %w", err)
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

// ResolveGate locks the gate row (plus the joined dispatch_contexts row —
// MySQL's `SELECT ... FOR UPDATE` over a join locks every table it touches;
// unlike Postgres's `FOR UPDATE OF g`, MySQL's per-table lock list syntax
// is a newer (8.0.20+) feature this adapter avoids depending on, so this is
// slightly more conservative locking scope than the Postgres adapter, not
// less) before checking `status = 'pending'` — same "resolved exactly
// once" invariant enforced under a race.
func (r *Repository) ResolveGate(ctx context.Context, tenantID, gateID, resolution string) (domain.DecisionGate, []string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.DecisionGate{}, nil, fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var gate domain.DecisionGate
	var status string
	var dispatchContextID *string
	var fromHandle *string
	var optionsJSON []byte
	var resolvedAt *time.Time
	// resolution is nullable (a still-pending gate has never been
	// resolved) — scanned into *string, not gate.Resolution directly, to
	// avoid "converting NULL to string is unsupported"
	// (database/sql.Scan refuses NULL into a non-pointer string; this is
	// the SAME class of bug found pre-existing in
	// internal/adapter/postgres's ResolveGate — see BE-DB-SOL-010's
	// "Kết quả thực tế" — but fixed HERE since this is this rollout's own
	// new code, not the out-of-scope pre-existing Postgres adapter).
	var resolutionCol *string
	err = tx.QueryRowContext(ctx, `
		SELECT g.id, g.tenant_id, g.orchestration_task_id, g.dispatch_context_id, g.question, g.options,
		       g.status, g.resolution, g.created_at, g.resolved_at, dc.handle
		FROM decision_gates g
		LEFT JOIN dispatch_contexts dc ON dc.id = g.dispatch_context_id
		WHERE g.id = ? AND g.tenant_id = ?
		FOR UPDATE
	`, gateID, tenantID).Scan(
		&gate.ID, &gate.TenantID, &gate.OrchestrationTaskID, &dispatchContextID, &gate.Question, &optionsJSON,
		&status, &resolutionCol, &gate.CreatedAt, &resolvedAt, &fromHandle,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DecisionGate{}, nil, usecase.ErrGateNotFound
	}
	if err != nil {
		return domain.DecisionGate{}, nil, fmt.Errorf("mysql: query decision gate: %w", err)
	}
	if resolutionCol != nil {
		gate.Resolution = *resolutionCol
	}
	if status != string(domain.GateStatusPending) {
		return domain.DecisionGate{}, nil, usecase.ErrGateNotPending
	}
	if dispatchContextID != nil {
		gate.DispatchContextID = *dispatchContextID
	}
	if len(optionsJSON) > 0 {
		if err := json.Unmarshal(optionsJSON, &gate.Options); err != nil {
			return domain.DecisionGate{}, nil, fmt.Errorf("mysql: unmarshal gate options: %w", err)
		}
	}

	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		UPDATE decision_gates SET status = 'resolved', resolution = ?, resolved_at = ?
		WHERE id = ?
	`, resolution, now, gateID); err != nil {
		return domain.DecisionGate{}, nil, fmt.Errorf("mysql: resolve decision gate: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE orchestration_tasks SET status = 'ready'
		WHERE id = ? AND tenant_id = ? AND status = 'blocked'
	`, gate.OrchestrationTaskID, tenantID); err != nil {
		return domain.DecisionGate{}, nil, fmt.Errorf("mysql: unblock owning task: %w", err)
	}

	replyFromHandle := "coordinator"
	if fromHandle != nil && *fromHandle != "" {
		replyFromHandle = *fromHandle
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO messages (tenant_id, from_handle, to_handle, subject, body, type)
		VALUES (?, ?, 'coordinator', 'Decision resolved', ?, 'decision_gate')
	`, tenantID, replyFromHandle, resolution); err != nil {
		return domain.DecisionGate{}, nil, fmt.Errorf("mysql: insert coordinator message: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return domain.DecisionGate{}, nil, fmt.Errorf("mysql: commit tx: %w", err)
	}

	gate.Status = domain.GateStatusResolved
	gate.Resolution = resolution
	gate.ResolvedAt = now
	return gate, []string{gate.OrchestrationTaskID}, nil
}

func (r *Repository) ListPending(ctx context.Context, tenantID string) ([]domain.DecisionGate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, tenant_id, orchestration_task_id, dispatch_context_id, question, options, status, resolution, created_at, resolved_at
		FROM decision_gates
		WHERE tenant_id = ? AND status = 'pending'
		ORDER BY created_at ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query pending gates: %w", err)
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
			return nil, fmt.Errorf("mysql: scan pending gate: %w", err)
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
				return nil, fmt.Errorf("mysql: unmarshal gate options: %w", err)
			}
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate pending gates: %w", err)
	}
	return out, nil
}

// ---- CoordinatorRunRepository -------------------------------------

func (r *Repository) CreateWithTasks(ctx context.Context, tenantID string, run domain.CoordinatorRun) (domain.CoordinatorRun, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	id := run.ID
	if id == "" {
		id = uuid.NewString()
	}
	createdAt := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO coordinator_runs (id, tenant_id, origin_task_id, spec, status, coordinator_handle, poll_interval_ms, worktree_id, created_at)
		VALUES (?,?,?,?,?,?,?,?,?)
	`, id, tenantID, run.OriginTaskID, []byte(run.Spec), string(run.Status), run.CoordinatorHandle, run.PollIntervalMs, nullableString(run.WorktreeID), createdAt); err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("mysql: insert coordinator run: %w", err)
	}

	// ExpandSpec resolves each node's own tempId->real-id NOW that the
	// run's real id (id, minted above) exists — mirrors postgres adapter's
	// identical comment.
	tasks, err := domain.ExpandSpec(tenantID, id, run.OriginTaskID, run.Spec)
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("mysql: expand spec: %w", err)
	}

	realIDs := make(map[string]string, len(tasks))
	var nodes []domain.SpecNode
	if err := json.Unmarshal(run.Spec, &nodes); err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("mysql: re-parse spec for id mapping: %w", err)
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
			return domain.CoordinatorRun{}, fmt.Errorf("mysql: marshal resolved deps: %w", err)
		}
		spec := t.Spec
		if spec == nil {
			spec = json.RawMessage(`{}`)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO orchestration_tasks (
				id, tenant_id, coordinator_run_id, origin_task_id, task_title, spec, status, deps, created_at
			) VALUES (?,?,?,?,?,?,?,?,?)
		`, taskID, tenantID, id, nullableString(t.OriginTaskID), t.TaskTitle, []byte(spec), string(t.Status), depsJSON, createdAt); err != nil {
			return domain.CoordinatorRun{}, fmt.Errorf("mysql: insert orchestration task %q: %w", t.TaskTitle, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("mysql: commit create-with-tasks tx: %w", err)
	}

	run.ID = id
	run.CreatedAt = createdAt
	return run, nil
}

func (r *Repository) GetRun(ctx context.Context, tenantID, id string) (domain.CoordinatorRun, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, origin_task_id, spec, status, coordinator_handle, poll_interval_ms,
		       worktree_id, result, error_message, created_at, completed_at, reported_at
		FROM coordinator_runs
		WHERE id = ? AND tenant_id = ?
	`, id, tenantID)
	run, err := scanCoordinatorRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CoordinatorRun{}, usecase.ErrRunNotFound
	}
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("mysql: query coordinator run: %w", err)
	}
	return run, nil
}

func (r *Repository) Complete(ctx context.Context, tenantID, id string, result json.RawMessage) (domain.CoordinatorRun, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		UPDATE coordinator_runs
		SET status = 'completed', result = ?, completed_at = ?
		WHERE id = ? AND tenant_id = ? AND status = 'running'
	`, []byte(result), time.Now().UTC(), id, tenantID)
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("mysql: complete coordinator run: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("mysql: complete coordinator run rows affected: %w", err)
	}
	if affected == 0 {
		return domain.CoordinatorRun{}, usecase.ErrRunNotFound
	}

	run, err := scanCoordinatorRun(tx.QueryRowContext(ctx, `
		SELECT id, tenant_id, origin_task_id, spec, status, coordinator_handle, poll_interval_ms,
		       worktree_id, result, error_message, created_at, completed_at, reported_at
		FROM coordinator_runs WHERE id = ? AND tenant_id = ?
	`, id, tenantID))
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("mysql: reselect completed run: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("mysql: commit complete tx: %w", err)
	}
	return run, nil
}

func (r *Repository) Fail(ctx context.Context, tenantID, id, errMsg string) (domain.CoordinatorRun, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		UPDATE coordinator_runs
		SET status = 'failed', error_message = ?, completed_at = ?
		WHERE id = ? AND tenant_id = ? AND status IN ('running', 'idle')
	`, errMsg, time.Now().UTC(), id, tenantID)
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("mysql: fail coordinator run: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("mysql: fail coordinator run rows affected: %w", err)
	}
	if affected == 0 {
		return domain.CoordinatorRun{}, usecase.ErrRunNotFound
	}

	run, err := scanCoordinatorRun(tx.QueryRowContext(ctx, `
		SELECT id, tenant_id, origin_task_id, spec, status, coordinator_handle, poll_interval_ms,
		       worktree_id, result, error_message, created_at, completed_at, reported_at
		FROM coordinator_runs WHERE id = ? AND tenant_id = ?
	`, id, tenantID))
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("mysql: reselect failed run: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("mysql: commit fail tx: %w", err)
	}
	return run, nil
}

func (r *Repository) ListRunning(ctx context.Context) ([]domain.CoordinatorRun, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, tenant_id, origin_task_id, spec, status, coordinator_handle, poll_interval_ms,
		       worktree_id, result, error_message, created_at, completed_at, reported_at
		FROM coordinator_runs
		WHERE status = 'running'
	`)
	if err != nil {
		return nil, fmt.Errorf("mysql: query running coordinator runs: %w", err)
	}
	defer rows.Close()

	out := []domain.CoordinatorRun{}
	for rows.Next() {
		run, err := scanCoordinatorRun(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan running coordinator run: %w", err)
		}
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate running coordinator runs: %w", err)
	}
	return out, nil
}

func (r *Repository) MarkReported(ctx context.Context, tenantID, id string) error {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE coordinator_runs SET reported_at = ?
		WHERE id = ? AND tenant_id = ?
	`, time.Now().UTC(), id, tenantID); err != nil {
		return fmt.Errorf("mysql: mark coordinator run reported: %w", err)
	}
	return nil
}

func (r *Repository) ListUnreportedTerminal(ctx context.Context) ([]domain.CoordinatorRun, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, tenant_id, origin_task_id, spec, status, coordinator_handle, poll_interval_ms,
		       worktree_id, result, error_message, created_at, completed_at, reported_at
		FROM coordinator_runs
		WHERE status IN ('completed', 'failed') AND reported_at IS NULL
	`)
	if err != nil {
		return nil, fmt.Errorf("mysql: query unreported terminal runs: %w", err)
	}
	defer rows.Close()

	out := []domain.CoordinatorRun{}
	for rows.Next() {
		run, err := scanCoordinatorRun(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan unreported terminal run: %w", err)
		}
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate unreported terminal runs: %w", err)
	}
	return out, nil
}

func scanCoordinatorRun(row scannable) (domain.CoordinatorRun, error) {
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
	rows, err := r.db.QueryContext(ctx, `
		SELECT ot.id, ot.tenant_id, ot.coordinator_run_id, COALESCE(ot.parent_id, ''), COALESCE(ot.origin_task_id, ''),
		       ot.task_title, ot.spec, ot.status, ot.deps, ot.result, ot.created_at, ot.completed_at
		FROM orchestration_tasks ot
		WHERE ot.status = 'ready'
		  AND NOT EXISTS (
		    SELECT 1 FROM dispatch_contexts dc WHERE dc.orchestration_task_id = ot.id
		  )
	`)
	if err != nil {
		return nil, fmt.Errorf("mysql: query ready unclaimed tasks: %w", err)
	}
	defer rows.Close()

	out := []domain.OrchestrationTask{}
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan ready unclaimed task: %w", err)
		}
		out = append(out, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate ready unclaimed tasks: %w", err)
	}
	return out, nil
}

// ClaimReady is the CAS the tick loop relies on. MySQL has no
// `UPDATE ... RETURNING`, so this wraps the UPDATE in a transaction: the
// row lock InnoDB takes for the UPDATE's WHERE match is what makes
// "exactly one concurrent caller's UPDATE affects a row" hold, same
// guarantee a single-statement `UPDATE ... WHERE status='ready'` gives
// Postgres — only the affected-row-count check has to be done explicitly
// here instead of via RETURNING's presence/absence.
func (r *Repository) ClaimReady(ctx context.Context, tenantID, taskID string) (domain.OrchestrationTask, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.OrchestrationTask{}, false, fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		UPDATE orchestration_tasks
		SET status = 'dispatched'
		WHERE id = ? AND tenant_id = ? AND status = 'ready'
	`, taskID, tenantID)
	if err != nil {
		return domain.OrchestrationTask{}, false, fmt.Errorf("mysql: claim ready task: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return domain.OrchestrationTask{}, false, fmt.Errorf("mysql: claim ready task rows affected: %w", err)
	}
	if affected == 0 {
		return domain.OrchestrationTask{}, false, nil // lost the race — not an error
	}

	task, err := scanTask(tx.QueryRowContext(ctx, `
		SELECT id, tenant_id, coordinator_run_id, COALESCE(parent_id, ''), COALESCE(origin_task_id, ''),
		       task_title, spec, status, deps, result, created_at, completed_at
		FROM orchestration_tasks
		WHERE id = ? AND tenant_id = ?
	`, taskID, tenantID))
	if err != nil {
		return domain.OrchestrationTask{}, false, fmt.Errorf("mysql: reselect claimed task: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.OrchestrationTask{}, false, fmt.Errorf("mysql: commit claim tx: %w", err)
	}
	return task, true, nil
}

func (r *Repository) CountNonTerminalByRun(ctx context.Context, tenantID, coordinatorRunID string) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `
		SELECT count(*) FROM orchestration_tasks
		WHERE coordinator_run_id = ? AND tenant_id = ?
		  AND status NOT IN ('completed', 'failed')
	`, coordinatorRunID, tenantID).Scan(&count); err != nil {
		return 0, fmt.Errorf("mysql: count non-terminal tasks: %w", err)
	}
	return count, nil
}

// ---- common/outbox.Store -------------------------------------------------

func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("mysql: query unpublished outbox events: %w", err)
	}
	defer rows.Close()

	var out []outbox.Record
	for rows.Next() {
		var rec outbox.Record
		if err := rows.Scan(&rec.ID, &rec.Event.TenantID, &rec.Subject, &rec.Event.OccurredAt, &rec.Event.Version, &rec.Event.Payload); err != nil {
			return nil, fmt.Errorf("mysql: scan outbox event row: %w", err)
		}
		rec.Event.ID = rec.ID
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate outbox event rows: %w", err)
	}
	return out, nil
}

func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders, args := inClausePlaceholders(ids)
	query := fmt.Sprintf(`UPDATE outbox_events SET published_at = NOW(6) WHERE id IN (%s)`, placeholders)
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("mysql: mark outbox events published: %w", err)
	}
	return nil
}

// ---- helpers -------------------------------------------------------------

// nullableString returns nil for an empty string, non-nil otherwise —
// database/sql's default parameter converter dereferences a non-nil *string
// and maps a nil *string to SQL NULL, giving the same effect Postgres's
// NULLIF($n,”)/explicit-*string-arg pattern has for a nullable TEXT/UUID
// column, without needing NULLIF's own SQL-side type-inference workaround
// (see internal/adapter/postgres/repository.go's CreateDispatchContext doc
// comment for why Postgres needed that workaround at all — MySQL's CHAR(36)
// columns have no equivalent uuid-vs-text bind-type mismatch).
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// inClausePlaceholders builds a MySQL "?,?,...,?" placeholder list plus its
// matching args — MySQL has no `id = ANY($1)` array predicate, so every
// dynamic IN(...) call site in this file (promoteReadySiblings,
// MarkPublished) needs this instead.
func inClausePlaceholders(ids []string) (string, []any) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	return placeholders, args
}
