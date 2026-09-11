// Package mysql implements automation-service's AutomationRepository and
// AutomationRunRepository ports (defined in internal/usecase) against
// MySQL/TiDB via database/sql + github.com/go-sql-driver/mysql — the
// multi-database rollout's translation of internal/adapter/postgres,
// following the pattern BE-DB-SOL-001/002 (usage-service, the pilot) and
// BE-DB-SOL-005 (annotation-service) established. See
// specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-012.md for
// this service's own audit/deviations.
package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/automation-service/internal/adapter/eventbus"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
	"github.com/stablyai/orca-go/services/automation-service/internal/usecase"
)

// ── actions_json / action_results_json (de)serialization — mirrors
// internal/adapter/postgres's actionRow/actionResultRow 1:1 (same JSON wire
// shape, matching automation.proto's snake_case field names). Kept as a
// separate copy in this package rather than shared, per this codebase's
// one-package-per-service-adapter convention (no cross-adapter imports).

type actionRow struct {
	ID                string `json:"id"`
	Type              string `json:"type"`
	ConfigJSON        string `json:"config_json"`
	ContinueOnFailure bool   `json:"continue_on_failure"`
}

type actionResultRow struct {
	ActionID   string `json:"action_id"`
	Status     string `json:"status"`
	OutputJSON string `json:"output_json"`
	Error      string `json:"error"`
}

func marshalActions(actions []domain.AutomationAction) ([]byte, error) {
	rows := make([]actionRow, len(actions))
	for i, a := range actions {
		rows[i] = actionRow{ID: a.ID, Type: string(a.Type), ConfigJSON: a.ConfigJSON, ContinueOnFailure: a.ContinueOnFailure}
	}
	return json.Marshal(rows)
}

func unmarshalActions(raw []byte) ([]domain.AutomationAction, error) {
	var rows []actionRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	out := make([]domain.AutomationAction, len(rows))
	for i, r := range rows {
		out[i] = domain.AutomationAction{ID: r.ID, Type: domain.AutomationActionType(r.Type), ConfigJSON: r.ConfigJSON, ContinueOnFailure: r.ContinueOnFailure}
	}
	return out, nil
}

func marshalActionResults(results []domain.ActionResult) ([]byte, error) {
	rows := make([]actionResultRow, len(results))
	for i, r := range results {
		rows[i] = actionResultRow{ActionID: r.ActionID, Status: r.Status, OutputJSON: r.OutputJSON, Error: r.Error}
	}
	return json.Marshal(rows)
}

func unmarshalActionResults(raw []byte) ([]domain.ActionResult, error) {
	var rows []actionResultRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	out := make([]domain.ActionResult, len(rows))
	for i, r := range rows {
		out[i] = domain.ActionResult{ActionID: r.ActionID, Status: r.Status, OutputJSON: r.OutputJSON, Error: r.Error}
	}
	return out, nil
}

// AutomationRepository implements usecase.AutomationRepository (and
// usecase.DueAutomationClaimer's ClaimDue) against MySQL/TiDB via
// database/sql. No RLS equivalent exists in MySQL — every query below
// filters by tenant_id explicitly, which is the ONLY tenant-isolation
// enforcement for this adapter (see TASK-BE-DB-003's finding, true here for
// the same reason it was true for usage-service/annotation-service: no Go
// code anywhere calls SET LOCAL app.tenant_id, so the Postgres adapter's
// RLS policy never actually activated either — this doesn't lower the bar,
// it just doesn't add a backstop that was never real).
type AutomationRepository struct {
	db *sql.DB
}

func NewAutomationRepository(db *sql.DB) *AutomationRepository {
	return &AutomationRepository{db: db}
}

const automationColumns = `id, tenant_id, project_id, name, rrule, dtstart, step_type, step_config_json,
	actions_json, enabled, timezone, trigger_type, trigger_event, trigger_filter_json,
	max_run_history, run_timeout_seconds, running_run_id, running_since,
	next_run_at, created_at, updated_at`

