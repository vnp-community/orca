package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

func scanAnalysisRun(row pgx.Row) (domain.AnalysisRun, error) {
	var (
		r                                                          domain.AnalysisRun
		kind, mode, status                                         string
		solutionID, projectID, actorID, feedback, enforcement, rep *string
	)
	err := row.Scan(&r.ID, &r.TenantID, &r.RequestID, &kind, &mode, &status, &r.IdempotencyKey, &r.Attempt, &r.LeaseOwner, &r.LeaseExpiresAt,
		&r.ErrorCode, &r.ErrorMessage, &r.RawOutput, &r.StartedAt, &r.FinishedAt, &solutionID, &projectID, &actorID, &feedback, &enforcement, &rep)
	if err != nil {
		return domain.AnalysisRun{}, err
	}
	r.Kind, r.Mode, r.Status = domain.RunKind(kind), domain.AnalysisMode(mode), domain.RunStatus(status)
	r.SolutionID, r.ProjectID, r.ActorID = derefString(solutionID), derefString(projectID), derefString(actorID)
	r.Feedback, r.Enforcement, r.RepoCheck = derefString(feedback), derefString(enforcement), derefString(rep)
	r.StartedAt = r.StartedAt.UTC()
	if r.LeaseExpiresAt != nil {
		t := r.LeaseExpiresAt.UTC()
		r.LeaseExpiresAt = &t
	}
	if r.FinishedAt != nil {
		t := r.FinishedAt.UTC()
		r.FinishedAt = &t
	}
	return r, nil
}

func (r *AnalysisRunRepository) findBlocking(ctx context.Context, db dbExecer, tenantID string, run domain.AnalysisRun) (domain.AnalysisRun, bool, error) {
	got, err := scanAnalysisRun(db.QueryRow(ctx, `SELECT `+analysisRunColumns+` FROM request.analysis_runs
		WHERE tenant_id = $1 AND request_id = $2
		  AND ((idempotency_key IS NOT NULL AND idempotency_key = $3::text) OR (kind = $4 AND status = 'running'))
		ORDER BY started_at DESC LIMIT 1`, tenantID, run.RequestID, run.IdempotencyKey, string(run.Kind)))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AnalysisRun{}, false, nil
	}
	if err != nil {
		return domain.AnalysisRun{}, false, fmt.Errorf("postgres: find blocking analysis run: %w", err)
	}
	return got, true, nil
}

