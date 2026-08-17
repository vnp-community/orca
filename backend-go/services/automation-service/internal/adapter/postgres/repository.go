// Package postgres implements automation-service's AutomationRepository and
// AutomationRunRepository ports (defined in internal/usecase) against this
// service's own PostgreSQL database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule: this is the ONLY package in automation-service
// that knows SQL exists.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

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

func (r *AutomationRepository) Create(ctx context.Context, a domain.Automation) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO automation.automations (
			id, tenant_id, name, rrule, dtstart, step_type, step_config_json, enabled, timezone, next_run_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`, a.ID, a.TenantID, a.Name, a.RRule, a.DTStart, string(a.StepType), a.StepConfigJSON, a.Enabled, a.Timezone, nullableTime(a.NextRunAt), a.CreatedAt, a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres: insert automation: %w", err)
	}
	return nil
}

func (r *AutomationRepository) Get(ctx context.Context, tenantID, id string) (domain.Automation, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, rrule, dtstart, step_type, step_config_json, enabled, timezone, next_run_at, created_at, updated_at
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
		SELECT id, tenant_id, name, rrule, dtstart, step_type, step_config_json, enabled, timezone, next_run_at, created_at, updated_at
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
	var stepType string
	var nextRunAt *time.Time
	if err := row.Scan(
		&a.ID, &a.TenantID, &a.Name, &a.RRule, &a.DTStart, &stepType, &a.StepConfigJSON,
		&a.Enabled, &a.Timezone, &nextRunAt, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return domain.Automation{}, err
	}
	a.StepType = domain.StepType(stepType)
	if nextRunAt != nil {
		a.NextRunAt = *nextRunAt
	}
	return a, nil
}

// AutomationRunRepository implements usecase.AutomationRunRepository.
type AutomationRunRepository struct {
	pool *pgxpool.Pool
}

func NewAutomationRunRepository(pool *pgxpool.Pool) *AutomationRunRepository {
	return &AutomationRunRepository{pool: pool}
}

func (r *AutomationRunRepository) Create(ctx context.Context, run domain.AutomationRun) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO automation.automation_runs (
			id, automation_id, tenant_id, request_id, status, step_type, trigger, step_config_json,
			output_json, error_message, created_at, started_at, completed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`,
		run.ID, run.AutomationID, run.TenantID, run.RequestID, string(run.Status), string(run.StepType), string(run.Trigger), run.StepConfigJSON,
		nullableString(run.OutputJSON), nullableString(run.ErrorMessage), run.CreatedAt, nullableTime(run.StartedAt), nullableTime(run.CompletedAt),
	)
	if err != nil {
		return fmt.Errorf("postgres: insert automation run: %w", err)
	}
	return nil
}

func (r *AutomationRunRepository) FindByRequestID(ctx context.Context, tenantID, automationID, requestID string) (domain.AutomationRun, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, automation_id, tenant_id, request_id, status, step_type, trigger, step_config_json,
		       output_json, error_message, created_at, started_at, completed_at
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

func (r *AutomationRunRepository) UpdateStatus(ctx context.Context, run domain.AutomationRun) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE automation.automation_runs
		SET status = $1, output_json = $2, error_message = $3, started_at = $4, completed_at = $5
		WHERE id = $6 AND tenant_id = $7
	`, string(run.Status), nullableString(run.OutputJSON), nullableString(run.ErrorMessage),
		nullableTime(run.StartedAt), nullableTime(run.CompletedAt), run.ID, run.TenantID)
	if err != nil {
		return fmt.Errorf("postgres: update automation run status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: automation run %s not found for tenant %s", run.ID, run.TenantID)
	}
	return nil
}

func (r *AutomationRunRepository) ListByAutomation(ctx context.Context, tenantID, automationID, pageToken string, pageSize int32) ([]domain.AutomationRun, string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, automation_id, tenant_id, request_id, status, step_type, trigger, step_config_json,
		       output_json, error_message, created_at, started_at, completed_at
		FROM automation.automation_runs
		WHERE tenant_id = $1 AND automation_id = $2 AND id > $3
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

// rowScanner abstracts over pgx.Row and pgx.Rows, which share the same
// Scan signature — lets scanRun serve both FindByRequestID and
// ListByAutomation without duplicating the column list.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanRun(row rowScanner) (domain.AutomationRun, error) {
	var run domain.AutomationRun
	var status, stepType, trigger string
	var outputJSON, errorMessage *string
	var startedAt, completedAt *time.Time
	if err := row.Scan(
		&run.ID, &run.AutomationID, &run.TenantID, &run.RequestID, &status, &stepType, &trigger, &run.StepConfigJSON,
		&outputJSON, &errorMessage, &run.CreatedAt, &startedAt, &completedAt,
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
