package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type RequestCheckRepository struct {
	*Repository
}

func NewRequestCheckRepository(r *Repository) *RequestCheckRepository {
	return &RequestCheckRepository{Repository: r}
}

var _ usecase.RequestCheckRepository = (*RequestCheckRepository)(nil)

const requestCheckColumns = `id, request_id, kind, status, metrics, summary, source, task_id, recorded_by, created_at`

// Append has no RETURNING in MySQL, so the stored row is read back by id inside the same transaction.
func (r *RequestCheckRepository) Append(ctx context.Context, c domain.RequestCheck) (domain.RequestCheck, error) {
	var out domain.RequestCheck
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		metrics := string(c.Metrics)
		if metrics == "" {
			metrics = "{}" // the column has no default: MySQL JSON defaults need expression syntax
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO request_checks (id, tenant_id, request_id, kind, status, metrics, summary, source, task_id, recorded_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			c.ID, tenantID, c.RequestID, string(c.Kind), string(c.Status), metrics, c.Summary, string(c.Source), nullIfEmpty(c.TaskID), nullIfEmpty(c.RecordedBy)); err != nil {
			return fmt.Errorf("mysql: append request check: %w", err)
		}
		var err error
		out, err = scanRequestCheck(db.QueryRowContext(ctx, `SELECT `+requestCheckColumns+` FROM request_checks WHERE tenant_id = ? AND id = ?`, tenantID, c.ID), tenantID)
		return err
	})
	return out, err
}

type checkRowScanner interface{ Scan(dest ...any) error }

func scanRequestCheck(row checkRowScanner, tenantID string) (domain.RequestCheck, error) {
	var c domain.RequestCheck
	var kind, status, source string
	var metrics []byte
	var taskID, recordedBy sql.NullString
	if err := row.Scan(&c.ID, &c.RequestID, &kind, &status, &metrics, &c.Summary, &source, &taskID, &recordedBy, &c.CreatedAt); err != nil {
		return domain.RequestCheck{}, err
	}
	c.TenantID, c.Kind, c.Status, c.Source, c.Metrics = tenantID, domain.CheckKind(kind), domain.CheckStatus(status), domain.CheckSource(source), metrics
	c.TaskID, c.RecordedBy, c.CreatedAt = taskID.String, recordedBy.String, c.CreatedAt.UTC()
	return c, nil
}

func (r *RequestCheckRepository) ListByRequest(ctx context.Context, requestID string) ([]domain.RequestCheck, error) {
	var out []domain.RequestCheck
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.QueryContext(ctx, `SELECT `+requestCheckColumns+` FROM request_checks
			WHERE tenant_id = ? AND request_id = ? ORDER BY created_at, id`, tenantID, requestID)
		if err != nil {
			return fmt.Errorf("mysql: list request checks: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			c, err := scanRequestCheck(rows, tenantID)
			if err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}

func (r *RequestCheckRepository) Latest(ctx context.Context, requestID string, kind domain.CheckKind) (domain.RequestCheck, bool, error) {
	var out domain.RequestCheck
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.QueryContext(ctx, `SELECT `+requestCheckColumns+` FROM request_checks
			WHERE tenant_id = ? AND request_id = ? AND kind = ? ORDER BY created_at DESC, id DESC LIMIT 1`, tenantID, requestID, string(kind))
		if err != nil {
			return fmt.Errorf("mysql: latest request check: %w", err)
		}
		defer rows.Close()
		if rows.Next() {
			if out, err = scanRequestCheck(rows, tenantID); err != nil {
				return err
			}
			found = true
		}
		return rows.Err()
	})
	return out, found, err
}