func (r *AnalysisRunRepository) EnsureProjectGate(ctx context.Context, projectID string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, err := db.Exec(ctx, `INSERT INTO request.analysis_project_gates (tenant_id, project_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, tenantID, projectID); err != nil {
			return fmt.Errorf("postgres: ensure analysis gate: %w", err)
		}
		return nil
	})
}

func (r *AnalysisRunRepository) StartRun(ctx context.Context, run domain.AnalysisRun, draft domain.Solution, opts usecase.StartRunOptions) (usecase.StartRunResult, error) {
	var res usecase.StartRunResult
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if run.TenantID != tenantID || draft.TenantID != tenantID {
			return domain.ErrRequestTenantRequired()
		}
		if existing, found, err := r.findBlocking(ctx, db, tenantID, run); err != nil || found {
			res = usecase.StartRunResult{Run: existing}
			return err
		}
		if run.Mode == domain.AnalysisModeAgentReadonly && opts.MaxAgentRuns > 0 {
			// The gate row serialises concurrent starts of one project: count-then-insert would race without it.
			var locked int
			if err := db.QueryRow(ctx, `SELECT 1 FROM request.analysis_project_gates WHERE tenant_id = $1 AND project_id = $2 FOR UPDATE`, tenantID, run.ProjectID).Scan(&locked); err != nil {
				return fmt.Errorf("postgres: lock analysis gate (EnsureProjectGate must run first): %w", err)
			}
			var running int
			if err := db.QueryRow(ctx, `SELECT count(*) FROM request.analysis_runs WHERE tenant_id = $1 AND project_id = $2 AND mode = 'agent_readonly' AND status = 'running'`,
				tenantID, run.ProjectID).Scan(&running); err != nil {
				return fmt.Errorf("postgres: count running analyses: %w", err)
			}
			if running >= opts.MaxAgentRuns {
				return domain.ErrAnalysisBusy()
			}
		}
		tag, err := db.Exec(ctx, `INSERT INTO request.analysis_runs (id, tenant_id, request_id, kind, mode, status, idempotency_key, attempt,
				lease_owner, lease_expires_at, started_at, solution_id, project_id, actor_id, feedback)
			VALUES ($1, $2, $3, $4, $5, 'running', $6, $7, $8, now() + make_interval(secs => $9), now(), $10, $11, $12, $13)
			ON CONFLICT DO NOTHING`,
			run.ID, tenantID, run.RequestID, string(run.Kind), string(run.Mode), run.IdempotencyKey, max(run.Attempt, 1),
			derefString(run.LeaseOwner), opts.LeaseTTL.Seconds(), nullIfEmpty(run.SolutionID), nullIfEmpty(run.ProjectID), nullIfEmpty(run.ActorID), nullIfEmpty(run.Feedback))
		if err != nil {
			return fmt.Errorf("postgres: insert analysis run: %w", err)
		}
		if tag.RowsAffected() == 0 {
			// A concurrent start committed between the lookup and the insert; its run wins.
			existing, found, err := r.findBlocking(ctx, db, tenantID, run)
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("postgres: analysis run insert conflicted but no blocking run is visible")
			}
			res = usecase.StartRunResult{Run: existing}
			return nil
		}
		if err := insertSolutionRow(ctx, db, tenantID, draft); err != nil {
			return err
		}
		stored, err := scanAnalysisRun(db.QueryRow(ctx, `SELECT `+analysisRunColumns+` FROM request.analysis_runs WHERE tenant_id = $1 AND id = $2`, tenantID, run.ID))
		if err != nil {
			return fmt.Errorf("postgres: reread analysis run: %w", err)
		}
		res = usecase.StartRunResult{Run: stored, Created: true}
		return nil
	})
	return res, err
}

func (r *AnalysisRunRepository) Get(ctx context.Context, runID string) (domain.AnalysisRun, error) {
	var out domain.AnalysisRun
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, perr := uuid.Parse(runID); perr != nil {
			return domain.ErrSolutionNotFound(runID)
		}
		got, err := scanAnalysisRun(db.QueryRow(ctx, `SELECT `+analysisRunColumns+` FROM request.analysis_runs WHERE tenant_id = $1 AND id = $2`, tenantID, runID))
		if errors.Is(err, pgx.ErrNoRows) {
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
		tag, err := db.Exec(ctx, `UPDATE request.analysis_runs SET lease_expires_at = now() + make_interval(secs => $4)
			WHERE tenant_id = $1 AND id = $2 AND status = 'running' AND lease_owner = $3`, tenantID, runID, owner, ttl.Seconds())
		ok = err == nil && tag.RowsAffected() == 1
		return err
	})
	return ok, err
}

func (r *AnalysisRunRepository) FinishOwned(ctx context.Context, run domain.AnalysisRun, owner string) (bool, error) {
	ok := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `UPDATE request.analysis_runs
			SET status = $4, attempt = $5, error_code = $6, error_message = $7, raw_output = $8, enforcement = $9, repo_check = $10,
				finished_at = now(), lease_owner = NULL, lease_expires_at = NULL
			WHERE tenant_id = $1 AND id = $2 AND status = 'running' AND lease_owner = $3`,
			tenantID, run.ID, owner, string(run.Status), run.Attempt, run.ErrorCode, run.ErrorMessage, run.RawOutput,
			nullIfEmpty(run.Enforcement), nullIfEmpty(run.RepoCheck))
		if err != nil {
			return fmt.Errorf("postgres: finish analysis run: %w", err)
		}
		ok = tag.RowsAffected() == 1
		return nil
	})
	return ok, err
}

func (r *AnalysisRunRepository) ClaimExpired(ctx context.Context, owner string, ttl time.Duration, batch int) ([]domain.AnalysisRun, error) {
	var out []domain.AnalysisRun
	err := r.withRelayTx(ctx, func(ctx context.Context, db dbExecer) error {
		rows, err := db.Query(ctx, `
			WITH due AS (
				SELECT id FROM request.analysis_runs WHERE status = 'running' AND lease_expires_at < now()
				ORDER BY lease_expires_at LIMIT $1 FOR UPDATE SKIP LOCKED)
			UPDATE request.analysis_runs a SET lease_owner = $2, lease_expires_at = now() + make_interval(secs => $3)
			FROM due WHERE a.id = due.id
			RETURNING a.id, a.tenant_id, a.request_id, a.kind, a.mode, a.status, a.idempotency_key, a.attempt, a.lease_owner, a.lease_expires_at,
				a.error_code, a.error_message, a.raw_output, a.started_at, a.finished_at, a.solution_id, a.project_id, a.actor_id, a.feedback, a.enforcement, a.repo_check`,
			batch, owner, ttl.Seconds())
		if err != nil {
			return err
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
	if err != nil {
		return nil, fmt.Errorf("postgres: claim expired analysis runs: %w", err)
	}
	return out, nil
}

func (r *AnalysisRunRepository) ListRecent(ctx context.Context, requestID string, limit int) ([]domain.AnalysisRun, error) {
	var out []domain.AnalysisRun
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, perr := uuid.Parse(requestID); perr != nil {
			return nil
		}
		rows, err := db.Query(ctx, `SELECT `+analysisRunColumns+` FROM request.analysis_runs
			WHERE tenant_id = $1 AND request_id = $2 ORDER BY started_at DESC, id LIMIT $3`, tenantID, requestID, limit)
		if err != nil {
			return fmt.Errorf("postgres: list analysis runs: %w", err)
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
		if _, perr := uuid.Parse(projectID); perr != nil {
			return nil
		}
		return db.QueryRow(ctx, `SELECT count(*) FROM request.analysis_runs WHERE tenant_id = $1 AND project_id = $2 AND mode = $3 AND status = 'running'`,
			tenantID, projectID, string(mode)).Scan(&n)
	})
	return n, err
}
