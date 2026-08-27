// Package postgres implements automation-service's AutomationRepository and
// AutomationRunRepository ports (defined in internal/usecase) against this
// service's own PostgreSQL database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule: this is the ONLY package in automation-service
// that knows SQL exists.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/automation-service/internal/adapter/eventbus"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
	"github.com/stablyai/orca-go/services/automation-service/internal/usecase"
)

// AutomationRepository implements usecase.AutomationRepository against
// Postgres via pgx — hand-written SQL (see architecture/04-tech-stack.md:
// sqlc codegen is the eventual target, this scaffold hand-writes the
// equivalent queries directly to avoid a build-time dependency on the sqlc
// binary, matching usage-service's reference pattern).
type AutomationRepository struct {
	pool *pgxpool.Pool
}

func NewAutomationRepository(pool *pgxpool.Pool) *AutomationRepository {
	return &AutomationRepository{pool: pool}
}

const automationColumns = `id, tenant_id, project_id, name, rrule, dtstart, step_type, step_config_json,
	actions_json, enabled, timezone, trigger_type, trigger_event, trigger_filter_json,
	next_run_at, created_at, updated_at`

func (r *AutomationRepository) Create(ctx context.Context, a domain.Automation) error {
	actionsJSON, err := marshalActions(a.Actions)
	if err != nil {
		return fmt.Errorf("postgres: marshal actions: %w", err)
	}
	filterJSON, err := marshalTriggerFilter(a.TriggerFilter)
	if err != nil {
		return fmt.Errorf("postgres: marshal trigger filter: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO automation.automations (
			id, tenant_id, project_id, name, rrule, dtstart, step_type, step_config_json,
			actions_json, enabled, timezone, trigger_type, trigger_event, trigger_filter_json,
			next_run_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
	`,
		a.ID, a.TenantID, nullableString(a.ProjectID), a.Name, a.RRule, a.DTStart, string(a.StepType), a.StepConfigJSON,
		actionsJSON, a.Enabled, a.Timezone, string(a.TriggerType), nullableString(string(a.TriggerEvent)), filterJSON,
		nullableTime(a.NextRunAt), a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: insert automation: %w", err)
	}
	return nil
}

func (r *AutomationRepository) Get(ctx context.Context, tenantID, id string) (domain.Automation, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+automationColumns+`
		FROM automation.automations
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	a, err := scanAutomation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Automation{}, fmt.Errorf("postgres: automation %s not found: %w", id, err)
	}
	if err != nil {
		return domain.Automation{}, fmt.Errorf("postgres: query automation: %w", err)
	}
	return a, nil
}

// List returns tenantID's automations, cursor-paginated by id.
func (r *AutomationRepository) List(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Automation, string, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+automationColumns+`
		FROM automation.automations
		WHERE tenant_id = $1 AND ($2 = '' OR id > $2::uuid)
		ORDER BY id
		LIMIT $3
	`, tenantID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query automations: %w", err)
	}
	defer rows.Close()

	var out []domain.Automation
	for rows.Next() {
		a, err := scanAutomation(rows)
		if err != nil {
			return nil, "", fmt.Errorf("postgres: scan automation row: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate automation rows: %w", err)
	}
	nextToken := ""
	if int32(len(out)) == pageSize {
		nextToken = out[len(out)-1].ID
	}
	return out, nextToken, nil
}

// Update persists a full row replace of the caller-merged Automation
// (usecase.UpdateAutomation already merged unset fields from the current
// row before calling this) — scheduling fields (next_run_at) are
// deliberately left untouched by this statement.
func (r *AutomationRepository) Update(ctx context.Context, tenantID string, a domain.Automation) error {
	actionsJSON, err := marshalActions(a.Actions)
	if err != nil {
		return fmt.Errorf("postgres: marshal actions: %w", err)
	}
	filterJSON, err := marshalTriggerFilter(a.TriggerFilter)
	if err != nil {
		return fmt.Errorf("postgres: marshal trigger filter: %w", err)
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE automation.automations
		SET name = $3, rrule = $4, step_type = $5, step_config_json = $6,
		    enabled = $7, timezone = $8, dtstart = $9, project_id = $10, actions_json = $11,
		    trigger_type = $12, trigger_event = $13, trigger_filter_json = $14, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, a.ID, a.Name, a.RRule, string(a.StepType), a.StepConfigJSON, a.Enabled, a.Timezone, a.DTStart,
		nullableString(a.ProjectID), actionsJSON, string(a.TriggerType), nullableString(string(a.TriggerEvent)), filterJSON)
	if err != nil {
		return fmt.Errorf("postgres: update automation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: automation %s not found for tenant %s", a.ID, tenantID)
	}
	return nil
}

// Delete removes an automation. automation_runs.automation_id has
// ON DELETE CASCADE (migrations/0001_init.up.sql), so run rows referencing
// this automation are removed by Postgres itself — no separate cleanup step.
func (r *AutomationRepository) Delete(ctx context.Context, tenantID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM automation.automations WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("postgres: delete automation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: automation %s not found for tenant %s", id, tenantID)
	}
	return nil
}