func (r *AutomationRepository) Create(ctx context.Context, a domain.Automation) error {
	actionsJSON, err := marshalActions(a.Actions)
	if err != nil {
		return fmt.Errorf("mysql: marshal actions: %w", err)
	}
	filterJSON, err := marshalTriggerFilter(a.TriggerFilter)
	if err != nil {
		return fmt.Errorf("mysql: marshal trigger filter: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO automations (
			id, tenant_id, project_id, name, rrule, dtstart, step_type, step_config_json,
			actions_json, enabled, timezone, trigger_type, trigger_event, trigger_filter_json,
			max_run_history, run_timeout_seconds,
			next_run_at, created_at, updated_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`,
		a.ID, a.TenantID, nullableString(a.ProjectID), a.Name, a.RRule, a.DTStart, string(a.StepType), a.StepConfigJSON,
		actionsJSON, a.Enabled, a.Timezone, string(a.TriggerType), nullableString(string(a.TriggerEvent)), filterJSON,
		a.MaxRunHistory, a.RunTimeoutSeconds,
		nullableTime(a.NextRunAt), a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("mysql: insert automation: %w", err)
	}
	return nil
}

func (r *AutomationRepository) Get(ctx context.Context, tenantID, id string) (domain.Automation, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+automationColumns+`
		FROM automations
		WHERE tenant_id = ? AND id = ?
	`, tenantID, id)

	a, err := scanAutomation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Automation{}, fmt.Errorf("mysql: automation %s not found: %w", id, err)
	}
	if err != nil {
		return domain.Automation{}, fmt.Errorf("mysql: query automation: %w", err)
	}
	return a, nil
}

// List returns tenantID's automations, cursor-paginated by id. Unlike the
// Postgres adapter's `$2::uuid` cast (needed because Postgres's typed uuid
// column rejects an empty-string bind), MySQL's CHAR(36) id column accepts
// an empty-string comparison without error, so no cast/guard is needed here.
func (r *AutomationRepository) List(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Automation, string, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+automationColumns+`
		FROM automations
		WHERE tenant_id = ? AND (? = '' OR id > ?)
		ORDER BY id
		LIMIT ?
	`, tenantID, pageToken, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: query automations: %w", err)
	}
	defer rows.Close()

	var out []domain.Automation
	for rows.Next() {
		a, err := scanAutomation(rows)
		if err != nil {
			return nil, "", fmt.Errorf("mysql: scan automation row: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate automation rows: %w", err)
	}
	nextToken := ""
	if int32(len(out)) == pageSize {
		nextToken = out[len(out)-1].ID
	}
	return out, nextToken, nil
}

// Update persists a full row replace of the caller-merged Automation, same
// as internal/adapter/postgres.AutomationRepository.Update. RowsAffected()
// deciding not-found relies on this package's connection DSN carrying
// clientFoundRows=true (see cmd/server/main.go's toMySQLDriverDSN) — without
// it, go-sql-driver/mysql's default RowsAffected() counts only rows whose
// VALUES actually changed, so a no-op retry (identical field values) would
// be misreported as "not found" (the exact pitfall BE-DB-SOL-005 §3.1
// documents for annotation-service's UpdateAnnotation, fixed there with an
// extra SELECT instead). clientFoundRows restores Postgres's "matched rows"
// semantics at the driver level for the whole connection, so this method
// (and UpdateStatus below, and every other UPDATE in this adapter) needs no
// per-call workaround; verified against real MySQL 8 in
// TestAutomationRepository_Update_NoopRetryStillSucceeds.
func (r *AutomationRepository) Update(ctx context.Context, tenantID string, a domain.Automation) error {
	actionsJSON, err := marshalActions(a.Actions)
	if err != nil {
		return fmt.Errorf("mysql: marshal actions: %w", err)
	}
	filterJSON, err := marshalTriggerFilter(a.TriggerFilter)
	if err != nil {
		return fmt.Errorf("mysql: marshal trigger filter: %w", err)
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE automations
		SET name = ?, rrule = ?, step_type = ?, step_config_json = ?,
		    enabled = ?, timezone = ?, dtstart = ?, project_id = ?, actions_json = ?,
		    trigger_type = ?, trigger_event = ?, trigger_filter_json = ?,
		    max_run_history = ?, run_timeout_seconds = ?, updated_at = NOW(6)
		WHERE tenant_id = ? AND id = ?
	`, a.Name, a.RRule, string(a.StepType), a.StepConfigJSON, a.Enabled, a.Timezone, a.DTStart,
		nullableString(a.ProjectID), actionsJSON, string(a.TriggerType), nullableString(string(a.TriggerEvent)), filterJSON,
		a.MaxRunHistory, a.RunTimeoutSeconds, tenantID, a.ID)
	if err != nil {
		return fmt.Errorf("mysql: update automation: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("mysql: automation %s not found for tenant %s", a.ID, tenantID)
	}
	return nil
}

// Delete removes an automation. automation_runs.automation_id has
// ON DELETE CASCADE (migrations/mysql/0001_init.up.sql's fk_automation_runs_automation),
// so run rows referencing this automation are removed by MySQL itself — no
// separate cleanup step. DELETE's RowsAffected() always counts
// WHERE-matched rows on every dialect (no "changed value" ambiguity exists
// for DELETE), so this needs no clientFoundRows-dependent reasoning, unlike
// Update above.
func (r *AutomationRepository) Delete(ctx context.Context, tenantID, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM automations WHERE tenant_id = ? AND id = ?`, tenantID, id)
	if err != nil {
		return fmt.Errorf("mysql: delete automation: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("mysql: automation %s not found for tenant %s", id, tenantID)
	}
	return nil
}

// CountByProject returns the number of automations for tenantID scoped to
// projectID — backs BR-AT-02's per-project cap.
func (r *AutomationRepository) CountByProject(ctx context.Context, tenantID, projectID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM automations WHERE tenant_id = ? AND project_id = ?`,
		tenantID, projectID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("mysql: count automations by project: %w", err)
	}
	return count, nil
}

// ListByTrigger returns enabled automations for tenantID whose trigger_type
// is 'event' and trigger_event matches eventName — backs event dispatch.
func (r *AutomationRepository) ListByTrigger(ctx context.Context, tenantID string, eventName domain.EventName) ([]domain.Automation, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+automationColumns+`
		FROM automations
		WHERE tenant_id = ? AND trigger_type = 'event' AND trigger_event = ? AND enabled = true
	`, tenantID, string(eventName))
	if err != nil {
		return nil, fmt.Errorf("mysql: query automations by trigger: %w", err)
	}
	defer rows.Close()

	var out []domain.Automation
	for rows.Next() {
		a, err := scanAutomation(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan automation row: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate automation rows: %w", err)
	}
	return out, nil
}

// ListEventTriggered returns every event-triggered automation for tenantID
// (regardless of enabled) — backs DetectTriggerCycle's graph build.
func (r *AutomationRepository) ListEventTriggered(ctx context.Context, tenantID string) ([]domain.Automation, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+automationColumns+`
		FROM automations
		WHERE tenant_id = ? AND trigger_type = 'event'
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query event-triggered automations: %w", err)
	}
	defer rows.Close()

	var out []domain.Automation
	for rows.Next() {
		a, err := scanAutomation(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan automation row: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate automation rows: %w", err)
	}
	return out, nil
}

// AcquireRunLock implements usecase.AutomationRepository.AcquireRunLock — a
// single conditional UPDATE (no separate SELECT+UPDATE) so the check and
// claim are atomic under concurrent callers, mirroring
// internal/adapter/postgres's same method. Postgres's
// `now() - make_interval(secs => $4)` becomes MySQL's
// `DATE_SUB(NOW(6), INTERVAL ? MICROSECOND)` — MICROSECOND (not SECOND) is
// used deliberately so ttl's sub-second precision survives the int64 bind
// without float-to-SQL-literal rounding ambiguity.
func (r *AutomationRepository) AcquireRunLock(ctx context.Context, tenantID, automationID, runID string, ttl time.Duration) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE automations
		SET running_run_id = ?, running_since = NOW(6)
		WHERE tenant_id = ? AND id = ?
		  AND (running_run_id IS NULL OR running_since < DATE_SUB(NOW(6), INTERVAL ? MICROSECOND))
	`, runID, tenantID, automationID, ttl.Microseconds())
	if err != nil {
		return false, fmt.Errorf("mysql: acquire run lock: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("mysql: rows affected: %w", err)
	}
	return affected > 0, nil
}

// ReleaseRunLock implements usecase.AutomationRepository.ReleaseRunLock —
// the `running_run_id = ?` guard means a lock already reclaimed by a newer
// run (past its TTL) is left alone, not clobbered by a late release from the
// run that used to hold it.
func (r *AutomationRepository) ReleaseRunLock(ctx context.Context, tenantID, automationID, runID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE automations
		SET running_run_id = NULL, running_since = NULL
		WHERE tenant_id = ? AND id = ? AND running_run_id = ?
	`, tenantID, automationID, runID)
	if err != nil {
		return fmt.Errorf("mysql: release run lock: %w", err)
	}
	return nil
}

// ClaimDue implements usecase.DueAutomationClaimer — see that port's doc
// comment for why the returned batch's transaction stays open across
// dispatch. `FOR UPDATE SKIP LOCKED` is supported by InnoDB since MySQL
// 8.0 (confirmed against the mysql:8 image this task's integration tests
// run on — see BE-DB-SOL-012 §Kết quả thực tế), so this translates
// verbatim from the Postgres query, only the placeholder style changes.
func (r *AutomationRepository) ClaimDue(ctx context.Context, now time.Time, limit int32) (usecase.ClaimedBatch, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mysql: begin claim tx: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT `+automationColumns+`
		FROM automations
		WHERE enabled = true AND next_run_at IS NOT NULL AND next_run_at <= ?
		ORDER BY next_run_at
		LIMIT ?
		FOR UPDATE SKIP LOCKED
	`, now, limit)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("mysql: query due automations: %w", err)
	}

	var claimed []domain.Automation
	for rows.Next() {
		a, err := scanAutomation(rows)
		if err != nil {
			rows.Close()
			_ = tx.Rollback()
			return nil, fmt.Errorf("mysql: scan due automation row: %w", err)
		}
		claimed = append(claimed, a)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("mysql: iterate due automation rows: %w", rowsErr)
	}

	return &claimedBatch{tx: tx, automations: claimed}, nil
}

