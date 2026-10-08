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

type ClassificationRunRepository struct {
	*Repository
}

func NewClassificationRunRepository(r *Repository) *ClassificationRunRepository {
	return &ClassificationRunRepository{Repository: r}
}

var _ usecase.ClassificationRunRepository = (*ClassificationRunRepository)(nil)

const classificationRunColumns = `id, tenant_id, request_id, trigger_name, source_event_id, manual, actor_id, status, claims,
	lease_owner, lease_expires_at, error_code, started_at, finished_at`

func scanClassificationRun(row pgx.Row) (domain.ClassificationRun, error) {
	var (
		run    domain.ClassificationRun
		event  *string
		status string
	)
	err := row.Scan(&run.ID, &run.TenantID, &run.RequestID, &run.Trigger, &event, &run.Manual, &run.ActorID, &status, &run.Claims,
		&run.LeaseOwner, &run.LeaseExpiresAt, &run.ErrorCode, &run.StartedAt, &run.FinishedAt)
	if err != nil {
		return domain.ClassificationRun{}, err
	}
	run.SourceEventID = derefString(event)
	run.Status = domain.ClassificationRunStatus(status)
	run.LeaseExpiresAt, run.StartedAt = run.LeaseExpiresAt.UTC(), run.StartedAt.UTC()
	return run, nil
}

func (r *ClassificationRunRepository) Start(ctx context.Context, run domain.ClassificationRun) (domain.ClassificationRun, bool, error) {
	var existing domain.ClassificationRun
	started := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `
			INSERT INTO request.classification_runs (id, tenant_id, request_id, trigger_name, source_event_id, manual, actor_id,
				status, claims, lease_owner, lease_expires_at, started_at, active)
			VALUES ($1,$2,$3,$4,$5,$6,$7,'running',$11,$8,$9,$10,1) ON CONFLICT DO NOTHING`,
			run.ID, tenantID, run.RequestID, run.Trigger, nullIfEmpty(run.SourceEventID), run.Manual, run.ActorID,
			run.LeaseOwner, run.LeaseExpiresAt, run.StartedAt, max(run.Claims, 1))
		if err != nil {
			return fmt.Errorf("postgres: start classification run: %w", err)
		}
		if tag.RowsAffected() == 1 {
			started, existing = true, run
			return nil
		}
		existing, err = scanClassificationRun(db.QueryRow(ctx, `
			SELECT `+classificationRunColumns+` FROM request.classification_runs
			WHERE tenant_id = $1 AND (($2::uuid IS NOT NULL AND source_event_id = $2::uuid) OR (request_id = $3 AND active = 1))
			ORDER BY started_at DESC LIMIT 1`, tenantID, nullIfEmpty(run.SourceEventID), run.RequestID))
		if err != nil {
			return fmt.Errorf("postgres: find blocking classification run: %w", err)
		}
		return nil
	})
	return existing, started, err
}

func (r *ClassificationRunRepository) Get(ctx context.Context, runID string) (domain.ClassificationRun, error) {
	var run domain.ClassificationRun
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		got, err := scanClassificationRun(db.QueryRow(ctx, `SELECT `+classificationRunColumns+` FROM request.classification_runs
			WHERE tenant_id = $1 AND id = $2`, tenantID, runID))
		if errors.Is(err, pgx.ErrNoRows) {
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
		tag, err := db.Exec(ctx, `UPDATE request.classification_runs SET lease_expires_at = $4
			WHERE tenant_id = $1 AND id = $2 AND status = 'running' AND lease_owner = $3`, tenantID, runID, owner, until)
		ok = err == nil && tag.RowsAffected() == 1
		return err
	})
	return ok, err
}

func (r *ClassificationRunRepository) Finish(ctx context.Context, runID string, status domain.ClassificationRunStatus, errCode string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		_, err := db.Exec(ctx, `UPDATE request.classification_runs
			SET status = $3, error_code = $4, finished_at = now(), active = NULL, lease_owner = ''
			WHERE tenant_id = $1 AND id = $2 AND status = 'running'`, tenantID, runID, string(status), errCode)
		return err
	})
}

func (r *ClassificationRunRepository) ClaimExpired(ctx context.Context, owner string, until time.Time, batch int) ([]domain.ClassificationRun, error) {
	var out []domain.ClassificationRun
	err := r.withRelayTx(ctx, func(ctx context.Context, db dbExecer) error {
		if _, err := db.Exec(ctx, `UPDATE request.classification_runs
			SET status = 'failed', error_code = 'LEASE_EXPIRED', finished_at = now(), active = NULL, lease_owner = ''
			WHERE status = 'running' AND lease_expires_at < now() AND claims >= $1`, domain.MaxClassificationRunClaims); err != nil {
			return err
		}
		rows, err := db.Query(ctx, `
			WITH due AS (
				SELECT id FROM request.classification_runs WHERE status = 'running' AND lease_expires_at < now()
				ORDER BY lease_expires_at LIMIT $1 FOR UPDATE SKIP LOCKED)
			UPDATE request.classification_runs c SET claims = c.claims + 1, lease_owner = $2, lease_expires_at = $3
			FROM due WHERE c.id = due.id
			RETURNING c.id, c.tenant_id, c.request_id, c.trigger_name, c.source_event_id, c.manual, c.actor_id, c.status, c.claims,
				c.lease_owner, c.lease_expires_at, c.error_code, c.started_at, c.finished_at`, batch, owner, until)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			run, err := scanClassificationRun(rows)
			if err != nil {
				return err
			}
			out = append(out, run)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: claim expired classification runs: %w", err)
	}
	return out, nil
}