// CountByProject returns the number of automations for tenantID scoped to
// projectID — backs BR-AT-02's per-project cap.
func (r *AutomationRepository) CountByProject(ctx context.Context, tenantID, projectID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM automation.automations WHERE tenant_id = $1 AND project_id = $2`,
		tenantID, projectID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("postgres: count automations by project: %w", err)
	}
	return count, nil
}

// ListByTrigger returns enabled automations for tenantID whose trigger_type
// is 'event' and trigger_event matches eventName — backs event dispatch.
func (r *AutomationRepository) ListByTrigger(ctx context.Context, tenantID string, eventName domain.EventName) ([]domain.Automation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+automationColumns+`
		FROM automation.automations
		WHERE tenant_id = $1 AND trigger_type = 'event' AND trigger_event = $2 AND enabled = true
	`, tenantID, string(eventName))
	if err != nil {
		return nil, fmt.Errorf("postgres: query automations by trigger: %w", err)
	}
	defer rows.Close()

	var out []domain.Automation
	for rows.Next() {
		a, err := scanAutomation(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan automation row: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate automation rows: %w", err)
	}
	return out, nil
}

// ListEventTriggered returns every event-triggered automation for tenantID
// (regardless of enabled) — backs DetectTriggerCycle's graph build
// (BR-AT-10): a disabled automation can still be re-enabled later, so it
// must still count as a node in the cycle graph.
func (r *AutomationRepository) ListEventTriggered(ctx context.Context, tenantID string) ([]domain.Automation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+automationColumns+`
		FROM automation.automations
		WHERE tenant_id = $1 AND trigger_type = 'event'
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query event-triggered automations: %w", err)
	}
	defer rows.Close()

	var out []domain.Automation
	for rows.Next() {
		a, err := scanAutomation(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan automation row: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate automation rows: %w", err)
	}
	return out, nil
}

// ClaimDue implements usecase.DueAutomationClaimer — see that port's doc
// comment for why the returned batch's transaction stays open across
// dispatch. The query intentionally has no tenant filter: the scheduler
// scans across every tenant on a timer, it is not a per-request caller with
// a single tenant to scope to (see automation-service.md §7's "every
// replica runs a ticker" model) — every row it returns still carries its
// own tenant_id, which RunNow scopes its own work to via context.
func (r *AutomationRepository) ClaimDue(ctx context.Context, now time.Time, limit int32) (usecase.ClaimedBatch, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: begin claim tx: %w", err)
	}

	rows, err := tx.Query(ctx, `
		SELECT `+automationColumns+`
		FROM automation.automations
		WHERE enabled = true AND next_run_at IS NOT NULL AND next_run_at <= $1
		ORDER BY next_run_at
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`, now, limit)
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("postgres: query due automations: %w", err)
	}

	var claimed []domain.Automation
	for rows.Next() {
		a, err := scanAutomation(rows)
		if err != nil {
			rows.Close()
			_ = tx.Rollback(ctx)
			return nil, fmt.Errorf("postgres: scan due automation row: %w", err)
		}
		claimed = append(claimed, a)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("postgres: iterate due automation rows: %w", rowsErr)
	}

	return &claimedBatch{tx: tx, automations: claimed}, nil
}

// claimedBatch implements usecase.ClaimedBatch by wrapping the still-open
// pgx.Tx ClaimDue began.
type claimedBatch struct {
	tx          pgx.Tx
	automations []domain.Automation
}

func (b *claimedBatch) Automations() []domain.Automation { return b.automations }

