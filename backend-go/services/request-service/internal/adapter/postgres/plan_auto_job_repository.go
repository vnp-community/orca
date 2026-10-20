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

var _ usecase.PlanAutoJobRepository = (*PlanAutoJobRepository)(nil)

type PlanAutoJobRepository struct {
	*Repository
}

func NewPlanAutoJobRepository(r *Repository) *PlanAutoJobRepository {
	return &PlanAutoJobRepository{Repository: r}
}

var planAutoJobColumns = planAutoJobColumnsOf("")

const planAutoActiveStates = `('pending','running','committing')`

func scanPlanAutoJob(row pgx.Row) (domain.PlanAutoJob, error) {
	var (
		j                                    domain.PlanAutoJob
		origin, state                        string
		runID, reason, leaseOwner, requester *string
	)
	err := row.Scan(&j.TenantID, &j.RequestID, &j.Anchor, &origin, &j.ProjectID, &state, &j.Attempt, &j.MaxAttempts, &runID,
		&reason, &j.CommitAttempts, &j.NextCheckAt, &leaseOwner, &j.LeaseExpiresAt, &requester, &j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		return domain.PlanAutoJob{}, err
	}
	j.Origin, j.State = domain.PlanAutoOrigin(origin), domain.PlanAutoState(state)
	j.RunID, j.ReasonCode, j.LeaseOwner, j.RequestedBy = derefString(runID), derefString(reason), derefString(leaseOwner), derefString(requester)
	j.NextCheckAt, j.CreatedAt, j.UpdatedAt = j.NextCheckAt.UTC(), j.CreatedAt.UTC(), j.UpdatedAt.UTC()
	if j.LeaseExpiresAt != nil {
		t := j.LeaseExpiresAt.UTC()
		j.LeaseExpiresAt = &t
	}
	return j, nil
}

func timeOrNil(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

// Insert relies on ON CONFLICT DO NOTHING without a target, which covers both the primary key and the one-active-job
// index and, unlike a caught error, leaves an enclosing transaction usable.
func (r *PlanAutoJobRepository) Insert(ctx context.Context, job domain.PlanAutoJob) (bool, error) {
	inserted := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if job.TenantID != tenantID {
			return domain.ErrRequestTenantRequired()
		}
		tag, err := db.Exec(ctx, `
			INSERT INTO request.plan_auto_jobs (tenant_id, request_id, anchor, origin, project_id, state, attempt, max_attempts, run_id,
				reason_code, commit_attempts, next_check_at, requested_by, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, COALESCE($12, now()), $13, COALESCE($14, now()), COALESCE($15, now()))
			ON CONFLICT DO NOTHING`,
			tenantID, job.RequestID, job.Anchor, string(job.Origin), job.ProjectID, string(job.State), max(job.Attempt, 1), max(job.MaxAttempts, 1),
			nullIfEmpty(job.RunID), nullIfEmpty(job.ReasonCode), job.CommitAttempts, timeOrNil(job.NextCheckAt), nullIfEmpty(job.RequestedBy),
			timeOrNil(job.CreatedAt), timeOrNil(job.UpdatedAt))
		if err != nil {
			return fmt.Errorf("postgres: insert plan auto job: %w", err)
		}
		inserted = tag.RowsAffected() == 1
		return nil
	})
	return inserted, err
}

func (r *PlanAutoJobRepository) LatestByRequest(ctx context.Context, requestID string) (domain.PlanAutoJob, bool, error) {
	return r.readOne(ctx, requestID, ``, `ORDER BY created_at DESC, updated_at DESC, anchor`)
}

func (r *PlanAutoJobRepository) ActiveByRequest(ctx context.Context, requestID string) (domain.PlanAutoJob, bool, error) {
	return r.readOne(ctx, requestID, `AND state IN `+planAutoActiveStates, ``)
}

func (r *PlanAutoJobRepository) readOne(ctx context.Context, requestID, where, order string) (domain.PlanAutoJob, bool, error) {
	var job domain.PlanAutoJob
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		got, err := scanPlanAutoJob(db.QueryRow(ctx, `SELECT `+planAutoJobColumns+` FROM request.plan_auto_jobs
			WHERE tenant_id = $1 AND request_id = $2 `+where+` `+order+` LIMIT 1`, tenantID, requestID))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("postgres: read plan auto job: %w", err)
		}
		job, found = got, true
		return nil
	})
	return job, found, err
}

// ClaimDue spans tenants through the relay policy; SKIP LOCKED keeps concurrent workers off each other's rows.
func (r *PlanAutoJobRepository) ClaimDue(ctx context.Context, owner string, ttl time.Duration, batch int) ([]domain.PlanAutoJob, error) {
	var out []domain.PlanAutoJob
	err := r.withRelayTx(ctx, func(ctx context.Context, db dbExecer) error {
		rows, err := db.Query(ctx, `
			WITH due AS (
				SELECT tenant_id, request_id, anchor FROM request.plan_auto_jobs
				WHERE state IN `+planAutoActiveStates+` AND next_check_at <= now() AND (lease_owner IS NULL OR lease_expires_at < now())
				ORDER BY next_check_at LIMIT $1 FOR UPDATE SKIP LOCKED)
			UPDATE request.plan_auto_jobs j SET lease_owner = $2, lease_expires_at = now() + make_interval(secs => $3)
			FROM due WHERE j.tenant_id = due.tenant_id AND j.request_id = due.request_id AND j.anchor = due.anchor
			RETURNING `+planAutoJobColumnsOf("j"), batch, owner, ttl.Seconds())
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			job, err := scanPlanAutoJob(rows)
			if err != nil {
				return err
			}
			out = append(out, job)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: claim plan auto jobs: %w", err)
	}
	return out, nil
}

// planAutoJobColumnsOf lists the columns in scan order; an alias is needed where a joined table has columns of the same name.
func planAutoJobColumnsOf(alias string) string {
	a := ""
	if alias != "" {
		a = alias + "."
	}
	return a + `tenant_id::text, ` + a + `request_id::text, ` + a + `anchor, ` + a + `origin, ` + a + `project_id::text, ` + a + `state, ` +
		a + `attempt, ` + a + `max_attempts, ` + a + `run_id::text, ` + a + `reason_code, ` + a + `commit_attempts, ` + a + `next_check_at, ` +
		a + `lease_owner, ` + a + `lease_expires_at, ` + a + `requested_by::text, ` + a + `created_at, ` + a + `updated_at`
}

func (r *PlanAutoJobRepository) Update(ctx context.Context, job domain.PlanAutoJob, owner string) (bool, error) {
	ok := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(job.RequestID) {
			return nil
		}
		tag, err := db.Exec(ctx, `
			UPDATE request.plan_auto_jobs
			SET state = $5, attempt = $6, run_id = $7, reason_code = $8, commit_attempts = $9, next_check_at = COALESCE($10, now()),
				updated_at = now(), lease_owner = NULL, lease_expires_at = NULL
			WHERE tenant_id = $1 AND request_id = $2 AND anchor = $3 AND lease_owner = $4`,
			tenantID, job.RequestID, job.Anchor, owner, string(job.State), job.Attempt, nullIfEmpty(job.RunID), nullIfEmpty(job.ReasonCode),
			job.CommitAttempts, timeOrNil(job.NextCheckAt))
		if err != nil {
			return fmt.Errorf("postgres: update plan auto job: %w", err)
		}
		ok = tag.RowsAffected() == 1
		return nil
	})
	return ok, err
}
