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

type ClassificationRunRepository struct {
	*Repository
}

func NewClassificationRunRepository(r *Repository) *ClassificationRunRepository {
	return &ClassificationRunRepository{Repository: r}
}

var _ usecase.ClassificationRunRepository = (*ClassificationRunRepository)(nil)

const classificationRunColumns = `id, tenant_id, request_id, trigger_name, source_event_id, manual, actor_id, status, claims,
	lease_owner, lease_expires_at, error_code, started_at, finished_at`

func scanClassificationRun(row rowScanner) (domain.ClassificationRun, error) {
	var (
		run      domain.ClassificationRun
		event    sql.NullString
		finished sql.NullTime
		status   string
	)
	err := row.Scan(&run.ID, &run.TenantID, &run.RequestID, &run.Trigger, &event, &run.Manual, &run.ActorID, &status, &run.Claims,
		&run.LeaseOwner, &run.LeaseExpiresAt, &run.ErrorCode, &run.StartedAt, &finished)
	if err != nil {
		return domain.ClassificationRun{}, err
	}
	run.SourceEventID = event.String
	run.Status = domain.ClassificationRunStatus(status)
	run.LeaseExpiresAt, run.StartedAt = run.LeaseExpiresAt.UTC(), run.StartedAt.UTC()
	if finished.Valid {
		t := finished.Time.UTC()
		run.FinishedAt = &t
	}
	return run, nil
}

func (r *ClassificationRunRepository) Start(ctx context.Context, run domain.ClassificationRun) (domain.ClassificationRun, bool, error) {
	var existing domain.ClassificationRun
	started := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		// No-op update instead of INSERT IGNORE, which would also swallow CHECK and FK errors.
		res, err := db.ExecContext(ctx, `
			INSERT INTO classification_runs (id, tenant_id, request_id, trigger_name, source_event_id, manual, actor_id,
				status, claims, lease_owner, lease_expires_at, started_at, active)
			VALUES (?,?,?,?,?,?,?,'running',?,?,?,?,1) ON DUPLICATE KEY UPDATE tenant_id = tenant_id`,
			run.ID, tenantID, run.RequestID, run.Trigger, nullIfEmpty(run.SourceEventID), run.Manual, run.ActorID,
			max(run.Claims, 1), run.LeaseOwner, run.LeaseExpiresAt.UTC(), run.StartedAt.UTC())
		if err != nil {
			return fmt.Errorf("mysql: start classification run: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			started, existing = true, run
			return nil
		}
		existing, err = scanClassificationRun(db.QueryRowContext(ctx, `
			SELECT `+classificationRunColumns+` FROM classification_runs
			WHERE tenant_id = ? AND ((? IS NOT NULL AND source_event_id = ?) OR (request_id = ? AND active = 1))
			ORDER BY started_at DESC LIMIT 1`, tenantID, nullIfEmpty(run.SourceEventID), nullIfEmpty(run.SourceEventID), run.RequestID))
		if err != nil {
			return fmt.Errorf("mysql: find blocking classification run: %w", err)
		}
		return nil
	})
	return existing, started, err
}

func (r *ClassificationRunRepository) Get(ctx context.Context, runID string) (domain.ClassificationRun, error) {
	var run domain.ClassificationRun
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		got, err := scanClassificationRun(db.QueryRowContext(ctx, `SELECT `+classificationRunColumns+` FROM classification_runs
			WHERE tenant_id = ? AND id = ?`, tenantID, runID))
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrRequestNotFound(runID)
		}
		run = got
		return err
	})
	return run, err
}

func (r *ClassificationRunRepository) Renew(ctx context.Context, runID, owner string, until time.Time) (bool, error) {
	ok := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `UPDATE classification_runs SET lease_expires_at = ?
			WHERE tenant_id = ? AND id = ? AND status = 'running' AND lease_owner = ?`, until.UTC(), tenantID, runID, owner)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		ok = n == 1
		return nil
	})
	return ok, err
}

func (r *ClassificationRunRepository) Finish(ctx context.Context, runID string, status domain.ClassificationRunStatus, errCode string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		_, err := db.ExecContext(ctx, `UPDATE classification_runs
			SET status = ?, error_code = ?, finished_at = CURRENT_TIMESTAMP(6), active = NULL, lease_owner = ''
			WHERE tenant_id = ? AND id = ? AND status = 'running'`, string(status), errCode, tenantID, runID)
		return err
	})
}

// ClaimExpired spans tenants (MySQL has no RLS); SKIP LOCKED keeps concurrent sweepers off each other's rows.
func (r *ClassificationRunRepository) ClaimExpired(ctx context.Context, owner string, until time.Time, batch int) ([]domain.ClassificationRun, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mysql: begin claim: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE classification_runs
		SET status = 'failed', error_code = 'LEASE_EXPIRED', finished_at = CURRENT_TIMESTAMP(6), active = NULL, lease_owner = ''
		WHERE status = 'running' AND lease_expires_at < CURRENT_TIMESTAMP(6) AND claims >= ?`, domain.MaxClassificationRunClaims); err != nil {
		return nil, fmt.Errorf("mysql: fail exhausted runs: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM classification_runs WHERE status = 'running' AND lease_expires_at < CURRENT_TIMESTAMP(6)
		ORDER BY lease_expires_at LIMIT ? FOR UPDATE SKIP LOCKED`, batch)
	if err != nil {
		return nil, fmt.Errorf("mysql: select expired runs: %w", err)
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
	var out []domain.ClassificationRun
	if len(ids) > 0 {
		in := strings.Repeat("?,", len(ids)-1) + "?"
		args := append([]any{owner, until.UTC()}, ids...)
		if _, err := tx.ExecContext(ctx, `UPDATE classification_runs SET claims = claims + 1, lease_owner = ?, lease_expires_at = ? WHERE id IN (`+in+`)`, args...); err != nil {
			return nil, fmt.Errorf("mysql: claim runs: %w", err)
		}
		got, err := tx.QueryContext(ctx, `SELECT `+classificationRunColumns+` FROM classification_runs WHERE id IN (`+in+`)`, ids...)
		if err != nil {
			return nil, err
		}
		for got.Next() {
			run, err := scanClassificationRun(got)
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
