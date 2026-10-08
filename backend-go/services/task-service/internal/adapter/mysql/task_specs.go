package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	drivermysql "github.com/go-sql-driver/mysql"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

const (
	mysqlFKFailsChild  = 1452 // ER_NO_REFERENCED_ROW_2
	mysqlNoSuchTable   = 1146 // ER_NO_SUCH_TABLE
	specBatchSize      = usecase.MaxTaskSpecsPerRead
	taskSpecColumnList = `task_id, tenant_id, schema_version, CAST(spec AS CHAR), digest, locked_at, created_at, updated_at, version`
)

func scanTaskSpec(row rowScanner) (domain.TaskSpec, error) {
	var s domain.TaskSpec
	var spec string
	var locked sql.NullTime
	if err := row.Scan(&s.TaskID, &s.TenantID, &s.SchemaVersion, &spec, &s.Digest, &locked, &s.CreatedAt, &s.UpdatedAt, &s.Version); err != nil {
		return domain.TaskSpec{}, err
	}
	s.Spec = []byte(spec)
	if locked.Valid {
		t := locked.Time
		s.LockedAt = &t
	}
	return s, nil
}

// inTx joins the open transaction (inside RunInTx*) or opens a short one.
func (r *Repository) inTx(ctx context.Context, fn func(db dbtx) error) error {
	if _, ok := r.db.(*sql.Tx); ok {
		return fn(r.db)
	}
	tx, err := r.pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// RunInTxWithSpecs is RunInTx plus a TaskSpecRepository on the same transaction; RunInTx itself is unchanged.
func (r *Repository) RunInTxWithSpecs(ctx context.Context, fn func(ctx context.Context, tasks usecase.TaskRepository, edges usecase.EdgeRepository, specs usecase.TaskSpecRepository) error) error {
	tx, err := r.pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	scoped := &Repository{pool: r.pool, db: tx}
	if err := fn(ctx, scoped, scoped, scoped); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r *Repository) Upsert(ctx context.Context, s domain.TaskSpec, expectedVersion int64) (domain.TaskSpec, error) {
	var saved domain.TaskSpec
	err := r.inTx(ctx, func(db dbtx) error {
		if expectedVersion == 0 {
			_, err := db.ExecContext(ctx, `INSERT INTO task_specs (task_id, tenant_id, schema_version, spec, digest) VALUES (?, ?, ?, ?, ?)`,
				s.TaskID, s.TenantID, s.SchemaVersion, string(s.Spec), s.Digest)
			var myErr *drivermysql.MySQLError
			switch {
			case err == nil:
			case errors.As(err, &myErr) && myErr.Number == mysqlDuplicateEntry:
				return diagnoseSpecWrite(ctx, db, s.TenantID, s.TaskID)
			case errors.As(err, &myErr) && myErr.Number == mysqlFKFailsChild:
				return domain.ErrTaskSpecNotFound
			default:
				return fmt.Errorf("mysql: insert task spec: %w", err)
			}
		} else {
			res, err := db.ExecContext(ctx, `
				UPDATE task_specs SET spec = ?, digest = ?, schema_version = ?, updated_at = CURRENT_TIMESTAMP(6), version = version + 1
				WHERE task_id = ? AND tenant_id = ? AND version = ? AND locked_at IS NULL`,
				string(s.Spec), s.Digest, s.SchemaVersion, s.TaskID, s.TenantID, expectedVersion)
			if err != nil {
				return fmt.Errorf("mysql: update task spec: %w", err)
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return diagnoseSpecWrite(ctx, db, s.TenantID, s.TaskID)
			}
		}
		got, err := scanTaskSpec(db.QueryRowContext(ctx, `SELECT `+taskSpecColumnList+` FROM task_specs WHERE tenant_id = ? AND task_id = ?`, s.TenantID, s.TaskID))
		if err != nil {
			return fmt.Errorf("mysql: reread task spec: %w", err)
		}
		saved = got
		return nil
	})
	return saved, err
}

// diagnoseSpecWrite tells apart why a guarded write touched no row.
func diagnoseSpecWrite(ctx context.Context, db dbtx, tenantID, taskID string) error {
	var locked bool
	err := db.QueryRowContext(ctx, `SELECT locked_at IS NOT NULL FROM task_specs WHERE tenant_id = ? AND task_id = ?`, tenantID, taskID).Scan(&locked)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrTaskSpecNotFound
	}
	if err != nil {
		return fmt.Errorf("mysql: diagnose task spec write: %w", err)
	}
	if locked {
		return domain.ErrTaskSpecLocked
	}
	return domain.ErrTaskSpecVersionConflict
}

func inClause(n int) string { return strings.TrimSuffix(strings.Repeat("?,", n), ",") }

func toAny(ids []string) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

func (r *Repository) GetMany(ctx context.Context, tenantID string, taskIDs []string) ([]domain.TaskSpec, error) {
	var out []domain.TaskSpec
	for start := 0; start < len(taskIDs); start += specBatchSize {
		batch := taskIDs[start:min(start+specBatchSize, len(taskIDs))]
		rows, err := r.db.QueryContext(ctx, `SELECT `+taskSpecColumnList+` FROM task_specs WHERE tenant_id = ? AND task_id IN (`+inClause(len(batch))+`)`,
			append([]any{tenantID}, toAny(batch)...)...)
		if err != nil {
			return nil, fmt.Errorf("mysql: get task specs: %w", err)
		}
		for rows.Next() {
			s, err := scanTaskSpec(rows)
			if err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("mysql: scan task spec: %w", err)
			}
			out = append(out, s)
		}
		_ = rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("mysql: get task specs: %w", err)
		}
	}
	return out, nil
}

func (r *Repository) LockSubtree(ctx context.Context, tenantID string, taskIDs []string, at time.Time) (int, error) {
	total := 0
	err := r.inTx(ctx, func(db dbtx) error {
		for start := 0; start < len(taskIDs); start += specBatchSize {
			batch := taskIDs[start:min(start+specBatchSize, len(taskIDs))]
			res, err := db.ExecContext(ctx, `UPDATE task_specs SET locked_at = ? WHERE tenant_id = ? AND task_id IN (`+inClause(len(batch))+`) AND locked_at IS NULL`,
				append([]any{at.UTC(), tenantID}, toAny(batch)...)...)
			if err != nil {
				return fmt.Errorf("mysql: lock task specs: %w", err)
			}
			n, _ := res.RowsAffected()
			total += int(n)
		}
		return nil
	})
	return total, err
}

func (r *Repository) IsLocked(ctx context.Context, tenantID, taskID string) (bool, error) {
	var locked bool
	err := r.db.QueryRowContext(ctx, `SELECT locked_at IS NOT NULL FROM task_specs WHERE tenant_id = ? AND task_id = ?`, tenantID, taskID).Scan(&locked)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	var myErr *drivermysql.MySQLError
	if errors.As(err, &myErr) && myErr.Number == mysqlNoSuchTable {
		return false, nil // migration 0020 not applied yet: nothing can be locked
	}
	return locked, err
}

var warnMissingSpecTable sync.Once

// HasSpec answers false (and warns once) when migration 0020 has not run yet.
func (r *Repository) HasSpec(ctx context.Context, tenantID, taskID string) (bool, error) {
	var has bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM task_specs WHERE tenant_id = ? AND task_id = ?)`, tenantID, taskID).Scan(&has)
	var myErr *drivermysql.MySQLError
	if errors.As(err, &myErr) && myErr.Number == mysqlNoSuchTable {
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
