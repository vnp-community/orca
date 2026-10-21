package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// TestSpecRunRepository stores the attempts to generate the test specs of a Request (migration 0099). Its own table: the code
// lane reads and writes none of it.
type TestSpecRunRepository struct {
	*Repository
}

func NewTestSpecRunRepository(r *Repository) *TestSpecRunRepository {
	return &TestSpecRunRepository{Repository: r}
}

var _ usecase.TestSpecRunRepository = (*TestSpecRunRepository)(nil)

const testSpecColumns = `id, tenant_id, request_id, attempt, status, reason_code, reason_message, context_mode, context_ref,
	context_sha256, tdd_run_id, feature, summary, COALESCE(started_by, ''), started_at, finished_at, updated_at, superseded_at`

func scanTestSpecRun(row rowScanner) (domain.TestSpecRun, error) {
	var r domain.TestSpecRun
	var state string
	var summary []byte
	var finished, superseded sql.NullTime
	if err := row.Scan(&r.ID, &r.TenantID, &r.RequestID, &r.Attempt, &state, &r.ReasonCode, &r.ReasonMessage, &r.ContextMode, &r.ContextRef,
		&r.ContextSHA256, &r.TDDRunID, &r.Feature, &summary, &r.StartedBy, &r.StartedAt, &finished, &r.UpdatedAt, &superseded); err != nil {
		return domain.TestSpecRun{}, err
	}
	r.State = domain.TestSpecStatus(state)
	r.Summary = summary
	r.StartedAt, r.UpdatedAt = r.StartedAt.UTC(), r.UpdatedAt.UTC()
	if finished.Valid {
		t := finished.Time.UTC()
		r.FinishedAt = &t
	}
	if superseded.Valid {
		t := superseded.Time.UTC()
		r.SupersededAt = &t
	}
	return r, nil
}

func tsSummaryJSON(s []byte) string {
	if len(s) == 0 {
		return "{}"
	}
	return string(s)
}

func tsNull(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (r *TestSpecRunRepository) Current(ctx context.Context, requestID string) (domain.TestSpecRun, bool, error) {
	var out domain.TestSpecRun
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		row, err := scanTestSpecRun(db.QueryRowContext(ctx, `SELECT `+testSpecColumns+` FROM request_test_spec_runs
			WHERE tenant_id = ? AND request_id = ? AND is_current = 1`, tenantID, requestID))
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("mysql: current test-spec run: %w", err)
		}
		out, found = row, true
		return nil
	})
	return out, found, err
}

func (r *TestSpecRunRepository) List(ctx context.Context, requestID string) ([]domain.TestSpecRun, error) {
	var out []domain.TestSpecRun
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.QueryContext(ctx, `SELECT `+testSpecColumns+` FROM request_test_spec_runs
			WHERE tenant_id = ? AND request_id = ? ORDER BY attempt DESC, started_at DESC`, tenantID, requestID)
		if err != nil {
			return fmt.Errorf("mysql: list test-spec runs: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			run, err := scanTestSpecRun(rows)
			if err != nil {
				return err
			}
			out = append(out, run)
		}
		return rows.Err()
	})
	return out, err
}

const testSpecInsert = `INSERT INTO request_test_spec_runs
	(id, tenant_id, request_id, attempt, status, reason_code, reason_message, context_mode, context_ref, context_sha256, tdd_run_id, feature, summary,
	 started_by, started_at, finished_at, updated_at, is_current)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`

func testSpecInsertArgs(tenantID string, run domain.TestSpecRun) []any {
	return []any{run.ID, tenantID, run.RequestID, run.Attempt, string(run.State), run.ReasonCode, run.ReasonMessage, run.ContextMode, run.ContextRef,
		run.ContextSHA256, run.TDDRunID, run.Feature, tsSummaryJSON(run.Summary), tsNull(run.StartedBy), run.StartedAt, run.FinishedAt, run.UpdatedAt}
}

// Create uses ON DUPLICATE KEY UPDATE with a no-op instead of INSERT IGNORE, which would also swallow data errors.
func (r *TestSpecRunRepository) Create(ctx context.Context, run domain.TestSpecRun) (bool, error) {
	created := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, testSpecInsert+` ON DUPLICATE KEY UPDATE tenant_id = tenant_id`, testSpecInsertArgs(tenantID, run)...)
		if err != nil {
			return fmt.Errorf("mysql: create test-spec run: %w", err)
		}
		n, err := res.RowsAffected()
		created = n == 1
		return err
	})
	return created, err
}

func (r *TestSpecRunRepository) Replace(ctx context.Context, run domain.TestSpecRun) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, err := db.ExecContext(ctx, `UPDATE request_test_spec_runs SET is_current = NULL, superseded_at = CURRENT_TIMESTAMP(6)
			WHERE tenant_id = ? AND request_id = ? AND is_current = 1`, tenantID, run.RequestID); err != nil {
			return fmt.Errorf("mysql: supersede test-spec run: %w", err)
		}
		if _, err := db.ExecContext(ctx, testSpecInsert, testSpecInsertArgs(tenantID, run)...); err != nil {
			return fmt.Errorf("mysql: replace test-spec run: %w", err)
		}
		return nil
	})
}

