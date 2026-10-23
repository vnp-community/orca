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

// TestScriptRunRepository stores the attempts to generate the test scripts of a Request (migration 0111). Its own table: the code
// lane and the test-spec lane read and write none of it.
type TestScriptRunRepository struct {
	*Repository
}

func NewTestScriptRunRepository(r *Repository) *TestScriptRunRepository {
	return &TestScriptRunRepository{Repository: r}
}

var _ usecase.TestScriptRunRepository = (*TestScriptRunRepository)(nil)

const testScriptColumns = `id, tenant_id, request_id, attempt, status, reason_code, reason_message, test_spec_run_id, feature, spec_version,
	tsa_job_id, branch, base_ref, worktree_path, commit_sha, summary, COALESCE(started_by, ''), started_at, finished_at, updated_at, superseded_at`

func scanTestScriptRun(row rowScanner) (domain.TestScriptRun, error) {
	var r domain.TestScriptRun
	var state string
	var summary []byte
	var finished, superseded sql.NullTime
	if err := row.Scan(&r.ID, &r.TenantID, &r.RequestID, &r.Attempt, &state, &r.ReasonCode, &r.ReasonMessage, &r.TestSpecRunID, &r.Feature,
		&r.SpecVersion, &r.TSAJobID, &r.Branch, &r.BaseRef, &r.WorktreePath, &r.CommitSHA, &summary, &r.StartedBy, &r.StartedAt, &finished,
		&r.UpdatedAt, &superseded); err != nil {
		return domain.TestScriptRun{}, err
	}
	r.State = domain.TestScriptStatus(state)
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

func (r *TestScriptRunRepository) Current(ctx context.Context, requestID string) (domain.TestScriptRun, bool, error) {
	var out domain.TestScriptRun
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		row, err := scanTestScriptRun(db.QueryRowContext(ctx, `SELECT `+testScriptColumns+` FROM request_test_script_runs
			WHERE tenant_id = ? AND request_id = ? AND is_current = 1`, tenantID, requestID))
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("mysql: current test-script run: %w", err)
		}
		out, found = row, true
		return nil
	})
	return out, found, err
}

func (r *TestScriptRunRepository) List(ctx context.Context, requestID string) ([]domain.TestScriptRun, error) {
	var out []domain.TestScriptRun
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.QueryContext(ctx, `SELECT `+testScriptColumns+` FROM request_test_script_runs
			WHERE tenant_id = ? AND request_id = ? ORDER BY attempt DESC, started_at DESC`, tenantID, requestID)
		if err != nil {
			return fmt.Errorf("mysql: list test-script runs: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			run, err := scanTestScriptRun(rows)
			if err != nil {
				return err
			}
			out = append(out, run)
		}
		return rows.Err()
	})
	return out, err
}

const testScriptInsert = `INSERT INTO request_test_script_runs
	(id, tenant_id, request_id, attempt, status, reason_code, reason_message, test_spec_run_id, feature, spec_version, tsa_job_id, branch,
	 base_ref, worktree_path, commit_sha, summary, started_by, started_at, finished_at, updated_at, is_current)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`

func testScriptInsertArgs(tenantID string, run domain.TestScriptRun) []any {
	return []any{run.ID, tenantID, run.RequestID, run.Attempt, string(run.State), run.ReasonCode, run.ReasonMessage, run.TestSpecRunID, run.Feature,
		run.SpecVersion, run.TSAJobID, run.Branch, run.BaseRef, run.WorktreePath, run.CommitSHA, tsSummaryJSON(run.Summary), tsNull(run.StartedBy),
		run.StartedAt, run.FinishedAt, run.UpdatedAt}
}

// Create uses ON DUPLICATE KEY UPDATE with a no-op instead of INSERT IGNORE, which would also swallow data errors.
func (r *TestScriptRunRepository) Create(ctx context.Context, run domain.TestScriptRun) (bool, error) {
	created := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, testScriptInsert+` ON DUPLICATE KEY UPDATE tenant_id = tenant_id`, testScriptInsertArgs(tenantID, run)...)
		if err != nil {
			return fmt.Errorf("mysql: create test-script run: %w", err)
		}
		n, err := res.RowsAffected()
		created = n == 1
		return err
	})
	return created, err
}

func (r *TestScriptRunRepository) Replace(ctx context.Context, run domain.TestScriptRun) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, err := db.ExecContext(ctx, `UPDATE request_test_script_runs SET is_current = NULL, superseded_at = CURRENT_TIMESTAMP(6)
			WHERE tenant_id = ? AND request_id = ? AND is_current = 1`, tenantID, run.RequestID); err != nil {
			return fmt.Errorf("mysql: supersede test-script run: %w", err)
		}
		if _, err := db.ExecContext(ctx, testScriptInsert, testScriptInsertArgs(tenantID, run)...); err != nil {
			return fmt.Errorf("mysql: replace test-script run: %w", err)
		}
		return nil
	})
}