func (b *claimedBatch) Advance(ctx context.Context, automationID string, nextRunAt time.Time, hasNext bool) error {
	var next *time.Time
	if hasNext {
		next = &nextRunAt
	}
	if _, err := b.tx.Exec(ctx, `
		UPDATE automation.automations SET next_run_at = $1, updated_at = now() WHERE id = $2
	`, next, automationID); err != nil {
		return fmt.Errorf("postgres: advance next_run_at for automation %s: %w", automationID, err)
	}
	return nil
}

func (b *claimedBatch) Commit(ctx context.Context) error {
	if err := b.tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit claim batch: %w", err)
	}
	return nil
}

func (b *claimedBatch) Rollback(ctx context.Context) error {
	if err := b.tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return fmt.Errorf("postgres: rollback claim batch: %w", err)
	}
	return nil
}

func scanAutomation(row rowScanner) (domain.Automation, error) {
	var a domain.Automation
	var stepType, triggerType string
	var projectID, triggerEvent, triggerFilterJSON *string
	var actionsJSON string
	var nextRunAt *time.Time
	if err := row.Scan(
		&a.ID, &a.TenantID, &projectID, &a.Name, &a.RRule, &a.DTStart, &stepType, &a.StepConfigJSON,
		&actionsJSON, &a.Enabled, &a.Timezone, &triggerType, &triggerEvent, &triggerFilterJSON,
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
		return domain.Automation{}, fmt.Errorf("postgres: unmarshal actions_json: %w", err)
	}
	a.Actions = actions
	a.TriggerType = domain.TriggerType(triggerType)
	if triggerEvent != nil {
		a.TriggerEvent = domain.EventName(*triggerEvent)
	}
	if triggerFilterJSON != nil {
		filter, err := domain.ParseTriggerFilter(*triggerFilterJSON)
		if err != nil {
			return domain.Automation{}, fmt.Errorf("postgres: unmarshal trigger_filter_json: %w", err)
		}
		a.TriggerFilter = filter
	}
	if nextRunAt != nil {
		a.NextRunAt = *nextRunAt
	}
	return a, nil
}

// actionRow is actions_json's on-disk shape — kept separate from
// domain.AutomationAction so domain/ stays free of serialization concerns
// (architecture/03-clean-architecture-guidelines.md).
type actionRow struct {
	StepType       string `json:"step_type"`
	StepConfigJSON string `json:"step_config_json"`
	OnFailure      string `json:"on_failure"`
}

