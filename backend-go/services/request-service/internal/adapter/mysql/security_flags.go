package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type SecurityFlagRepository struct{ *Repository }

func NewSecurityFlagRepository(r *Repository) *SecurityFlagRepository {
	return &SecurityFlagRepository{Repository: r}
}

var _ usecase.SecurityFlagStore = (*SecurityFlagRepository)(nil)

func (r *SecurityFlagRepository) MarkSecretSuspected(ctx context.Context, requestID string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		_, err := db.ExecContext(ctx, `
			INSERT INTO request_security_flags (tenant_id, request_id, contains_secret_suspected)
			VALUES (?, ?, 1)
			ON DUPLICATE KEY UPDATE contains_secret_suspected = 1`, tenantID, requestID)
		if err != nil {
			return fmt.Errorf("mysql: mark secret suspected: %w", err)
		}
		return nil
	})
}

func (r *SecurityFlagRepository) Get(ctx context.Context, requestID string) (usecase.SecurityFlags, error) {
	var f usecase.SecurityFlags
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		var erasedAt sql.NullTime
		var erasedBy sql.NullString
		err := db.QueryRowContext(ctx, `
			SELECT contains_secret_suspected, erased_at, erased_by
			FROM request_security_flags WHERE tenant_id = ? AND request_id = ?`, tenantID, requestID).
			Scan(&f.ContainsSecretSuspected, &erasedAt, &erasedBy)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("mysql: get security flags: %w", err)
		}
		if erasedAt.Valid {
			t := erasedAt.Time.UTC()
			f.ErasedAt = &t
		}
		f.ErasedBy = erasedBy.String
		return nil
	})
	return f, err
}
