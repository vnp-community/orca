package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

const (
	pgForeignKeyViolation = "23503"
	pgUndefinedTable      = "42P01"
	specBatchSize         = usecase.MaxTaskSpecsPerRead
)

const taskSpecColumns = `task_id::text, tenant_id::text, schema_version, spec::text, digest, locked_at, created_at, updated_at, version`

func scanTaskSpec(row pgx.Row) (domain.TaskSpec, error) {
	var s domain.TaskSpec
	var spec string
	if err := row.Scan(&s.TaskID, &s.TenantID, &s.SchemaVersion, &spec, &s.Digest, &s.LockedAt, &s.CreatedAt, &s.UpdatedAt, &s.Version); err != nil {
		return domain.TaskSpec{}, err
	}
	s.Spec = []byte(spec)
	return s, nil
}

// RunInTxWithSpecs is RunInTx plus a TaskSpecRepository on the same transaction; RunInTx itself is unchanged.
func (r *Repository) RunInTxWithSpecs(ctx context.Context, fn func(ctx context.Context, tasks usecase.TaskRepository, edges usecase.EdgeRepository, specs usecase.TaskSpecRepository) error) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		scoped := &Repository{pool: r.pool, db: tx}
		return fn(ctx, scoped, scoped, scoped)
	})
}

func (r *Repository) Upsert(ctx context.Context, s domain.TaskSpec, expectedVersion int64) (domain.TaskSpec, error) {
	var saved domain.TaskSpec
	err := r.inTenantTx(ctx, s.TenantID, func(db dbtx) error {
		var row pgx.Row
		if expectedVersion == 0 {
			row = db.QueryRow(ctx, `
				INSERT INTO task.task_specs (task_id, tenant_id, schema_version, spec, digest)
				VALUES ($1, $2, $3, $4::jsonb, $5)
				ON CONFLICT (task_id) DO NOTHING
				RETURNING `+taskSpecColumns, s.TaskID, s.TenantID, s.SchemaVersion, string(s.Spec), s.Digest)
		} else {
			row = db.QueryRow(ctx, `
				UPDATE task.task_specs
				SET spec = $4::jsonb, digest = $5, schema_version = $3, updated_at = now(), version = version + 1
				WHERE task_id = $1 AND tenant_id = $2 AND version = $6 AND locked_at IS NULL
				RETURNING `+taskSpecColumns, s.TaskID, s.TenantID, s.SchemaVersion, string(s.Spec), s.Digest, expectedVersion)
		}
		got, err := scanTaskSpec(row)
		if err == nil {
			saved = got
			return nil
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return diagnoseSpecWrite(ctx, db, s.TenantID, s.TaskID)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
			return domain.ErrTaskSpecNotFound
		}
		return fmt.Errorf("postgres: upsert task spec: %w", err)
	})
	return saved, err
}

// diagnoseSpecWrite tells apart why a guarded write touched no row.
func diagnoseSpecWrite(ctx context.Context, db dbtx, tenantID, taskID string) error {
	var locked bool
	err := db.QueryRow(ctx, `SELECT locked_at IS NOT NULL FROM task.task_specs WHERE tenant_id = $1 AND task_id = $2`, tenantID, taskID).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrTaskSpecNotFound
	}
	if err != nil {
		return fmt.Errorf("postgres: diagnose task spec write: %w", err)
	}
	if locked {
		return domain.ErrTaskSpecLocked
	}
	return domain.ErrTaskSpecVersionConflict
}

func (r *Repository) GetMany(ctx context.Context, tenantID string, taskIDs []string) ([]domain.TaskSpec, error) {
	var out []domain.TaskSpec
	err := r.inTenantTx(ctx, tenantID, func(db dbtx) error {
		for start := 0; start < len(taskIDs); start += specBatchSize {
			end := min(start+specBatchSize, len(taskIDs))
			rows, err := db.Query(ctx, `SELECT `+taskSpecColumns+` FROM task.task_specs WHERE tenant_id = $1 AND task_id = ANY($2::uuid[])`, tenantID, taskIDs[start:end])
			if err != nil {
				return fmt.Errorf("postgres: get task specs: %w", err)
			}
			for rows.Next() {
				s, err := scanTaskSpec(rows)
				if err != nil {
					rows.Close()
					return fmt.Errorf("postgres: scan task spec: %w", err)
				}
				out = append(out, s)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return fmt.Errorf("postgres: get task specs: %w", err)
			}
		}
		return nil
	})
	return out, err
}

func (r *Repository) LockSubtree(ctx context.Context, tenantID string, taskIDs []string, at time.Time) (int, error) {
	total := 0
	err := r.inTenantTx(ctx, tenantID, func(db dbtx) error {
		for start := 0; start < len(taskIDs); start += specBatchSize {
			end := min(start+specBatchSize, len(taskIDs))
			tag, err := db.Exec(ctx, `UPDATE task.task_specs SET locked_at = $3 WHERE tenant_id = $1 AND task_id = ANY($2::uuid[]) AND locked_at IS NULL`, tenantID, taskIDs[start:end], at)
			if err != nil {
				return fmt.Errorf("postgres: lock task specs: %w", err)
			}
			total += int(tag.RowsAffected())
		}
		return nil
	})
	return total, err
}

func (r *Repository) IsLocked(ctx context.Context, tenantID, taskID string) (bool, error) {
	locked := false
	err := r.inTenantTx(ctx, tenantID, func(db dbtx) error {
		err := db.QueryRow(ctx, `SELECT locked_at IS NOT NULL FROM task.task_specs WHERE tenant_id = $1 AND task_id = $2`, tenantID, taskID).Scan(&locked)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUndefinedTable {
		return false, nil // migration 0020 not applied yet: nothing can be locked
	}
	return locked, err
}

var warnMissingSpecTable sync.Once

// HasSpec answers false (and warns once) when migration 0020 has not run yet, so a rolling
// deploy never breaks Execute for ordinary tasks.
func (r *Repository) HasSpec(ctx context.Context, tenantID, taskID string) (bool, error) {
	has := false
	err := r.inTenantTx(ctx, tenantID, func(db dbtx) error {
		return db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM task.task_specs WHERE tenant_id = $1 AND task_id = $2)`, tenantID, taskID).Scan(&has)
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUndefinedTable {
		warnMissingSpecTable.Do(func() {
			slog.WarnContext(ctx, "task: task_specs table missing, spec routing disabled until migration 0020 runs")
		})
		return false, nil
	}
	return has, err
}

var (
	_ usecase.TaskSpecRepository = (*Repository)(nil)
	_ usecase.SpecTxRunner       = (*Repository)(nil)
)
