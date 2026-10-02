// Package usecase holds mcp-service's application logic. Nothing here
// imports a database driver, so a second SQL dialect only needs a new
// adapter, not usecase changes.
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// TenantSettingsRepository persists mcp.tenant_settings. Tenant scoping is
// by explicit argument; implementations must also enforce it in storage.
type TenantSettingsRepository interface {
	// GetOrCreateTenantSettings returns the tenant's row, inserting defaults
	// first if none exists. An existing row is never overwritten, so later
	// changes to the default flags don't affect tenants that already have one.
	GetOrCreateTenantSettings(ctx context.Context, defaults domain.TenantSettings) (domain.TenantSettings, error)
	// UpdateTenantSettings writes Enabled, DCREnabled, MaxTokenDays,
	// ApprovalTTLSeconds and UpdatedBy for the tenant, creating the row if
	// absent. The kill switch is deliberately not written here.
	UpdateTenantSettings(ctx context.Context, s domain.TenantSettings) (domain.TenantSettings, error)
}

// OutboxWriter enqueues an event transactionally. No usecase emits events
// yet (first producer: BE-MCP-SOL-004); the port exists so adapters and
// the relay are exercised from day one.
type OutboxWriter interface {
	EnqueueOutbox(ctx context.Context, tenantID string, rec domain.OutboxRecord) error
}

// Defaults are the process-level values for lazily created tenant rows.
type Defaults struct {
	TenantEnabled bool
	MaxTokenDays  int
}

func (d Defaults) For(tenantID string) domain.TenantSettings {
	return domain.DefaultTenantSettings(tenantID, d.TenantEnabled, d.MaxTokenDays)
}
