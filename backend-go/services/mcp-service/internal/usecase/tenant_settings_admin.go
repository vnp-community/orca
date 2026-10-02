package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// Admin-facing approval TTL bounds (spec §D: 30..900) are tighter than the
// table CHECK so the UI can't set a multi-hour approval window.
const (
	AdminMinApprovalTTL = 30
	AdminMaxApprovalTTL = 900
)

// SettingsAdmin implements mcp.admin.settings.get/set.
type SettingsAdmin struct {
	repo     PolicyRepository
	kills    KillSwitchRepository
	core     *GovernanceCore
	defaults Defaults
	clock    Clock
}

func NewSettingsAdmin(repo PolicyRepository, kills KillSwitchRepository, core *GovernanceCore, defaults Defaults, clock Clock) *SettingsAdmin {
	return &SettingsAdmin{repo: repo, kills: kills, core: core, defaults: defaults, clock: clock}
}

func (uc *SettingsAdmin) Get(ctx context.Context) (domain.TenantSettings, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return domain.TenantSettings{}, err
	}
	if err := id.requireAdmin(); err != nil {
		return domain.TenantSettings{}, err
	}
	snap, err := uc.repo.LoadPolicySnapshot(ctx, uc.defaults.For(id.TenantID))
	if err != nil {
		return domain.TenantSettings{}, domain.ErrInternal("failed to load settings", err)
	}
	return snap.Settings, nil
}

func (uc *SettingsAdmin) Set(ctx context.Context, patch SettingsPatch) (domain.TenantSettings, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return domain.TenantSettings{}, err
	}
	if err := id.requireAdmin(); err != nil {
		return domain.TenantSettings{}, err
	}
	if patch.Enabled == nil && patch.DCREnabled == nil && patch.MaxTokenDays == nil && patch.ApprovalTTLSeconds == nil {
		return domain.TenantSettings{}, domain.ErrInvalidArgument("no settings to change")
	}
	if patch.MaxTokenDays != nil && (*patch.MaxTokenDays < 1 || *patch.MaxTokenDays > domain.MaxTokenDaysCeiling) {
		return domain.TenantSettings{}, domain.ErrInvalidArgument("maxTokenDays must be between 1 and 90")
	}
	if patch.ApprovalTTLSeconds != nil && (*patch.ApprovalTTLSeconds < AdminMinApprovalTTL || *patch.ApprovalTTLSeconds > AdminMaxApprovalTTL) {
		return domain.TenantSettings{}, domain.ErrInvalidArgument("approvalTtlSeconds must be between 30 and 900")
	}
	now := uc.clock.Now()
	meta := map[string]any{}
	if patch.Enabled != nil {
		meta["enabled"] = *patch.Enabled
	}
	if patch.DCREnabled != nil {
		meta["dcr_enabled"] = *patch.DCREnabled
	}
	if patch.MaxTokenDays != nil {
		meta["max_token_days"] = *patch.MaxTokenDays
	}
	if patch.ApprovalTTLSeconds != nil {
		meta["approval_ttl_seconds"] = *patch.ApprovalTTLSeconds
	}
	changed, err := domain.NewOutboxEvent(uuid.NewString(), domain.SubjectPolicyChanged, id.TenantID, now, map[string]any{"op": "settings"})
	if err != nil {
		return domain.TenantSettings{}, domain.ErrInternal("failed to build event", err)
	}
	audit, err := domain.NewAdminAuditEvent(uuid.NewString(), uuid.NewString(), id.TenantID, id.UserID, domain.AuditActionSettingsUpdate, "mcp_settings", id.TenantID, "allowed", now, meta)
	if err != nil {
		return domain.TenantSettings{}, domain.ErrInternal("failed to build event", err)
	}
	out, err := uc.repo.PatchTenantSettings(ctx, uc.defaults.For(id.TenantID), patch, id.UserID, []domain.OutboxRecord{changed, audit})
	if err != nil {
		return domain.TenantSettings{}, wrapRepoErr(err, "failed to save settings")
	}
	uc.core.InvalidateTenant(id.TenantID)
	return out, nil
}
