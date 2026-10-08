package mysql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// FlowSettingsRepository reads and writes tenant_settings of the ctx tenant (MySQL has no RLS: every query filters tenant_id).
type FlowSettingsRepository struct{ *Repository }

func NewFlowSettingsRepository(r *Repository) *FlowSettingsRepository {
	return &FlowSettingsRepository{Repository: r}
}

var _ usecase.FlowSettingsRepository = (*FlowSettingsRepository)(nil)

func (r *FlowSettingsRepository) Get(ctx context.Context) (domain.FlowSettings, bool, error) {
	var s domain.FlowSettings
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		err := db.QueryRowContext(ctx, `SELECT tenant_id, request_flow_enabled FROM tenant_settings WHERE tenant_id = ?`, tenantID).
			Scan(&s.TenantID, &s.Enabled)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		found = err == nil
		return err
	})
	return s, found, err
}

func (r *FlowSettingsRepository) Upsert(ctx context.Context, enabled bool, updatedBy string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		_, err := db.ExecContext(ctx, `INSERT INTO tenant_settings (tenant_id, request_flow_enabled, updated_by, updated_at)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP(6))
			ON DUPLICATE KEY UPDATE request_flow_enabled = VALUES(request_flow_enabled),
				updated_by = VALUES(updated_by), updated_at = CURRENT_TIMESTAMP(6)`, tenantID, enabled, updatedBy)
		return err
	})
}