// claimedBatch implements usecase.ClaimedBatch by wrapping the still-open
// *sql.Tx ClaimDue began — the database/sql equivalent of
// internal/adapter/postgres's pgx.Tx-backed claimedBatch.
type claimedBatch struct {
	tx          *sql.Tx
	automations []domain.Automation
}

func (b *claimedBatch) Automations() []domain.Automation { return b.automations }

func (b *claimedBatch) Advance(ctx context.Context, automationID string, nextRunAt time.Time, hasNext bool) error {
	var next any
	if hasNext {
		next = nextRunAt
	}
	if _, err := b.tx.ExecContext(ctx, `
		UPDATE automations SET next_run_at = ?, updated_at = NOW(6) WHERE id = ?
	`, next, automationID); err != nil {
		return fmt.Errorf("mysql: advance next_run_at for automation %s: %w", automationID, err)
	}
	return nil
}

func (b *claimedBatch) Commit(ctx context.Context) error {
	if err := b.tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit claim batch: %w", err)
	}
	return nil
}

func (b *claimedBatch) Rollback(ctx context.Context) error {
	if err := b.tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return fmt.Errorf("mysql: rollback claim batch: %w", err)
	}
	return nil
}

// scanAutomation scans a row selected via automationColumns — column order
// must match that constant exactly. Nullable columns scan into pointer
// locals the same way internal/adapter/postgres's scanAutomation does;
// database/sql's convertAssign supports the same **T nullable-scan pattern
// pgx uses, so this translates without behavior change.
func scanAutomation(row rowScanner) (domain.Automation, error) {
	var a domain.Automation
	var stepType, triggerType string
	var projectID, triggerEvent, triggerFilterJSON *string
	var actionsJSON []byte
	var runningRunID *string
	var runningSince *time.Time
	var nextRunAt *time.Time
	if err := row.Scan(
		&a.ID, &a.TenantID, &projectID, &a.Name, &a.RRule, &a.DTStart, &stepType, &a.StepConfigJSON,
		&actionsJSON, &a.Enabled, &a.Timezone, &triggerType, &triggerEvent, &triggerFilterJSON,
		&a.MaxRunHistory, &a.RunTimeoutSeconds, &runningRunID, &runningSince,
		&nextRunAt, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return domain.Automation{}, err
	}
	a.StepType = domain.StepType(stepType)
	if projectID != nil {
		a.ProjectID = *projectID
	}
	actions, err := unmarshalActions(actionsJSON)
	if err != nil {
		return domain.Automation{}, fmt.Errorf("mysql: unmarshal actions_json: %w", err)
	}
	a.Actions = actions
	a.TriggerType = domain.TriggerType(triggerType)
	if triggerEvent != nil {
		a.TriggerEvent = domain.EventName(*triggerEvent)
	}
	if triggerFilterJSON != nil {
		filter, err := domain.ParseTriggerFilter(*triggerFilterJSON)
		if err != nil {
			return domain.Automation{}, fmt.Errorf("mysql: unmarshal trigger_filter_json: %w", err)
		}
		a.TriggerFilter = filter
	}
	if runningRunID != nil {
		a.RunningRunID = *runningRunID
	}
	if runningSince != nil {
		a.RunningSince = *runningSince
	}
	if nextRunAt != nil {
		a.NextRunAt = *nextRunAt
	}
	return a, nil
}

