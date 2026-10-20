package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

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

const planAutoJobColumns = `tenant_id, request_id, anchor, origin, project_id, state, attempt, max_attempts, run_id, reason_code,
	commit_attempts, next_check_at, lease_owner, lease_expires_at, requested_by, created_at, updated_at`

const planAutoActiveStates = `('pending','running','committing')`

func scanPlanAutoJob(row rowScanner) (domain.PlanAutoJob, error) {
	var (
		j                                    domain.PlanAutoJob
		origin, state                        string
		runID, reason, leaseOwner, requester sql.NullString
		leaseExpires                         sql.NullTime
	)
	err := row.Scan(&j.TenantID, &j.RequestID, &j.Anchor, &origin, &j.ProjectID, &state, &j.Attempt, &j.MaxAttempts, &runID,
		&reason, &j.CommitAttempts, &j.NextCheckAt, &leaseOwner, &leaseExpires, &requester, &j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		return domain.PlanAutoJob{}, err
	}
	j.Origin, j.State = domain.PlanAutoOrigin(origin), domain.PlanAutoState(state)
	j.RunID, j.ReasonCode, j.LeaseOwner, j.RequestedBy = runID.String, reason.String, leaseOwner.String, requester.String
	j.NextCheckAt, j.CreatedAt, j.UpdatedAt = j.NextCheckAt.UTC(), j.CreatedAt.UTC(), j.UpdatedAt.UTC()
	if leaseExpires.Valid {
		t := leaseExpires.Time.UTC()
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

// Insert takes the duplicate-key error rather than INSERT IGNORE, which would also swallow a foreign-key or data error.
func (r *PlanAutoJobRepository) Insert(ctx context.Context, job domain.PlanAutoJob) (bool, error) {
	inserted := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if job.TenantID != tenantID {
			return domain.ErrRequestTenantRequired()
		}
		_, err := db.ExecContext(ctx, `
			INSERT INTO plan_auto_jobs (tenant_id, request_id, anchor, origin, project_id, state, attempt, max_attempts, run_id,
				reason_code, commit_attempts, next_check_at, requested_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP(6)), ?, COALESCE(?, CURRENT_TIMESTAMP(6)), COALESCE(?, CURRENT_TIMESTAMP(6)))`,
			tenantID, job.RequestID, job.Anchor, string(job.Origin), job.ProjectID, string(job.State), max(job.Attempt, 1), max(job.MaxAttempts, 1),
			nullIfEmpty(job.RunID), nullIfEmpty(job.ReasonCode), job.CommitAttempts, timeOrNil(job.NextCheckAt), nullIfEmpty(job.RequestedBy),
			timeOrNil(job.CreatedAt), timeOrNil(job.UpdatedAt))
		if isDuplicateKey(err, "") {
			return nil
		}
		if err != nil {
			return fmt.Errorf("mysql: insert plan auto job: %w", err)
		}
		inserted = true
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
		got, err := scanPlanAutoJob(db.QueryRowContext(ctx, `SELECT `+planAutoJobColumns+` FROM plan_auto_jobs
			WHERE tenant_id = ? AND request_id = ? `+where+` `+order+` LIMIT 1`, tenantID, requestID))
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("mysql: read plan auto job: %w", err)
		}
		job, found = got, true
		return nil
	})
	return job, found, err
}

// ClaimDue spans tenants (MySQL has no RLS) in its own transaction on the pool, so it must not be called inside the
// caller's transaction; SKIP LOCKED keeps concurrent workers off each other's rows.
func (r *PlanAutoJobRepository) ClaimDue(ctx context.Context, owner string, ttl time.Duration, batch int) ([]domain.PlanAutoJob, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mysql: begin claim: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT tenant_id, request_id, anchor FROM plan_auto_jobs
		WHERE state IN `+planAutoActiveStates+` AND next_check_at <= CURRENT_TIMESTAMP(6) AND (lease_owner IS NULL OR lease_expires_at < CURRENT_TIMESTAMP(6))
		ORDER BY next_check_at LIMIT ? FOR UPDATE SKIP LOCKED`, batch)
	if err != nil {
		return nil, fmt.Errorf("mysql: select due plan auto jobs: %w", err)
	}
	var keys []any
	for rows.Next() {
		var tenantID, requestID, anchor string
		if err := rows.Scan(&tenantID, &requestID, &anchor); err != nil {
			_ = rows.Close()
			return nil, err
		}
		keys = append(keys, tenantID, requestID, anchor)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []domain.PlanAutoJob
	if len(keys) > 0 {
		in := strings.Repeat("(?,?,?),", len(keys)/3-1) + "(?,?,?)"
		args := append([]any{owner, leaseMicros(ttl)}, keys...)
		if _, err := tx.ExecContext(ctx, `UPDATE plan_auto_jobs SET lease_owner = ?, lease_expires_at = DATE_ADD(CURRENT_TIMESTAMP(6), INTERVAL ? MICROSECOND)
			WHERE (tenant_id, request_id, anchor) IN (`+in+`)`, args...); err != nil {
			return nil, fmt.Errorf("mysql: claim plan auto jobs: %w", err)
		}
		got, err := tx.QueryContext(ctx, `SELECT `+planAutoJobColumns+` FROM plan_auto_jobs WHERE (tenant_id, request_id, anchor) IN (`+in+`) ORDER BY next_check_at`, keys...)
		if err != nil {
			return nil, err
		}
		for got.Next() {
			job, err := scanPlanAutoJob(got)
			if err != nil {
				_ = got.Close()
				return nil, err
			}
			out = append(out, job)
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

func (r *PlanAutoJobRepository) Update(ctx context.Context, job domain.PlanAutoJob, owner string) (bool, error) {
	ok := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `
			UPDATE plan_auto_jobs
			SET state = ?, attempt = ?, run_id = ?, reason_code = ?, commit_attempts = ?, next_check_at = COALESCE(?, CURRENT_TIMESTAMP(6)),
				updated_at = CURRENT_TIMESTAMP(6), lease_owner = NULL, lease_expires_at = NULL
			WHERE tenant_id = ? AND request_id = ? AND anchor = ? AND lease_owner = ?`,
			string(job.State), job.Attempt, nullIfEmpty(job.RunID), nullIfEmpty(job.ReasonCode), job.CommitAttempts, timeOrNil(job.NextCheckAt),
			tenantID, job.RequestID, job.Anchor, owner)
		if err != nil {
			return fmt.Errorf("mysql: update plan auto job: %w", err)
		}
		n, _ := res.RowsAffected()
		ok = n == 1
		return nil
	})
	return ok, err
}
