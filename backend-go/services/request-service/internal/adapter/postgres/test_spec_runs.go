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

// TestSpecRunRepository stores the attempts to generate the test specs of a Request (migration 0110). Its own table: the code
// lane reads and writes none of it.
type TestSpecRunRepository struct {
	*Repository
}

func NewTestSpecRunRepository(r *Repository) *TestSpecRunRepository {
	return &TestSpecRunRepository{Repository: r}
}

var _ usecase.TestSpecRunRepository = (*TestSpecRunRepository)(nil)

const testSpecColumns = `id::text, tenant_id::text, request_id::text, attempt, status, reason_code, reason_message, context_mode, context_ref,
	context_sha256, tdd_run_id, feature, summary, COALESCE(started_by::text, ''), started_at, finished_at, updated_at, superseded_at`

func scanTestSpecRun(row pgx.Row) (domain.TestSpecRun, error) {
	var r domain.TestSpecRun
	var state string
	if err := row.Scan(&r.ID, &r.TenantID, &r.RequestID, &r.Attempt, &state, &r.ReasonCode, &r.ReasonMessage, &r.ContextMode, &r.ContextRef,
		&r.ContextSHA256, &r.TDDRunID, &r.Feature, &r.Summary, &r.StartedBy, &r.StartedAt, &r.FinishedAt, &r.UpdatedAt, &r.SupersededAt); err != nil {
		return domain.TestSpecRun{}, err
	}
	r.State = domain.TestSpecStatus(state)
	r.StartedAt, r.UpdatedAt = r.StartedAt.UTC(), r.UpdatedAt.UTC()
	return r, nil
}

func summaryJSON(s []byte) string {
	if len(s) == 0 {
		return "{}"
	}
	return string(s)
}

func (r *TestSpecRunRepository) Current(ctx context.Context, requestID string) (domain.TestSpecRun, bool, error) {
	var out domain.TestSpecRun
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		row, err := scanTestSpecRun(db.QueryRow(ctx, `SELECT `+testSpecColumns+` FROM request.request_test_spec_runs
			WHERE tenant_id = $1 AND request_id = $2::uuid AND is_current = 1`, tenantID, requestID))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("postgres: current test-spec run: %w", err)
		}
		out, found = row, true
		return nil
	})
	return out, found, err
}

func (r *TestSpecRunRepository) List(ctx context.Context, requestID string) ([]domain.TestSpecRun, error) {
	var out []domain.TestSpecRun
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		rows, err := db.Query(ctx, `SELECT `+testSpecColumns+` FROM request.request_test_spec_runs
			WHERE tenant_id = $1 AND request_id = $2::uuid ORDER BY attempt DESC, started_at DESC`, tenantID, requestID)
		if err != nil {
			return fmt.Errorf("postgres: list test-spec runs: %w", err)
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

const testSpecInsert = `INSERT INTO request.request_test_spec_runs
	(id, tenant_id, request_id, attempt, status, reason_code, reason_message, context_mode, context_ref, context_sha256, tdd_run_id, feature, summary,
	 started_by, started_at, finished_at, updated_at, is_current)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb, $14, $15, $16, $17, 1)`

func testSpecInsertArgs(tenantID string, run domain.TestSpecRun) []any {
	return []any{run.ID, tenantID, run.RequestID, run.Attempt, string(run.State), run.ReasonCode, run.ReasonMessage, run.ContextMode, run.ContextRef,
		run.ContextSHA256, run.TDDRunID, run.Feature, summaryJSON(run.Summary), nullIfEmpty(run.StartedBy), run.StartedAt, run.FinishedAt, run.UpdatedAt}
}

func (r *TestSpecRunRepository) Create(ctx context.Context, run domain.TestSpecRun) (bool, error) {
	created := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, testSpecInsert+` ON CONFLICT (tenant_id, request_id, is_current) DO NOTHING`, testSpecInsertArgs(tenantID, run)...)
		if err != nil {
			return fmt.Errorf("postgres: create test-spec run: %w", err)
		}
		created = tag.RowsAffected() == 1
		return nil
	})
	return created, err
}

func (r *TestSpecRunRepository) Replace(ctx context.Context, run domain.TestSpecRun) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, err := db.Exec(ctx, `UPDATE request.request_test_spec_runs SET is_current = NULL, superseded_at = now()
			WHERE tenant_id = $1 AND request_id = $2::uuid AND is_current = 1`, tenantID, run.RequestID); err != nil {
			return fmt.Errorf("postgres: supersede test-spec run: %w", err)
		}
		if _, err := db.Exec(ctx, testSpecInsert, testSpecInsertArgs(tenantID, run)...); err != nil {
			return fmt.Errorf("postgres: replace test-spec run: %w", err)
		}
		return nil
	})
}

