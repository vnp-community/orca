package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type SecurityFlagRepository struct{ *Repository }

func NewSecurityFlagRepository(r *Repository) *SecurityFlagRepository {
	return &SecurityFlagRepository{Repository: r}
}

var _ usecase.SecurityFlagStore = (*SecurityFlagRepository)(nil)

func (r *SecurityFlagRepository) MarkSecretSuspected(ctx context.Context, requestID string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		_, err := db.Exec(ctx, `
			INSERT INTO request.request_security_flags (tenant_id, request_id, contains_secret_suspected)
			VALUES ($1, $2, TRUE)
			ON CONFLICT (tenant_id, request_id) DO UPDATE SET contains_secret_suspected = TRUE`, tenantID, requestID)
		if err != nil {
			return fmt.Errorf("postgres: mark secret suspected: %w", err)
		}
		return nil
	})
}

func (r *SecurityFlagRepository) Get(ctx context.Context, requestID string) (usecase.SecurityFlags, error) {
	var f usecase.SecurityFlags
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		var erasedAt *time.Time
		var erasedBy *string
		err := db.QueryRow(ctx, `
			SELECT contains_secret_suspected, erased_at, erased_by::text
			FROM request.request_security_flags WHERE tenant_id = $1 AND request_id = $2::uuid`, tenantID, requestID).
			Scan(&f.ContainsSecretSuspected, &erasedAt, &erasedBy)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("postgres: get security flags: %w", err)
		}
		f.ErasedAt = erasedAt
		if erasedBy != nil {
			f.ErasedBy = *erasedBy
		}
		return nil
	})
	return f, err
}
