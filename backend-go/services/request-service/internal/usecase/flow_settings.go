package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// FlowSettings resolves and changes the two-level request_flow_enabled flag:
// effective = REQUEST_FLOW_ENABLED (global) AND the tenant row. Missing row or unreadable row is "off".
type FlowSettings struct {
	repo   FlowSettingsRepository
	global bool
	audit  RPCAuditRecorder
	log    *slog.Logger
}

func NewFlowSettings(repo FlowSettingsRepository, global bool, audit RPCAuditRecorder, log *slog.Logger) *FlowSettings {
	if audit == nil {
		audit = NoopRPCAuditRecorder{}
	}
	if log == nil {
		log = slog.Default()
	}
	return &FlowSettings{repo: repo, global: global, audit: audit, log: log}
}

// Effective returns false together with the read error when the tenant row cannot be read.
func (u *FlowSettings) Effective(ctx context.Context) (bool, error) {
	if !u.global {
		return false, nil
	}
	s, ok, err := u.repo.Get(ctx)
	if err != nil {
		u.log.ErrorContext(ctx, "request flow flag unreadable; treating as disabled", slog.Any("error", err))
		return false, err
	}
	return ok && s.Enabled, nil
}

// Get is the read RPC; an unreadable flag is reported, not defaulted, so the UI does not show a false "off".
func (u *FlowSettings) Get(ctx context.Context) (bool, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return false, domain.ErrRequestTenantRequired()
	}
	return u.Effective(ctx)
}

// Set is admin-only and audited. It returns the effective value, so an admin who enables a tenant while
// the global switch is off sees enabled=false.
func (u *FlowSettings) Set(ctx context.Context, enabled bool) (bool, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return false, domain.ErrRequestTenantRequired()
	}
	actor, _ := tenant.UserID(ctx)
	role, _ := tenant.Role(ctx)
	if role != "admin" {
		u.audit.Record(ctx, RPCAuditEvent{TenantID: tenantID, ActorID: actor, ActorKind: domain.AuditActorUser, Action: domain.ActionRequestFlowSet,
			TargetType: "tenant", TargetID: tenantID, Outcome: domain.AuditOutcomeDenied, Metadata: map[string]string{"enabled": boolString(enabled)}})
		return false, domain.ErrFlowAdminOnly()
	}
	if err := u.repo.Upsert(ctx, enabled, actor); err != nil {
		return false, err
	}
	u.audit.Record(ctx, RPCAuditEvent{TenantID: tenantID, ActorID: actor, ActorKind: domain.AuditActorUser, Action: domain.ActionRequestFlowSet,
		TargetType: "tenant", TargetID: tenantID, Outcome: domain.AuditOutcomeAllowed, Metadata: map[string]string{"enabled": boolString(enabled)}})
	return u.Effective(ctx)
}

func boolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
