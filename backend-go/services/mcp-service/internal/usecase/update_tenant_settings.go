package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// UpdateTenantSettingsInput is a partial update: nil fields keep the stored value.
type UpdateTenantSettingsInput struct {
	Enabled            *bool
	DCREnabled         *bool
	MaxTokenDays       *int
	ApprovalTTLSeconds *int
}

// UpdateTenantSettings has no gRPC RPC yet; the admin channel that calls it
// (and its role check) arrives with BE-MCP-SOL-003/013.
type UpdateTenantSettings struct {
	repo     TenantSettingsRepository
	defaults Defaults
}

func NewUpdateTenantSettings(repo TenantSettingsRepository, defaults Defaults) *UpdateTenantSettings {
	return &UpdateTenantSettings{repo: repo, defaults: defaults}
}

func (uc *UpdateTenantSettings) Execute(ctx context.Context, in UpdateTenantSettingsInput) (domain.TenantSettings, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.TenantSettings{}, domain.ErrNoTenant(err)
	}
	s, err := uc.repo.GetOrCreateTenantSettings(ctx, uc.defaults.For(tenantID))
	if err != nil {
		return domain.TenantSettings{}, domain.ErrInternal("failed to load tenant settings", err)
	}
	if in.Enabled != nil {
		s.Enabled = *in.Enabled
	}
	if in.DCREnabled != nil {
		s.DCREnabled = *in.DCREnabled
	}
	if in.MaxTokenDays != nil {
		s.MaxTokenDays = *in.MaxTokenDays
	}
	if in.ApprovalTTLSeconds != nil {
		s.ApprovalTTLSeconds = *in.ApprovalTTLSeconds
	}
	s.UpdatedBy, _ = tenant.UserID(ctx)
	if err := s.Validate(); err != nil {
		return domain.TenantSettings{}, err
	}
	out, err := uc.repo.UpdateTenantSettings(ctx, s)
	if err != nil {
		return domain.TenantSettings{}, domain.ErrInternal("failed to save tenant settings", err)
	}
	return out, nil
}
