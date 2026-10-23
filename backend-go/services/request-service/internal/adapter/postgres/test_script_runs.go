package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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

const testScriptColumns = `id::text, tenant_id::text, request_id::text, attempt, status, reason_code, reason_message, test_spec_run_id, feature,
	spec_version, tsa_job_id, branch, base_ref, worktree_path, commit_sha, summary, COALESCE(started_by::text, ''), started_at, finished_at,
	updated_at, superseded_at`

func scanTestScriptRun(row pgx.Row) (domain.TestScriptRun, error) {
	var r domain.TestScriptRun
	var state string
	if err := row.Scan(&r.ID, &r.TenantID, &r.RequestID, &r.Attempt, &state, &r.ReasonCode, &r.ReasonMessage, &r.TestSpecRunID, &r.Feature,
		&r.SpecVersion, &r.TSAJobID, &r.Branch, &r.BaseRef, &r.WorktreePath, &r.CommitSHA, &r.Summary, &r.StartedBy, &r.StartedAt, &r.FinishedAt,
		&r.UpdatedAt, &r.SupersededAt); err != nil {
		return domain.TestScriptRun{}, err
	}
	r.State = domain.TestScriptStatus(state)
	r.StartedAt, r.UpdatedAt = r.StartedAt.UTC(), r.UpdatedAt.UTC()
	return r, nil
}

func (r *TestScriptRunRepository) Current(ctx context.Context, requestID string) (domain.TestScriptRun, bool, error) {
	var out domain.TestScriptRun
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		row, err := scanTestScriptRun(db.QueryRow(ctx, `SELECT `+testScriptColumns+` FROM request.request_test_script_runs
			WHERE tenant_id = $1 AND request_id = $2::uuid AND is_current = 1`, tenantID, requestID))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("postgres: current test-script run: %w", err)
		}
		out, found = row, true
		return nil
	})
	return out, found, err
}

func (r *TestScriptRunRepository) List(ctx context.Context, requestID string) ([]domain.TestScriptRun, error) {
	var out []domain.TestScriptRun
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		rows, err := db.Query(ctx, `SELECT `+testScriptColumns+` FROM request.request_test_script_runs
			WHERE tenant_id = $1 AND request_id = $2::uuid ORDER BY attempt DESC, started_at DESC`, tenantID, requestID)
		if err != nil {
			return fmt.Errorf("postgres: list test-script runs: %w", err)
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

const testScriptInsert = `INSERT INTO request.request_test_script_runs
	(id, tenant_id, request_id, attempt, status, reason_code, reason_message, test_spec_run_id, feature, spec_version, tsa_job_id, branch,
	 base_ref, worktree_path, commit_sha, summary, started_by, started_at, finished_at, updated_at, is_current)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16::jsonb, $17, $18, $19, $20, 1)`

func testScriptInsertArgs(tenantID string, run domain.TestScriptRun) []any {
	return []any{run.ID, tenantID, run.RequestID, run.Attempt, string(run.State), run.ReasonCode, run.ReasonMessage, run.TestSpecRunID, run.Feature,
		run.SpecVersion, run.TSAJobID, run.Branch, run.BaseRef, run.WorktreePath, run.CommitSHA, summaryJSON(run.Summary), nullIfEmpty(run.StartedBy),
		run.StartedAt, run.FinishedAt, run.UpdatedAt}
}

func (r *TestScriptRunRepository) Create(ctx context.Context, run domain.TestScriptRun) (bool, error) {
	created := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, testScriptInsert+` ON CONFLICT (tenant_id, request_id, is_current) DO NOTHING`, testScriptInsertArgs(tenantID, run)...)
		if err != nil {
			return fmt.Errorf("postgres: create test-script run: %w", err)
		}
		created = tag.RowsAffected() == 1
		return nil
	})
	return created, err
}

func (r *TestScriptRunRepository) Replace(ctx context.Context, run domain.TestScriptRun) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, err := db.Exec(ctx, `UPDATE request.request_test_script_runs SET is_current = NULL, superseded_at = now()
			WHERE tenant_id = $1 AND request_id = $2::uuid AND is_current = 1`, tenantID, run.RequestID); err != nil {
			return fmt.Errorf("postgres: supersede test-script run: %w", err)
		}
		if _, err := db.Exec(ctx, testScriptInsert, testScriptInsertArgs(tenantID, run)...); err != nil {
			return fmt.Errorf("postgres: replace test-script run: %w", err)
		}
		return nil
	})
}

