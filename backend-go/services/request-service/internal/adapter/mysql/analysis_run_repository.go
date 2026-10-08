package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type AnalysisRunRepository struct {
	*Repository
}

func NewAnalysisRunRepository(r *Repository) *AnalysisRunRepository {
	return &AnalysisRunRepository{Repository: r}
}

var _ usecase.AnalysisRunStore = (*AnalysisRunRepository)(nil)

const analysisRunColumns = `id, tenant_id, request_id, kind, mode, status, idempotency_key, attempt, lease_owner, lease_expires_at,
	error_code, error_message, raw_output, started_at, finished_at, solution_id, project_id, actor_id, feedback, enforcement, repo_check`

func scanAnalysisRun(row rowScanner) (domain.AnalysisRun, error) {
	var (
		r                                                                                             domain.AnalysisRun
		kind, mode, status                                                                            string
		idem, owner, errCode, errMsg, raw, solutionID, projectID, actorID, feedback, enforcement, rep sql.NullString
		leaseExpires, finished                                                                        sql.NullTime
	)
	err := row.Scan(&r.ID, &r.TenantID, &r.RequestID, &kind, &mode, &status, &idem, &r.Attempt, &owner, &leaseExpires,
		&errCode, &errMsg, &raw, &r.StartedAt, &finished, &solutionID, &projectID, &actorID, &feedback, &enforcement, &rep)
	if err != nil {
		return domain.AnalysisRun{}, err
	}
	r.Kind, r.Mode, r.Status = domain.RunKind(kind), domain.AnalysisMode(mode), domain.RunStatus(status)
	r.IdempotencyKey, r.LeaseOwner, r.ErrorCode, r.ErrorMessage, r.RawOutput = nullStringPtr(idem), nullStringPtr(owner), nullStringPtr(errCode), nullStringPtr(errMsg), nullStringPtr(raw)
	r.SolutionID, r.ProjectID, r.ActorID = solutionID.String, projectID.String, actorID.String
	r.Feedback, r.Enforcement, r.RepoCheck = feedback.String, enforcement.String, rep.String
	r.StartedAt = r.StartedAt.UTC()
	if leaseExpires.Valid {
		t := leaseExpires.Time.UTC()
		r.LeaseExpiresAt = &t
	}
	if finished.Valid {
		t := finished.Time.UTC()
		r.FinishedAt = &t
	}
	return r, nil
}

func ptrArg(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// findBlocking looks for the run that already answers this start. After a lost insert race the lookup must be a
// locking read: a plain read would keep using the transaction's old snapshot and miss the winner's committed row.
func (r *AnalysisRunRepository) findBlocking(ctx context.Context, db dbExecer, tenantID string, run domain.AnalysisRun, current bool) (domain.AnalysisRun, bool, error) {
	lock := ""
	if current {
		lock = " FOR SHARE"
	}
	got, err := scanAnalysisRun(db.QueryRowContext(ctx, `SELECT `+analysisRunColumns+` FROM analysis_runs
		WHERE tenant_id = ? AND request_id = ?
		  AND ((idempotency_key IS NOT NULL AND idempotency_key = ?) OR (kind = ? AND status = 'running'))
		ORDER BY started_at DESC LIMIT 1`+lock, tenantID, run.RequestID, ptrArg(run.IdempotencyKey), string(run.Kind)))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AnalysisRun{}, false, nil
	}
	if err != nil {
		return domain.AnalysisRun{}, false, fmt.Errorf("mysql: find blocking analysis run: %w", err)
	}
	return got, true, nil
}

func leaseMicros(d time.Duration) int64 { return d.Microseconds() }

func (r *AnalysisRunRepository) EnsureProjectGate(ctx context.Context, projectID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ErrRequestTenantRequired()
	}
	// Autocommit on the pool, never the ctx transaction: see the port comment.
	if _, err := r.db.ExecContext(ctx, `INSERT IGNORE INTO analysis_project_gates (tenant_id, project_id) VALUES (?, ?)`, tenantID, projectID); err != nil {
		return fmt.Errorf("mysql: ensure analysis gate: %w", err)
	}
	return nil
}