func marshalTriggerFilter(f *domain.TriggerFilter) (*string, error) {
	if f == nil {
		return nil, nil
	}
	b, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	s := string(b)
	return &s, nil
}

// AutomationRunRepository implements usecase.AutomationRunRepository and
// common/outbox.Store against MySQL/TiDB.
type AutomationRunRepository struct {
	db *sql.DB
}

// NewAutomationRunRepository takes no publisher argument, unlike
// internal/adapter/postgres.NewAutomationRunRepository: that constructor's
// publisher (internal/adapter/eventbus.RunCompletedPublisher) writes the
// run-completed outbox row via a hard-coded pgx.Tx parameter
// (PublishRunCompleted(ctx, tx pgx.Tx, run)) — Postgres-specific, and not
// something this MySQL adapter's *sql.Tx can satisfy. Rather than change
// that shared eventbus package (out of scope: BE-DB-SOL-012 doesn't touch
// files outside automation-service's DB layer, and the publisher's
// pgx.Tx-typed signature is used correctly by the one caller that has a
// pgx.Tx — this is a real but pre-existing dialect coupling in
// automation-service's own eventbus package, flagged not fixed, see
// BE-DB-SOL-012 §3), UpdateStatus below writes the equivalent outbox row
// directly against its own *sql.Tx, matching runCompletedPayload's wire
// shape and eventbus.RunCompletedSubject exactly so downstream consumers
// (notification-service) see identical events on either dialect.
func NewAutomationRunRepository(db *sql.DB) *AutomationRunRepository {
	return &AutomationRunRepository{db: db}
}

