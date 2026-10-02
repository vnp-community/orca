package usecase

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

type SetKillSwitchInput struct {
	Scope, TargetID, Reason string
	Active                  bool
}

// KillSwitchAdmin sets and lists kill switches (admin only).
type KillSwitchAdmin struct {
	kills   KillSwitchRepository
	clients ClientStatusReader
	core    *GovernanceCore
	red     domain.Redactor
	clock   Clock
}

func NewKillSwitchAdmin(kills KillSwitchRepository, clients ClientStatusReader, core *GovernanceCore, red domain.Redactor, clock Clock) *KillSwitchAdmin {
	return &KillSwitchAdmin{kills: kills, clients: clients, core: core, red: red, clock: clock}
}

func (uc *KillSwitchAdmin) Set(ctx context.Context, in SetKillSwitchInput) error {
	id, err := userCaller(ctx)
	if err != nil {
		return err
	}
	if err := id.requireAdmin(); err != nil {
		return err
	}
	e := domain.KillSwitchEntry{ID: uuid.NewString(), TenantID: id.TenantID, Scope: in.Scope, TargetID: in.TargetID, Active: in.Active, Reason: in.Reason, SetBy: id.UserID}
	if err := e.Validate(); err != nil {
		return err
	}
	if err := uc.checkTarget(ctx, e); err != nil {
		return err
	}
	if e.Scope == domain.KillScopeTenant {
		// The tenant_settings row must exist (created with the D6 default)
		// before the switch is mirrored into it.
		if _, err := uc.core.snapshot(ctx, id.TenantID); err != nil {
			return domain.ErrInternal("failed to load settings", err)
		}
	}
	now := uc.clock.Now()
	e.SetAt = now
	reason := e.Reason
	if uc.red != nil {
		reason, _ = uc.red.Redact(reason)
	}
	changed, err := domain.NewOutboxEvent(uuid.NewString(), domain.SubjectKillSwitchChanged, id.TenantID, now, map[string]any{
		"scope": e.Scope, "target_id": e.TargetID, "active": e.Active, "reason": reason,
	})
	if err != nil {
		return domain.ErrInternal("failed to build event", err)
	}
	audit, err := domain.NewAdminAuditEvent(uuid.NewString(), uuid.NewString(), id.TenantID, id.UserID, domain.AuditActionKillSwitchSet,
		"mcp_killswitch", e.Scope+":"+e.TargetID, "allowed", now,
		map[string]any{"scope": e.Scope, "target_id": e.TargetID, "active": e.Active, "reason": reason})
	if err != nil {
		return domain.ErrInternal("failed to build event", err)
	}
	if _, err := uc.kills.UpsertKillSwitch(ctx, e, []domain.OutboxRecord{changed, audit}); err != nil {
		return wrapRepoErr(err, "failed to set kill switch")
	}
	uc.core.InvalidateTenant(id.TenantID)
	return nil
}

// checkTarget: a client/grant must exist in this tenant (RLS makes other
// tenants' ids look missing). Sessions live in the gateway, so any id is accepted.
func (uc *KillSwitchAdmin) checkTarget(ctx context.Context, e domain.KillSwitchEntry) error {
	switch e.Scope {
	case domain.KillScopeGrant:
		ok, err := uc.kills.GrantExists(ctx, e.TenantID, e.TargetID)
		if err != nil {
			return domain.ErrInternal("failed to look up grant", err)
		}
		if !ok {
			return domain.ErrNotFound()
		}
	case domain.KillScopeClient:
		if uc.clients == nil {
			return nil
		}
		if _, err := uc.clients.ClientStatus(ctx, e.TenantID, e.TargetID); err != nil {
			return domain.ErrNotFound()
		}
	}
	return nil
}

func (uc *KillSwitchAdmin) List(ctx context.Context) ([]domain.KillSwitchEntry, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return nil, err
	}
	if err := id.requireAdmin(); err != nil {
		return nil, err
	}
	out, err := uc.kills.ListKillSwitches(ctx, id.TenantID, true)
	if err != nil {
		return nil, domain.ErrInternal("failed to list kill switches", err)
	}
	return out, nil
}

// GetKillState tells the gateway whether a request is stopped (cached ≤ KillStateTTL).
type GetKillState struct{ core *GovernanceCore }

func NewGetKillState(core *GovernanceCore) *GetKillState { return &GetKillState{core: core} }

func (uc *GetKillState) Execute(ctx context.Context, clientID, grantID, sessionID string) (domain.KillSwitchEntry, bool, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.KillSwitchEntry{}, false, domain.ErrNoTenant(err)
	}
	st, err := uc.core.KillState(ctx, tenantID)
	if err != nil {
		return domain.KillSwitchEntry{}, false, domain.ErrInternal("failed to read kill state", err)
	}
	e, blocked := st.Blocked(clientID, grantID, sessionID)
	return e, blocked, nil
}

// KillSwitchCleanup performs the idempotent side effects of an activated
// switch (retried until all succeed): cancel approvals, stop running calls,
// revoke OAuth refresh tokens. PATs are not revoked: the deny state already
// blocks them and no auth RPC suspends a PAT.
type KillSwitchCleanup struct {
	kills     KillSwitchRepository
	approvals ApprovalRepository
	calls     ToolCallRepository
	revoker   RefreshTokenRevoker
	canceller ToolCanceller
	clock     Clock
	log       *slog.Logger
}

func NewKillSwitchCleanup(kills KillSwitchRepository, approvals ApprovalRepository, calls ToolCallRepository, revoker RefreshTokenRevoker, canceller ToolCanceller, clock Clock, log *slog.Logger) *KillSwitchCleanup {
	if canceller == nil {
		canceller = noopCanceller{}
	}
	if log == nil {
		log = slog.Default()
	}
	return &KillSwitchCleanup{kills: kills, approvals: approvals, calls: calls, revoker: revoker, canceller: canceller, clock: clock, log: log}
}

func (uc *KillSwitchCleanup) Execute(ctx context.Context, batch int) (int, error) {
	pending, err := uc.kills.PendingKillCleanups(ctx, batch)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, e := range pending {
		if err := uc.cleanOne(ctx, e); err != nil {
			uc.log.WarnContext(ctx, "kill switch cleanup will retry", slog.String("scope", e.Scope), slog.Any("error", err))
			continue
		}
		done++
	}
	return done, nil
}

func (uc *KillSwitchCleanup) cleanOne(ctx context.Context, e domain.KillSwitchEntry) error {
	ctx = tenant.WithTenantID(ctx, e.TenantID)
	if e.SetBy != "" {
		ctx = tenant.WithUserID(ctx, e.SetBy)
	}
	now := uc.clock.Now()
	if e.Active {
		if _, err := uc.approvals.CancelApprovals(ctx, e.TenantID, e.Scope, e.TargetID, now); err != nil {
			return err
		}
		ids, err := uc.calls.InterruptCallsInScope(ctx, e.TenantID, e.Scope, e.TargetID, domain.ReasonKilled, now)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := uc.canceller.Cancel(ctx, id); err != nil {
				uc.log.WarnContext(ctx, "cancel running tool failed", slog.String("call_id", id), slog.Any("error", err))
			}
		}
		if uc.revoker != nil {
			grants, err := uc.kills.GrantIDsInScope(ctx, e.TenantID, e.Scope, e.TargetID)
			if err != nil {
				return err
			}
			for _, g := range grants {
				if err := uc.revoker.RevokeGrantTokens(ctx, g, "kill_switch"); err != nil {
					return err
				}
			}
		}
	}
	return uc.kills.ClearKillCleanup(ctx, e.TenantID, e.ID)
}
