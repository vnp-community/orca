// Package domain holds mcp-service's entities. Stdlib only.
package domain

import "time"

// KillSwitch is the tenant-wide emergency stop for MCP tool execution.
type KillSwitch struct {
	Active bool
	Reason string
	At     *time.Time
}

// TenantSettings is one tenant's MCP configuration (mcp.tenant_settings).
type TenantSettings struct {
	TenantID           string
	Enabled            bool
	DCREnabled         bool
	MaxTokenDays       int
	ApprovalTTLSeconds int
	KillSwitch         KillSwitch
	UpdatedBy          string // logical FK to auth-service user; empty = system
	UpdatedAt          time.Time
}

const (
	MaxTokenDaysCeiling   = 90
	MinApprovalTTLSeconds = 30
	MaxApprovalTTLSeconds = 86400
	DefaultApprovalTTL    = 600
)

// DefaultTenantSettings is the row created lazily for a tenant with none.
// Only `enabled` and the token ceiling are configurable (D6); everything
// else is fixed so an env flag can never loosen a safety default.
func DefaultTenantSettings(tenantID string, enabled bool, maxTokenDays int) TenantSettings {
	return TenantSettings{
		TenantID:           tenantID,
		Enabled:            enabled,
		MaxTokenDays:       maxTokenDays,
		ApprovalTTLSeconds: DefaultApprovalTTL,
	}
}

// Validate mirrors the CHECK constraints of mcp.tenant_settings so callers
// get a typed error instead of a constraint violation.
func (s TenantSettings) Validate() error {
	if s.TenantID == "" {
		return ErrTenantSettingsInvalid("tenant id is required")
	}
	if s.MaxTokenDays < 1 || s.MaxTokenDays > MaxTokenDaysCeiling {
		return ErrTenantSettingsInvalid("max token days must be between 1 and 90")
	}
	if s.ApprovalTTLSeconds < MinApprovalTTLSeconds || s.ApprovalTTLSeconds > MaxApprovalTTLSeconds {
		return ErrTenantSettingsInvalid("approval ttl seconds must be between 30 and 86400")
	}
	return nil
}