func (r *TestSpecRunRepository) Update(ctx context.Context, run domain.TestSpecRun) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `UPDATE request_test_spec_runs
			SET status = ?, reason_code = ?, reason_message = ?, context_mode = ?, context_ref = ?, context_sha256 = ?, tdd_run_id = ?,
			    feature = ?, summary = ?, started_by = ?, finished_at = ?, updated_at = ?
			WHERE tenant_id = ? AND id = ?`,
			string(run.State), run.ReasonCode, run.ReasonMessage, run.ContextMode, run.ContextRef, run.ContextSHA256, run.TDDRunID,
			run.Feature, tsSummaryJSON(run.Summary), tsNull(run.StartedBy), run.FinishedAt, run.UpdatedAt, tenantID, run.ID)
		if err != nil {
			return fmt.Errorf("mysql: update test-spec run: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			// MySQL reports 0 for an update that changed nothing; make sure the row exists before calling it an error.
			var one int
			if err := db.QueryRowContext(ctx, `SELECT 1 FROM request_test_spec_runs WHERE tenant_id = ? AND id = ?`, tenantID, run.ID).Scan(&one); err != nil {
				return fmt.Errorf("mysql: update test-spec run %s: no such row", run.ID)
			}
		}
		return nil
	})
}

// Claim is a compare-and-set on the lease columns: exactly one of two racing callers sees one row changed.
func (r *TestSpecRunRepository) Claim(ctx context.Context, id, owner string, lease time.Duration) (bool, error) {
	claimed := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `UPDATE request_test_spec_runs
			SET lease_owner = ?, lease_until = TIMESTAMPADD(MICROSECOND, ?, CURRENT_TIMESTAMP(6))
			WHERE tenant_id = ? AND id = ? AND lease_until < CURRENT_TIMESTAMP(6)`, owner, lease.Microseconds(), tenantID, id)
		if err != nil {
			return fmt.Errorf("mysql: claim test-spec lease: %w", err)
		}
		n, err := res.RowsAffected()
		claimed = n == 1
		return err
	})
	return claimed, err
}

func (r *TestSpecRunRepository) Release(ctx context.Context, id, owner string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		_, err := db.ExecContext(ctx, `UPDATE request_test_spec_runs SET lease_until = '2000-01-01 00:00:01.000000'
			WHERE tenant_id = ? AND id = ? AND lease_owner = ?`, tenantID, id, owner)
		return err
	})
}

// ListDue picks candidates across tenants (MySQL has no RLS to lift); the per-row lease is what keeps replicas apart.
func (r *TestSpecRunRepository) ListDue(ctx context.Context, quiet time.Duration, limit int) ([]usecase.TestSpecRef, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.exec(ctx).QueryContext(ctx, `SELECT tenant_id, request_id, id FROM request_test_spec_runs
		WHERE is_current = 1 AND status IN ('pending','starting','running')
		  AND updated_at < TIMESTAMPADD(MICROSECOND, -?, CURRENT_TIMESTAMP(6)) AND lease_until < CURRENT_TIMESTAMP(6)
		ORDER BY updated_at LIMIT ?`, quiet.Microseconds(), limit)
	if err != nil {
		return nil, fmt.Errorf("mysql: list due test-spec runs: %w", err)
	}
	defer rows.Close()
	var refs []usecase.TestSpecRef
	for rows.Next() {
		var ref usecase.TestSpecRef
		if err := rows.Scan(&ref.TenantID, &ref.RequestID, &ref.RunID); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func (r *TestSpecRunRepository) ListExecutingWithoutRun(ctx context.Context, quiet time.Duration, limit int) ([]usecase.TestSpecRef, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.exec(ctx).QueryContext(ctx, `SELECT r.tenant_id, r.id FROM requests r
		WHERE r.status IN ('planning', 'awaiting_plan_approval', 'executing') AND r.updated_at < TIMESTAMPADD(MICROSECOND, -?, CURRENT_TIMESTAMP(6))
		  AND NOT EXISTS (SELECT 1 FROM request_test_spec_runs t WHERE t.tenant_id = r.tenant_id AND t.request_id = r.id)
		ORDER BY r.updated_at LIMIT ?`, quiet.Microseconds(), limit)
	if err != nil {
		return nil, fmt.Errorf("mysql: list executing requests without test specs: %w", err)
	}
	defer rows.Close()
	var refs []usecase.TestSpecRef
	for rows.Next() {
		var ref usecase.TestSpecRef
		if err := rows.Scan(&ref.TenantID, &ref.RequestID); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}