func (r *TestSpecRunRepository) Update(ctx context.Context, run domain.TestSpecRun) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `UPDATE request.request_test_spec_runs
			SET status = $3, reason_code = $4, reason_message = $5, context_mode = $6, context_ref = $7, context_sha256 = $8, tdd_run_id = $9,
			    feature = $10, summary = $11::jsonb, started_by = $12, finished_at = $13, updated_at = $14
			WHERE tenant_id = $1 AND id = $2::uuid`,
			tenantID, run.ID, string(run.State), run.ReasonCode, run.ReasonMessage, run.ContextMode, run.ContextRef, run.ContextSHA256, run.TDDRunID,
			run.Feature, summaryJSON(run.Summary), nullIfEmpty(run.StartedBy), run.FinishedAt, run.UpdatedAt)
		if err != nil {
			return fmt.Errorf("postgres: update test-spec run: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("postgres: update test-spec run %s: no such row", run.ID)
		}
		return nil
	})
}

// Claim is a compare-and-set on the lease columns: exactly one of two racing callers sees one row updated.
func (r *TestSpecRunRepository) Claim(ctx context.Context, id, owner string, lease time.Duration) (bool, error) {
	claimed := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(id) {
			return nil
		}
		tag, err := db.Exec(ctx, `UPDATE request.request_test_spec_runs
			SET lease_owner = $3, lease_until = now() + make_interval(secs => $4)
			WHERE tenant_id = $1 AND id = $2::uuid AND lease_until < now()`, tenantID, id, owner, lease.Seconds())
		if err != nil {
			return fmt.Errorf("postgres: claim test-spec lease: %w", err)
		}
		claimed = tag.RowsAffected() == 1
		return nil
	})
	return claimed, err
}

func (r *TestSpecRunRepository) Release(ctx context.Context, id, owner string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(id) {
			return nil
		}
		_, err := db.Exec(ctx, `UPDATE request.request_test_spec_runs SET lease_until = '2000-01-01'
			WHERE tenant_id = $1 AND id = $2::uuid AND lease_owner = $3`, tenantID, id, owner)
		return err
	})
}

// ListDue reads across tenants under the relay setting (policy relay_scan, SELECT only).
func (r *TestSpecRunRepository) ListDue(ctx context.Context, quiet time.Duration, limit int) ([]usecase.TestSpecRef, error) {
	if limit <= 0 {
		limit = 20
	}
	var refs []usecase.TestSpecRef
	err := r.withRelayTx(ctx, func(ctx context.Context, db dbExecer) error {
		rows, err := db.Query(ctx, `SELECT tenant_id::text, request_id::text, id::text FROM request.request_test_spec_runs
			WHERE is_current = 1 AND status IN ('pending','starting','running')
			  AND updated_at < now() - make_interval(secs => $1) AND lease_until < now()
			ORDER BY updated_at LIMIT $2`, quiet.Seconds(), limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var ref usecase.TestSpecRef
			if err := rows.Scan(&ref.TenantID, &ref.RequestID, &ref.RunID); err != nil {
				return err
			}
			refs = append(refs, ref)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: list due test-spec runs: %w", err)
	}
	return refs, nil
}

// ListExecutingWithoutRun reads across tenants under the relay setting; it only picks candidates (StartTestSpecs is idempotent).
func (r *TestSpecRunRepository) ListExecutingWithoutRun(ctx context.Context, quiet time.Duration, limit int) ([]usecase.TestSpecRef, error) {
	if limit <= 0 {
		limit = 20
	}
	var refs []usecase.TestSpecRef
	err := r.withRelayTx(ctx, func(ctx context.Context, db dbExecer) error {
		rows, err := db.Query(ctx, `SELECT r.tenant_id::text, r.id::text FROM request.requests r
			WHERE r.status IN ('planning', 'awaiting_plan_approval', 'executing') AND r.updated_at < now() - make_interval(secs => $1)
			  AND NOT EXISTS (SELECT 1 FROM request.request_test_spec_runs t WHERE t.tenant_id = r.tenant_id AND t.request_id = r.id)
			ORDER BY r.updated_at LIMIT $2`, quiet.Seconds(), limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var ref usecase.TestSpecRef
			if err := rows.Scan(&ref.TenantID, &ref.RequestID); err != nil {
				return err
			}
			refs = append(refs, ref)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: list executing requests without test specs: %w", err)
	}
	return refs, nil
}