func (r *TestScriptRunRepository) Update(ctx context.Context, run domain.TestScriptRun) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `UPDATE request_test_script_runs
			SET status = ?, reason_code = ?, reason_message = ?, test_spec_run_id = ?, feature = ?, spec_version = ?, tsa_job_id = ?, branch = ?,
			    base_ref = ?, worktree_path = ?, commit_sha = ?, summary = ?, started_by = ?, finished_at = ?, updated_at = ?
			WHERE tenant_id = ? AND id = ?`,
			string(run.State), run.ReasonCode, run.ReasonMessage, run.TestSpecRunID, run.Feature, run.SpecVersion, run.TSAJobID, run.Branch,
			run.BaseRef, run.WorktreePath, run.CommitSHA, tsSummaryJSON(run.Summary), tsNull(run.StartedBy), run.FinishedAt, run.UpdatedAt,
			tenantID, run.ID)
		if err != nil {
			return fmt.Errorf("mysql: update test-script run: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			// MySQL reports 0 for an update that changed nothing; make sure the row exists before calling it an error.
			var one int
			if err := db.QueryRowContext(ctx, `SELECT 1 FROM request_test_script_runs WHERE tenant_id = ? AND id = ?`, tenantID, run.ID).Scan(&one); err != nil {
				return fmt.Errorf("mysql: update test-script run %s: no such row", run.ID)
			}
		}
		return nil
	})
}

// Claim is a compare-and-set on the lease columns: exactly one of two racing callers sees one row changed.
func (r *TestScriptRunRepository) Claim(ctx context.Context, id, owner string, lease time.Duration) (bool, error) {
	claimed := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `UPDATE request_test_script_runs
			SET lease_owner = ?, lease_until = TIMESTAMPADD(MICROSECOND, ?, CURRENT_TIMESTAMP(6))
			WHERE tenant_id = ? AND id = ? AND lease_until < CURRENT_TIMESTAMP(6)`, owner, lease.Microseconds(), tenantID, id)
		if err != nil {
			return fmt.Errorf("mysql: claim test-script lease: %w", err)
		}
		n, err := res.RowsAffected()
		claimed = n == 1
		return err
	})
	return claimed, err
}

func (r *TestScriptRunRepository) Release(ctx context.Context, id, owner string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		_, err := db.ExecContext(ctx, `UPDATE request_test_script_runs SET lease_until = '2000-01-01 00:00:01.000000'
			WHERE tenant_id = ? AND id = ? AND lease_owner = ?`, tenantID, id, owner)
		return err
	})
}

func (r *TestScriptRunRepository) scanRefs(ctx context.Context, what, query string, withRun bool, args ...any) ([]usecase.TestScriptRef, error) {
	rows, err := r.exec(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql: %s: %w", what, err)
	}
	defer rows.Close()
	var refs []usecase.TestScriptRef
	for rows.Next() {
		var ref usecase.TestScriptRef
		dest := []any{&ref.TenantID, &ref.RequestID}
		if withRun {
			dest = append(dest, &ref.RunID)
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

// ListDue picks candidates across tenants (MySQL has no RLS to lift); the per-row lease is what keeps replicas apart.
func (r *TestScriptRunRepository) ListDue(ctx context.Context, quiet time.Duration, limit int) ([]usecase.TestScriptRef, error) {
	if limit <= 0 {
		limit = 20
	}
	return r.scanRefs(ctx, "list due test-script runs", `SELECT tenant_id, request_id, id FROM request_test_script_runs
		WHERE is_current = 1 AND status IN ('pending','running')
		  AND updated_at < TIMESTAMPADD(MICROSECOND, -?, CURRENT_TIMESTAMP(6)) AND lease_until < CURRENT_TIMESTAMP(6)
		ORDER BY updated_at LIMIT ?`, true, quiet.Microseconds(), limit)
}

func (r *TestScriptRunRepository) ListReadyWithoutRun(ctx context.Context, quiet time.Duration, limit int) ([]usecase.TestScriptRef, error) {
	if limit <= 0 {
		limit = 20
	}
	return r.scanRefs(ctx, "list ready requests without test scripts", `SELECT r.tenant_id, r.id FROM requests r
		JOIN request_test_spec_runs s ON s.tenant_id = r.tenant_id AND s.request_id = r.id AND s.is_current = 1 AND s.status = 'completed'
		WHERE r.status = 'executing' AND s.updated_at < TIMESTAMPADD(MICROSECOND, -?, CURRENT_TIMESTAMP(6))
		  AND NOT EXISTS (SELECT 1 FROM request_test_script_runs t WHERE t.tenant_id = r.tenant_id AND t.request_id = r.id)
		ORDER BY s.updated_at LIMIT ?`, false, quiet.Microseconds(), limit)
}