func marshalActions(actions []domain.AutomationAction) (string, error) {
	rows := make([]actionRow, len(actions))
	for i, a := range actions {
		rows[i] = actionRow{StepType: string(a.StepType), StepConfigJSON: a.StepConfigJSON, OnFailure: string(a.OnFailure)}
	}
	b, err := json.Marshal(rows)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func unmarshalActions(raw string) ([]domain.AutomationAction, error) {
	if raw == "" {
		return nil, nil
	}
	var rows []actionRow
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, err
	}
	out := make([]domain.AutomationAction, len(rows))
	for i, r := range rows {
		out[i] = domain.AutomationAction{StepType: domain.StepType(r.StepType), StepConfigJSON: r.StepConfigJSON, OnFailure: domain.OnFailurePolicy(r.OnFailure)}
	}
	return out, nil
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

// AutomationRunRepository implements usecase.AutomationRunRepository.
type AutomationRunRepository struct {
	pool      *pgxpool.Pool
	publisher *eventbus.RunCompletedPublisher
}

// NewAutomationRunRepository wires publisher — the transactional-outbox
// writer used inside UpdateStatus for terminal transitions (BR: never a
// bare post-hoc publish call). A nil publisher is accepted for tests/
// call-sites that don't need the outbox side effect.
func NewAutomationRunRepository(pool *pgxpool.Pool, publisher *eventbus.RunCompletedPublisher) *AutomationRunRepository {
	return &AutomationRunRepository{pool: pool, publisher: publisher}
}

func (r *AutomationRunRepository) Create(ctx context.Context, run domain.AutomationRun) error {
	actionResultsJSON, err := marshalActionResults(run.ActionResults)
	if err != nil {
		return fmt.Errorf("postgres: marshal action_results: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO automation.automation_runs (
			id, automation_id, tenant_id, request_id, status, step_type, trigger, step_config_json,
			output_json, error_message, action_results_json, created_at, started_at, completed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
	`,
		run.ID, run.AutomationID, run.TenantID, run.RequestID, string(run.Status), string(run.StepType), string(run.Trigger), run.StepConfigJSON,
		nullableString(run.OutputJSON), nullableString(run.ErrorMessage), actionResultsJSON, run.CreatedAt, nullableTime(run.StartedAt), nullableTime(run.CompletedAt),
	)
	if err != nil {
		return fmt.Errorf("postgres: insert automation run: %w", err)
	}
	return nil
}

func (r *AutomationRunRepository) FindByRequestID(ctx context.Context, tenantID, automationID, requestID string) (domain.AutomationRun, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+runColumns+`
		FROM automation.automation_runs
		WHERE tenant_id = $1 AND automation_id = $2 AND request_id = $3
	`, tenantID, automationID, requestID)

	run, err := scanRun(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AutomationRun{}, false, nil
	}
	if err != nil {
		return domain.AutomationRun{}, false, fmt.Errorf("postgres: query automation run by request_id: %w", err)
	}
	return run, true, nil
}

// FindRunning returns the currently-running run for automationID, if any —
// backed by idx_automation_runs_one_running.
func (r *AutomationRunRepository) FindRunning(ctx context.Context, tenantID, automationID string) (domain.AutomationRun, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+runColumns+`
		FROM automation.automation_runs
		WHERE tenant_id = $1 AND automation_id = $2 AND status = 'running'
		LIMIT 1
	`, tenantID, automationID)

	run, err := scanRun(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AutomationRun{}, false, nil
	}
	if err != nil {
		return domain.AutomationRun{}, false, fmt.Errorf("postgres: query running automation run: %w", err)
	}
	return run, true, nil
}

// UpdateStatus persists a run's status transition inside a transaction —
// for a terminal transition (Terminal() == true), the same transaction also
// writes the orca.automation.run.completed outbox entry, per the
// transactional-outbox convention (never a bare post-hoc publish call).
func (r *AutomationRunRepository) UpdateStatus(ctx context.Context, run domain.AutomationRun) error {
	actionResultsJSON, err := marshalActionResults(run.ActionResults)
	if err != nil {
		return fmt.Errorf("postgres: marshal action_results: %w", err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin update-status tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a successful Commit

	tag, err := tx.Exec(ctx, `
		UPDATE automation.automation_runs
		SET status = $1, output_json = $2, error_message = $3, action_results_json = $4, started_at = $5, completed_at = $6
		WHERE id = $7 AND tenant_id = $8
	`, string(run.Status), nullableString(run.OutputJSON), nullableString(run.ErrorMessage), actionResultsJSON,
		nullableTime(run.StartedAt), nullableTime(run.CompletedAt), run.ID, run.TenantID)
	if err != nil {
		if isUniqueViolation(err, "idx_automation_runs_one_running") {
			return usecase.ErrConcurrentRunActive
		}
		return fmt.Errorf("postgres: update automation run status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: automation run %s not found for tenant %s", run.ID, run.TenantID)
	}

	if run.Status.Terminal() && r.publisher != nil {
		if err := r.publisher.PublishRunCompleted(ctx, tx, run); err != nil {
			return fmt.Errorf("postgres: publish run-completed outbox entry: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit update-status tx: %w", err)
	}
	return nil
}

func (r *AutomationRunRepository) ListByAutomation(ctx context.Context, tenantID, automationID, pageToken string, pageSize int32) ([]domain.AutomationRun, string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+runColumns+`
		FROM automation.automation_runs
		WHERE tenant_id = $1 AND automation_id = $2 AND ($3 = '' OR id > $3::uuid)
		ORDER BY id
		LIMIT $4
	`, tenantID, automationID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query automation runs: %w", err)
	}
	defer rows.Close()

	var out []domain.AutomationRun
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, "", fmt.Errorf("postgres: scan automation run row: %w", err)
		}
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate automation run rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

// PruneOldRuns deletes every automation_runs row for automationID beyond
// the `keep` most recent (by created_at DESC) — BR-AT-07.
func (r *AutomationRunRepository) PruneOldRuns(ctx context.Context, tenantID, automationID string, keep int) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM automation.automation_runs
		WHERE tenant_id = $1 AND automation_id = $2
		  AND id NOT IN (
		    SELECT id FROM automation.automation_runs
		    WHERE tenant_id = $1 AND automation_id = $2
		    ORDER BY created_at DESC
		    LIMIT $3
		  )`,
		tenantID, automationID, keep,
	)
	if err != nil {
		return fmt.Errorf("postgres: prune old automation runs: %w", err)
	}
	return nil
}

// WriteCleanupReport persists one worktree_cleanup_log row per entry —
// BR-AT-14's per-worktree, per-reason audit trail. A single batched
// multi-row INSERT.
func (r *AutomationRunRepository) WriteCleanupReport(ctx context.Context, tenantID, runID string, entries []domain.CleanupLogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, e := range entries {
		batch.Queue(`
			INSERT INTO automation.worktree_cleanup_log (tenant_id, run_id, worktree_id, action, reason)
			VALUES ($1, $2, $3, $4, $5)
		`, tenantID, runID, e.WorktreeID, e.Action, nullableString(e.Reason))
	}
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range entries {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("postgres: insert worktree_cleanup_log row: %w", err)
		}
	}
	return nil
}

const runColumns = `id, automation_id, tenant_id, request_id, status, step_type, trigger, step_config_json,
	output_json, error_message, action_results_json, created_at, started_at, completed_at`

// FetchUnpublished and MarkPublished implement common/outbox.Store — polled
// by the common/outbox.Relay wired in cmd/server/main.go, which actually
// delivers each row to NATS JetStream.
func (r *AutomationRunRepository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM automation.outbox_events
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
		var payload []byte
		if err := rows.Scan(&rec.ID, &rec.Event.TenantID, &rec.Subject, &rec.Event.OccurredAt, &rec.Event.Version, &payload); err != nil {
			return nil, fmt.Errorf("postgres: scan outbox event row: %w", err)
		}
		rec.Event.ID = rec.ID
		rec.Event.Payload = payload
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate outbox event rows: %w", err)
	}
	return out, nil
}

