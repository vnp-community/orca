package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// FlowSettingsRepository reads and writes request.tenant_settings of the ctx tenant (RLS enforced).
type FlowSettingsRepository struct{ *Repository }

func NewFlowSettingsRepository(r *Repository) *FlowSettingsRepository {
	return &FlowSettingsRepository{Repository: r}
}

var _ usecase.FlowSettingsRepository = (*FlowSettingsRepository)(nil)

func (r *FlowSettingsRepository) Get(ctx context.Context) (domain.FlowSettings, bool, error) {
	var s domain.FlowSettings
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		err := db.QueryRow(ctx, `SELECT tenant_id::text, request_flow_enabled FROM request.tenant_settings WHERE tenant_id = $1`, tenantID).
			Scan(&s.TenantID, &s.Enabled)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		found = err == nil
		return err
	})
	return s, found, err
}

func (r *FlowSettingsRepository) Upsert(ctx context.Context, enabled bool, updatedBy string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		_, err := db.Exec(ctx, `INSERT INTO request.tenant_settings (tenant_id, request_flow_enabled, updated_by, updated_at)
			VALUES ($1, $2, $3, now())
			ON CONFLICT (tenant_id) DO UPDATE SET request_flow_enabled = EXCLUDED.request_flow_enabled,
				updated_by = EXCLUDED.updated_by, updated_at = now()`, tenantID, enabled, updatedBy)
		return err
	})
}