func (r *AutomationRunRepository) Create(ctx context.Context, run domain.AutomationRun) error {
	actionResultsJSON, err := marshalActionResults(run.ActionResults)
	if err != nil {
		return fmt.Errorf("mysql: marshal action_results: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO automation_runs (
			id, automation_id, tenant_id, request_id, status, step_type, `+"`trigger`"+`, step_config_json,
			output_json, error_message, action_results_json, created_at, started_at, completed_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`,
		run.ID, run.AutomationID, run.TenantID, run.RequestID, string(run.Status), string(run.StepType), string(run.Trigger), run.StepConfigJSON,
		nullableString(run.OutputJSON), nullableString(run.ErrorMessage), actionResultsJSON, run.CreatedAt, nullableTime(run.StartedAt), nullableTime(run.CompletedAt),
	)
	if err != nil {
		return fmt.Errorf("mysql: insert automation run: %w", err)
	}
	return nil
}

func (r *AutomationRunRepository) FindByRequestID(ctx context.Context, tenantID, automationID, requestID string) (domain.AutomationRun, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+runColumns+`
		FROM automation_runs
		WHERE tenant_id = ? AND automation_id = ? AND request_id = ?
	`, tenantID, automationID, requestID)

	run, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AutomationRun{}, false, nil
	}
	if err != nil {
		return domain.AutomationRun{}, false, fmt.Errorf("mysql: query automation run by request_id: %w", err)
	}
	return run, true, nil
}

// FindRunning returns the currently-running run for automationID, if any.
func (r *AutomationRunRepository) FindRunning(ctx context.Context, tenantID, automationID string) (domain.AutomationRun, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+runColumns+`
		FROM automation_runs
		WHERE tenant_id = ? AND automation_id = ? AND status = 'running'
		LIMIT 1
	`, tenantID, automationID)

	run, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AutomationRun{}, false, nil
	}
	if err != nil {
		return domain.AutomationRun{}, false, fmt.Errorf("mysql: query running automation run: %w", err)
	}
	return run, true, nil
}

// UpdateStatus persists a run's status transition inside a transaction —
// for a terminal transition (Terminal() == true), the same transaction also
// writes the orca.automation.run.completed outbox entry directly (see
// NewAutomationRunRepository's doc comment for why this doesn't go through
// internal/adapter/eventbus.RunCompletedPublisher). A second 'running' row
// for the same automation collides with migrations/mysql/
// 0004_one_running_run.up.sql's idx_automation_runs_one_running — the
// application-maintained running_slot column this method computes below —
// exactly like Postgres's partial unique index; isDuplicateKeyError maps
// that MySQL 1062 error to the same usecase.ErrConcurrentRunActive
// sentinel the Postgres adapter returns, so usecase/run_now.go's error
// handling is dialect-agnostic.
func (r *AutomationRunRepository) UpdateStatus(ctx context.Context, run domain.AutomationRun) error {
	actionResultsJSON, err := marshalActionResults(run.ActionResults)
	if err != nil {
		return fmt.Errorf("mysql: marshal action_results: %w", err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin update-status tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after a successful Commit

	// running_slot mirrors migrations/mysql/0004_one_running_run.up.sql's
	// doc comment: it's automation_id while status='running' (so 2
	// concurrently-running rows for the same automation collide on the
	// UNIQUE INDEX), NULL otherwise (so any number of non-running rows
	// coexist — MySQL's UNIQUE INDEX never treats 2 NULLs as equal). This
	// is computed here rather than by a GENERATED ALWAYS AS column because
	// InnoDB refuses to add a STORED generated column whose expression
	// references a column that also carries a FOREIGN KEY constraint
	// (automation_id → automations.id) — see that migration's comment for
	// the real "Error 1215" this app-level approach avoids.
	var runningSlot any
	if run.Status == domain.RunStatusRunning {
		runningSlot = run.AutomationID
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE automation_runs
		SET status = ?, output_json = ?, error_message = ?, action_results_json = ?, started_at = ?, completed_at = ?, running_slot = ?
		WHERE id = ? AND tenant_id = ?
	`, string(run.Status), nullableString(run.OutputJSON), nullableString(run.ErrorMessage), actionResultsJSON,
		nullableTime(run.StartedAt), nullableTime(run.CompletedAt), runningSlot, run.ID, run.TenantID)
	if err != nil {
		if isDuplicateKeyError(err, "idx_automation_runs_one_running") {
			return usecase.ErrConcurrentRunActive
		}
		return fmt.Errorf("mysql: update automation run status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("mysql: automation run %s not found for tenant %s", run.ID, run.TenantID)
	}

	if run.Status.Terminal() {
		payload, err := json.Marshal(runCompletedPayload{AutomationID: run.AutomationID, RunID: run.ID, Status: string(run.Status)})
		if err != nil {
			return fmt.Errorf("mysql: marshal run-completed payload: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES (?, ?, ?, ?, 1, ?)
		`, uuid.NewString(), run.TenantID, eventbus.RunCompletedSubject, time.Now().UTC(), payload); err != nil {
			return fmt.Errorf("mysql: insert run-completed outbox entry: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit update-status tx: %w", err)
	}
	return nil
}

// runCompletedPayload mirrors internal/adapter/eventbus's unexported
// runCompletedPayload exactly (field-for-field, same json tags) so this
// adapter's outbox rows are wire-identical to the Postgres adapter's,
// regardless of which dialect wrote them — a consumer (notification-service)
// must not be able to tell which dialect produced a given event.
type runCompletedPayload struct {
	AutomationID string `json:"automation_id"`
	RunID        string `json:"run_id"`
	Status       string `json:"status"`
}

func (r *AutomationRunRepository) ListByAutomation(ctx context.Context, tenantID, automationID, pageToken string, pageSize int32) ([]domain.AutomationRun, string, error) {
	// automationID/pageToken are legitimately empty (see
	// internal/adapter/postgres's ListByAutomation doc comment) — MySQL's
	// CHAR(36) columns accept an empty-string comparison without error
	// (unlike Postgres's uuid-typed columns), so no `::uuid`-cast guard is
	// needed here.
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+runColumns+`
		FROM automation_runs
		WHERE tenant_id = ?
		  AND (? = '' OR automation_id = ?)
		  AND (? = '' OR id > ?)
		ORDER BY id
		LIMIT ?
	`, tenantID, automationID, automationID, pageToken, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: query automation runs: %w", err)
	}
	defer rows.Close()

	var out []domain.AutomationRun
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, "", fmt.Errorf("mysql: scan automation run row: %w", err)
		}
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate automation run rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

// PruneOldRuns deletes every automation_runs row for automationID beyond
// the `keep` most recent (by created_at DESC) — BR-AT-07. The subquery is
// wrapped in an extra derived table (`AS keep`) because MySQL forbids
// selecting directly from the table a DELETE targets within that DELETE's
// own subquery ("You can't specify target table ... for update in FROM
// clause") — wrapping it in a materialized derived table is the standard
// MySQL workaround; Postgres has no such restriction, so its version
// selects straight from automation.automation_runs.
func (r *AutomationRunRepository) PruneOldRuns(ctx context.Context, tenantID, automationID string, keep int) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM automation_runs
		WHERE tenant_id = ? AND automation_id = ?
		  AND id NOT IN (
		    SELECT id FROM (
		      SELECT id FROM automation_runs
		      WHERE tenant_id = ? AND automation_id = ?
		      ORDER BY created_at DESC
		      LIMIT ?
		    ) AS keep
		  )`,
		tenantID, automationID, tenantID, automationID, keep,
	)
	if err != nil {
		return fmt.Errorf("mysql: prune old automation runs: %w", err)
	}
	return nil
}

// WriteCleanupReport persists one worktree_cleanup_log row per entry —
// BR-AT-14's per-worktree, per-reason audit trail. Unlike Postgres (`id`
// defaults to gen_random_uuid()), migrations/mysql/
// 0006_worktree_cleanup_log.up.sql's id column has no default (see this
// service's migrations/mysql/0001_init.up.sql's doc comment on why —
// application-generated ids only), so this adapter generates each row's id
// in Go via uuid.NewString(), same convention run_now.go/
// create_automation.go already use.
func (r *AutomationRunRepository) WriteCleanupReport(ctx context.Context, tenantID, runID string, entries []domain.CleanupLogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString(`INSERT INTO worktree_cleanup_log (id, tenant_id, run_id, worktree_id, action, reason) VALUES `)
	args := make([]any, 0, len(entries)*6)
	for i, e := range entries {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("(?,?,?,?,?,?)")
		args = append(args, uuid.NewString(), tenantID, runID, e.WorktreeID, e.Action, nullableString(e.Reason))
	}
	if _, err := r.db.ExecContext(ctx, sb.String(), args...); err != nil {
		return fmt.Errorf("mysql: insert worktree_cleanup_log rows: %w", err)
	}
	return nil
}

// `trigger` is backtick-quoted here (and in Create's INSERT above) because
// TRIGGER is a reserved keyword in MySQL — unlike Postgres, which accepts
// it unquoted as a plain identifier (internal/adapter/postgres's runColumns
// uses it bare); confirmed by a real MySQL 1064 syntax error against
// mysql:8 before this fix (see BE-DB-SOL-012 §Kết quả thực tế).
const runColumns = "id, automation_id, tenant_id, request_id, status, step_type, `trigger`, step_config_json," +
	"output_json, error_message, action_results_json, created_at, started_at, completed_at"

// FetchUnpublished and MarkPublished implement common/outbox.Store — polled
// by the common/outbox.Relay wired in cmd/server/main.go, mirroring
// internal/adapter/postgres's same-named methods and usage-service's mysql
// adapter's MarkPublished (dynamic `IN (?,...)` build — MySQL has no
// `= ANY($1)` equivalent).
func (r *AutomationRunRepository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
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
		var payload []byte
		if err := rows.Scan(&rec.ID, &rec.Event.TenantID, &rec.Subject, &rec.Event.OccurredAt, &rec.Event.Version, &payload); err != nil {
			return nil, fmt.Errorf("mysql: scan outbox event row: %w", err)
		}
		rec.Event.ID = rec.ID
		rec.Event.Payload = payload
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate outbox event rows: %w", err)
	}
	return out, nil
}

func (r *AutomationRunRepository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	query := fmt.Sprintf(`UPDATE outbox_events SET published_at = NOW(6) WHERE id IN (%s)`, placeholders)
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("mysql: mark outbox events published: %w", err)
	}
	return nil
}

// PruneRuns implements usecase.AutomationRunRepository.PruneRuns — same
// derived-table wrapping as PruneOldRuns above, for the same MySQL
// self-reference restriction.
func (r *AutomationRunRepository) PruneRuns(ctx context.Context, tenantID, automationID string, maxRuns int32) error {
	if maxRuns <= 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM automation_runs
		WHERE tenant_id = ? AND automation_id = ? AND id NOT IN (
			SELECT id FROM (
				SELECT id FROM automation_runs
				WHERE tenant_id = ? AND automation_id = ?
				ORDER BY created_at DESC
				LIMIT ?
			) AS keep
		)
	`, tenantID, automationID, tenantID, automationID, maxRuns)
	if err != nil {
		return fmt.Errorf("mysql: prune automation runs: %w", err)
	}
	return nil
}

// rowScanner abstracts over *sql.Row and *sql.Rows, which share the same
// Scan signature — lets scanRun/scanAutomation serve both single-row and
// multi-row callers without duplicating the column list, mirroring
// internal/adapter/postgres's identical rowScanner.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanRun scans a row selected via runColumns — column order must match
// that constant exactly.
func scanRun(row rowScanner) (domain.AutomationRun, error) {
	var run domain.AutomationRun
	var status, stepType, trigger string
	var outputJSON, errorMessage *string
	var actionResultsJSON []byte
	var startedAt, completedAt *time.Time
	if err := row.Scan(
		&run.ID, &run.AutomationID, &run.TenantID, &run.RequestID, &status, &stepType, &trigger, &run.StepConfigJSON,
		&outputJSON, &errorMessage, &actionResultsJSON, &run.CreatedAt, &startedAt, &completedAt,
	); err != nil {
		return domain.AutomationRun{}, err
	}
	run.Status = domain.RunStatus(status)
	run.StepType = domain.StepType(stepType)
	run.Trigger = domain.RunTrigger(trigger)
	if outputJSON != nil {
		run.OutputJSON = *outputJSON
	}
	if errorMessage != nil {
		run.ErrorMessage = *errorMessage
	}
	results, err := unmarshalActionResults(actionResultsJSON)
	if err != nil {
		return domain.AutomationRun{}, fmt.Errorf("mysql: unmarshal action_results_json: %w", err)
	}
	run.ActionResults = results
	if startedAt != nil {
		run.StartedAt = *startedAt
	}
	if completedAt != nil {
		run.CompletedAt = *completedAt
	}
	return run, nil
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// isDuplicateKeyError reports whether err is a MySQL duplicate-entry error
// (1062) on the named unique index — used to distinguish "a concurrent
// dispatch already claimed this" (idx_automation_runs_one_running,
// BR-AT-08) from a real failure, mirroring
// internal/adapter/postgres.isUniqueViolation's role for Postgres's 23505.
// Unlike pgconn.PgError (which carries a dedicated ConstraintName field),
// go-sql-driver/mysql's *mysqldriver.MySQLError only has a free-text
// Message ("Duplicate entry '...' for key '<index>'" — the exact key
// naming varies slightly by MySQL version, e.g. with or without a
// table-name prefix), so this matches by substring rather than equality.
func isDuplicateKeyError(err error, indexName string) bool {
	var myErr *mysqldriver.MySQLError
	if !errors.As(err, &myErr) {
		return false
	}
	if myErr.Number != 1062 {
		return false
	}
	return indexName == "" || strings.Contains(myErr.Message, indexName)
}