func (r *AutomationRunRepository) MarkPublished(ctx context.Context, ids []string) error {
	_, err := r.pool.Exec(ctx, `UPDATE automation.outbox_events SET published_at = now() WHERE id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("postgres: mark outbox events published: %w", err)
	}
	return nil
}

// rowScanner abstracts over pgx.Row and pgx.Rows, which share the same
// Scan signature — lets scanRun serve both FindByRequestID and
// ListByAutomation without duplicating the column list.
type rowScanner interface {
	Scan(dest ...any) error
}

type actionResultRow struct {
	Index        int    `json:"index"`
	Status       string `json:"status"`
	OutputJSON   string `json:"output_json"`
	ErrorMessage string `json:"error_message"`
}

func marshalActionResults(results []domain.ActionResult) (*string, error) {
	if len(results) == 0 {
		return nil, nil
	}
	rows := make([]actionResultRow, len(results))
	for i, r := range results {
		rows[i] = actionResultRow{Index: r.Index, Status: r.Status, OutputJSON: r.OutputJSON, ErrorMessage: r.ErrorMessage}
	}
	b, err := json.Marshal(rows)
	if err != nil {
		return nil, err
	}
	s := string(b)
	return &s, nil
}

func unmarshalActionResults(raw *string) ([]domain.ActionResult, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	var rows []actionResultRow
	if err := json.Unmarshal([]byte(*raw), &rows); err != nil {
		return nil, err
	}
	out := make([]domain.ActionResult, len(rows))
	for i, r := range rows {
		out[i] = domain.ActionResult{Index: r.Index, Status: r.Status, OutputJSON: r.OutputJSON, ErrorMessage: r.ErrorMessage}
	}
	return out, nil
}

func scanRun(row rowScanner) (domain.AutomationRun, error) {
	var run domain.AutomationRun
	var status, stepType, trigger string
	var outputJSON, errorMessage, actionResultsJSON *string
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
		return domain.AutomationRun{}, fmt.Errorf("unmarshal action_results_json: %w", err)
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

func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// isUniqueViolation reports whether err is a Postgres unique_violation
// (23505) on the named constraint/index — used to distinguish "a
// concurrent dispatch already claimed this" from a real failure, both for
// the (tenant_id, request_id) idempotency index and
// idx_automation_runs_one_running (BR-AT-08).
func isUniqueViolation(err error, constraintName string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Code != "23505" {
		return false
	}
	return constraintName == "" || pgErr.ConstraintName == constraintName
}