func (r *AnalysisRunRepository) StartRun(ctx context.Context, run domain.AnalysisRun, draft domain.Solution, opts usecase.StartRunOptions) (usecase.StartRunResult, error) {
	var res usecase.StartRunResult
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if run.TenantID != tenantID || draft.TenantID != tenantID {
			return domain.ErrRequestTenantRequired()
		}
		if existing, found, err := r.findBlocking(ctx, db, tenantID, run, false); err != nil || found {
			res = usecase.StartRunResult{Run: existing}
			return err
		}
		if run.Mode == domain.AnalysisModeAgentReadonly && opts.MaxAgentRuns > 0 {
			// MySQL has no partial index to count on: locking the gate row serialises concurrent starts of one project.
			var locked int
			if err := db.QueryRowContext(ctx, `SELECT 1 FROM analysis_project_gates WHERE tenant_id = ? AND project_id = ? FOR UPDATE`, tenantID, run.ProjectID).Scan(&locked); err != nil {
				return fmt.Errorf("mysql: lock analysis gate (EnsureProjectGate must run first): %w", err)
			}
			// FOR SHARE makes this a current read: a plain count would reuse the transaction's snapshot and miss
			// runs another start committed while this one waited for the gate.
			var running int
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM analysis_runs WHERE tenant_id = ? AND project_id = ? AND mode = 'agent_readonly' AND status = 'running' FOR SHARE`,
				tenantID, run.ProjectID).Scan(&running); err != nil {
				return fmt.Errorf("mysql: count running analyses: %w", err)
			}
			if running >= opts.MaxAgentRuns {
				return domain.ErrAnalysisBusy()
			}
		}
		// No-op update instead of INSERT IGNORE, which would also swallow CHECK and FK errors.
		out, err := db.ExecContext(ctx, `INSERT INTO analysis_runs (id, tenant_id, request_id, kind, mode, status, idempotency_key, attempt,
				lease_owner, lease_expires_at, started_at, solution_id, project_id, actor_id, feedback)
			VALUES (?, ?, ?, ?, ?, 'running', ?, ?, ?, DATE_ADD(CURRENT_TIMESTAMP(6), INTERVAL ? MICROSECOND), CURRENT_TIMESTAMP(6), ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE tenant_id = tenant_id`,
			run.ID, tenantID, run.RequestID, string(run.Kind), string(run.Mode), ptrArg(run.IdempotencyKey), max(run.Attempt, 1),
			derefStr(run.LeaseOwner), leaseMicros(opts.LeaseTTL), nullIfEmpty(run.SolutionID), nullIfEmpty(run.ProjectID), nullIfEmpty(run.ActorID), nullIfEmpty(run.Feedback))
		if err != nil {
			return fmt.Errorf("mysql: insert analysis run: %w", err)
		}
		if n, _ := out.RowsAffected(); n == 0 {
			existing, found, err := r.findBlocking(ctx, db, tenantID, run, true)
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("mysql: analysis run insert conflicted but no blocking run is visible")
			}
			res = usecase.StartRunResult{Run: existing}
			return nil
		}
		if err := insertSolutionRow(ctx, db, tenantID, draft); err != nil {
			return err
		}
		stored, err := scanAnalysisRun(db.QueryRowContext(ctx, `SELECT `+analysisRunColumns+` FROM analysis_runs WHERE tenant_id = ? AND id = ?`, tenantID, run.ID))
		if err != nil {
			return fmt.Errorf("mysql: reread analysis run: %w", err)
		}
		res = usecase.StartRunResult{Run: stored, Created: true}
		return nil
	})
	return res, err
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (r *AnalysisRunRepository) Get(ctx context.Context, runID string) (domain.AnalysisRun, error) {
	var out domain.AnalysisRun
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		got, err := scanAnalysisRun(db.QueryRowContext(ctx, `SELECT `+analysisRunColumns+` FROM analysis_runs WHERE tenant_id = ? AND id = ?`, tenantID, runID))
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrSolutionNotFound(runID)
		}
		out = got
		return err
	})
	return out, err
}

func (r *AnalysisRunRepository) RenewLease(ctx context.Context, runID, owner string, ttl time.Duration) (bool, error) {
	ok := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `UPDATE analysis_runs SET lease_expires_at = DATE_ADD(CURRENT_TIMESTAMP(6), INTERVAL ? MICROSECOND)
			WHERE tenant_id = ? AND id = ? AND status = 'running' AND lease_owner = ?`, leaseMicros(ttl), tenantID, runID, owner)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		ok = n == 1
		return nil
	})
	return ok, err
}

func (r *AnalysisRunRepository) FinishOwned(ctx context.Context, run domain.AnalysisRun, owner string) (bool, error) {
	ok := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `UPDATE analysis_runs
			SET status = ?, attempt = ?, error_code = ?, error_message = ?, raw_output = ?, enforcement = ?, repo_check = ?,
				finished_at = CURRENT_TIMESTAMP(6), lease_owner = NULL, lease_expires_at = NULL
			WHERE tenant_id = ? AND id = ? AND status = 'running' AND lease_owner = ?`,
			string(run.Status), run.Attempt, ptrArg(run.ErrorCode), ptrArg(run.ErrorMessage), ptrArg(run.RawOutput),
			nullIfEmpty(run.Enforcement), nullIfEmpty(run.RepoCheck), tenantID, run.ID, owner)
		if err != nil {
			return fmt.Errorf("mysql: finish analysis run: %w", err)
		}
		n, _ := res.RowsAffected()
		ok = n == 1
		return nil
	})
	return ok, err
}

// ClaimExpired spans tenants (MySQL has no RLS); SKIP LOCKED keeps concurrent sweepers off each other's rows.
func (r *AnalysisRunRepository) ClaimExpired(ctx context.Context, owner string, ttl time.Duration, batch int) ([]domain.AnalysisRun, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mysql: begin claim: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT id FROM analysis_runs WHERE status = 'running' AND lease_expires_at < CURRENT_TIMESTAMP(6)
		ORDER BY lease_expires_at LIMIT ? FOR UPDATE SKIP LOCKED`, batch)
	if err != nil {
		return nil, fmt.Errorf("mysql: select expired analysis runs: %w", err)
	}
	var ids []any
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []domain.AnalysisRun
	if len(ids) > 0 {
		in := strings.Repeat("?,", len(ids)-1) + "?"
		args := append([]any{owner, leaseMicros(ttl)}, ids...)
		if _, err := tx.ExecContext(ctx, `UPDATE analysis_runs SET lease_owner = ?, lease_expires_at = DATE_ADD(CURRENT_TIMESTAMP(6), INTERVAL ? MICROSECOND) WHERE id IN (`+in+`)`, args...); err != nil {
			return nil, fmt.Errorf("mysql: claim analysis runs: %w", err)
		}
		got, err := tx.QueryContext(ctx, `SELECT `+analysisRunColumns+` FROM analysis_runs WHERE id IN (`+in+`)`, ids...)
		if err != nil {
			return nil, err
		}
		for got.Next() {
			run, err := scanAnalysisRun(got)
			if err != nil {
				_ = got.Close()
				return nil, err
			}
			out = append(out, run)
		}
		_ = got.Close()
		if err := got.Err(); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("mysql: commit claim: %w", err)
	}
	return out, nil
}

func (r *AnalysisRunRepository) ListRecent(ctx context.Context, requestID string, limit int) ([]domain.AnalysisRun, error) {
	var out []domain.AnalysisRun
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.QueryContext(ctx, `SELECT `+analysisRunColumns+` FROM analysis_runs
			WHERE tenant_id = ? AND request_id = ? ORDER BY started_at DESC, id LIMIT ?`, tenantID, requestID, limit)
		if err != nil {
			return fmt.Errorf("mysql: list analysis runs: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			run, err := scanAnalysisRun(rows)
			if err != nil {
				return err
			}
			out = append(out, run)
		}
		return rows.Err()
	})
	return out, err
}

func (r *AnalysisRunRepository) CountRunning(ctx context.Context, projectID string, mode domain.AnalysisMode) (int, error) {
	n := 0
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		return db.QueryRowContext(ctx, `SELECT COUNT(*) FROM analysis_runs WHERE tenant_id = ? AND project_id = ? AND mode = ? AND status = 'running'`,
			tenantID, projectID, string(mode)).Scan(&n)
	})
	return n, err
}