func (r *TestScriptRunRepository) Update(ctx context.Context, run domain.TestScriptRun) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `UPDATE request.request_test_script_runs
			SET status = $3, reason_code = $4, reason_message = $5, test_spec_run_id = $6, feature = $7, spec_version = $8, tsa_job_id = $9,
			    branch = $10, base_ref = $11, worktree_path = $12, commit_sha = $13, summary = $14::jsonb, started_by = $15, finished_at = $16,
			    updated_at = $17
			WHERE tenant_id = $1 AND id = $2::uuid`,
			tenantID, run.ID, string(run.State), run.ReasonCode, run.ReasonMessage, run.TestSpecRunID, run.Feature, run.SpecVersion, run.TSAJobID,
			run.Branch, run.BaseRef, run.WorktreePath, run.CommitSHA, summaryJSON(run.Summary), nullIfEmpty(run.StartedBy), run.FinishedAt, run.UpdatedAt)
		if err != nil {
			return fmt.Errorf("postgres: update test-script run: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("postgres: update test-script run %s: no such row", run.ID)
		}
		return nil
	})
}

// Claim is a compare-and-set on the lease columns: exactly one of two racing callers sees one row updated.
func (r *TestScriptRunRepository) Claim(ctx context.Context, id, owner string, lease time.Duration) (bool, error) {
	claimed := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(id) {
			return nil
		}
		tag, err := db.Exec(ctx, `UPDATE request.request_test_script_runs
			SET lease_owner = $3, lease_until = now() + make_interval(secs => $4)
			WHERE tenant_id = $1 AND id = $2::uuid AND lease_until < now()`, tenantID, id, owner, lease.Seconds())
		if err != nil {
			return fmt.Errorf("postgres: claim test-script lease: %w", err)
		}
		claimed = tag.RowsAffected() == 1
		return nil
	})
	return claimed, err
}

func (r *TestScriptRunRepository) Release(ctx context.Context, id, owner string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(id) {
			return nil
		}
		_, err := db.Exec(ctx, `UPDATE request.request_test_script_runs SET lease_until = '2000-01-01'
			WHERE tenant_id = $1 AND id = $2::uuid AND lease_owner = $3`, tenantID, id, owner)
		return err
	})
}

func (r *TestScriptRunRepository) scanRefs(ctx context.Context, what, query string, withRun bool, args ...any) ([]usecase.TestScriptRef, error) {
	var refs []usecase.TestScriptRef
	err := r.withRelayTx(ctx, func(ctx context.Context, db dbExecer) error {
		rows, err := db.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var ref usecase.TestScriptRef
			dest := []any{&ref.TenantID, &ref.RequestID}
			if withRun {
				dest = append(dest, &ref.RunID)
			}
			if err := rows.Scan(dest...); err != nil {
				return err
			}
			refs = append(refs, ref)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: %s: %w", what, err)
	}
	return refs, nil
}

// ListDue reads across tenants under the relay setting (policy relay_scan, SELECT only).
func (r *TestScriptRunRepository) ListDue(ctx context.Context, quiet time.Duration, limit int) ([]usecase.TestScriptRef, error) {
	if limit <= 0 {
		limit = 20
	}
	return r.scanRefs(ctx, "list due test-script runs", `SELECT tenant_id::text, request_id::text, id::text FROM request.request_test_script_runs
		WHERE is_current = 1 AND status IN ('pending','running')
		  AND updated_at < now() - make_interval(secs => $1) AND lease_until < now()
		ORDER BY updated_at LIMIT $2`, true, quiet.Seconds(), limit)
}

// ListReadyWithoutRun reads across tenants under the relay setting: executing Requests whose current test specs completed and
// that have no test-script row. It only picks candidates (StartTestScripts is idempotent).
func (r *TestScriptRunRepository) ListReadyWithoutRun(ctx context.Context, quiet time.Duration, limit int) ([]usecase.TestScriptRef, error) {
	if limit <= 0 {
		limit = 20
	}
	return r.scanRefs(ctx, "list ready requests without test scripts", `SELECT r.tenant_id::text, r.id::text FROM request.requests r
		JOIN request.request_test_spec_runs s ON s.tenant_id = r.tenant_id AND s.request_id = r.id AND s.is_current = 1 AND s.status = 'completed'
		WHERE r.status = 'executing' AND s.updated_at < now() - make_interval(secs => $1)
		  AND NOT EXISTS (SELECT 1 FROM request.request_test_script_runs t WHERE t.tenant_id = r.tenant_id AND t.request_id = r.id)
		ORDER BY s.updated_at LIMIT $2`, false, quiet.Seconds(), limit)
}
