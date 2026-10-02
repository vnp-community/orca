package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

type ServerInfo struct {
	Settings domain.TenantSettings
	Scopes   []domain.ScopeDescriptor
}

type GetServerInfo struct {
	repo     TenantSettingsRepository
	defaults Defaults
}

func NewGetServerInfo(repo TenantSettingsRepository, defaults Defaults) *GetServerInfo {
	return &GetServerInfo{repo: repo, defaults: defaults}
}

func (uc *GetServerInfo) Execute(ctx context.Context) (ServerInfo, error) {
	// Tenant comes only from context, never from the request body.
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ServerInfo{}, domain.ErrNoTenant(err)
	}
	s, err := uc.repo.GetOrCreateTenantSettings(ctx, uc.defaults.For(tenantID))
	if err != nil {
		return ServerInfo{}, domain.ErrInternal("failed to load tenant settings", err)
	}
	return ServerInfo{Settings: s, Scopes: domain.ScopeCatalog()}, nil
}
