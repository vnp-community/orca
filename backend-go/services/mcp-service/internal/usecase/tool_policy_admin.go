package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// PolicyAdmin implements policy list/upsert/delete for tenant admins.
type PolicyAdmin struct {
	repo   PolicyRepository
	engine PolicyEngine
	core   *GovernanceCore
	clock  Clock
}

func NewPolicyAdmin(repo PolicyRepository, engine PolicyEngine, core *GovernanceCore, clock Clock) *PolicyAdmin {
	return &PolicyAdmin{repo: repo, engine: engine, core: core, clock: clock}
}

func (uc *PolicyAdmin) List(ctx context.Context) ([]domain.ToolPolicy, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return nil, err
	}
	if err := id.requireAdmin(); err != nil {
		return nil, err
	}
	out, err := uc.repo.ListToolPolicies(ctx, id.TenantID)
	if err != nil {
		return nil, domain.ErrInternal("failed to list policies", err)
	}
	return out, nil
}

// UpsertInput: an empty ID creates; otherwise Policy.Version must equal the stored version.
type UpsertInput struct {
	Policy       domain.ToolPolicy
	TouchedTools []domain.ToolRef
}

func toolNameOf(channel string) string { return strings.ReplaceAll(channel, ".", "_") }
func namespaceOf(channel string) string {
	if i := strings.IndexByte(channel, '.'); i >= 0 {
		return channel[:i]
	}
	return channel
}

// hardDeniedTouched lists the hard-denied tools a (non-deny) policy match would reach.
func (uc *PolicyAdmin) hardDeniedTouched(ctx context.Context, m domain.ToolPolicyMatch, touched []domain.ToolRef) ([]string, error) {
	channels, prefixes, err := uc.engine.HardDenySets(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var hits []string
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			hits = append(hits, name)
		}
	}
	for _, c := range channels {
		if (m.Tool != "" && toolNameOf(c) == m.Tool) || (m.Namespace != "" && namespaceOf(c) == m.Namespace) {
			add(toolNameOf(c))
		}
	}
	for _, p := range prefixes {
		pn := strings.TrimSuffix(p, ".")
		if (m.Tool != "" && strings.HasPrefix(m.Tool, pn+"_")) || (m.Namespace != "" && m.Namespace == pn) {
			add(pn + "_*")
		}
	}
	for _, t := range touched {
		denied, err := uc.engine.IsHardDenied(ctx, t.Channel)
		if err != nil {
			return nil, err
		}
		if denied {
			add(t.Name)
		}
	}
	// With no tool/namespace/risk dimension the match is the whole catalog,
	// which always contains hard-denied tools.
	if m.Tool == "" && m.Namespace == "" && m.Risk == "" && len(hits) == 0 {
		add("(entire catalog)")
	}
	return hits, nil
}

func (uc *PolicyAdmin) Upsert(ctx context.Context, in UpsertInput) (domain.ToolPolicy, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return domain.ToolPolicy{}, err
	}
	if err := id.requireAdmin(); err != nil {
		return domain.ToolPolicy{}, err
	}
	p := in.Policy
	if err := p.Validate(); err != nil {
		return domain.ToolPolicy{}, err
	}
	if p.Decision != domain.DecisionDeny {
		hits, err := uc.hardDeniedTouched(ctx, p.Match, in.TouchedTools)
		if err != nil {
			return domain.ToolPolicy{}, domain.ErrInternal("policy engine unavailable", err)
		}
		if len(hits) > 0 {
			return domain.ToolPolicy{}, domain.ErrPolicyHardDeny(hits)
		}
	}
	if p.Decision == domain.DecisionAllow && p.Match.Tool == "" {
		wide := p.Match.Risk == string(domain.RiskExec) || p.Match.Risk == string(domain.RiskDestructive)
		for _, t := range in.TouchedTools {
			if t.Risk == string(domain.RiskExec) || t.Risk == string(domain.RiskDestructive) {
				wide = true
			}
		}
		if wide {
			return domain.ToolPolicy{}, domain.ErrInvalidArgument("allowing exec or destructive tools requires a policy that names the exact tool")
		}
	}
	now := uc.clock.Now()
	p.UpdatedBy, p.UpdatedAt = id.UserID, now
	var out domain.ToolPolicy
	if p.ID == "" {
		p.ID, p.Version, p.CreatedBy = uuid.NewString(), 1, id.UserID
		evs, err := uc.policyEvents(id, p, "create", domain.AuditActionPolicyUpsert, now)
		if err != nil {
			return domain.ToolPolicy{}, domain.ErrInternal("failed to build events", err)
		}
		out, err = uc.repo.CreateToolPolicy(ctx, id.TenantID, p, evs)
		if err != nil {
			return domain.ToolPolicy{}, wrapRepoErr(err, "failed to create policy")
		}
	} else {
		evs, err := uc.policyEvents(id, domain.ToolPolicy{ID: p.ID, Version: p.Version + 1, Match: p.Match, Decision: p.Decision}, "update", domain.AuditActionPolicyUpsert, now)
		if err != nil {
			return domain.ToolPolicy{}, domain.ErrInternal("failed to build events", err)
		}
		out, err = uc.repo.UpdateToolPolicy(ctx, id.TenantID, p, evs)
		if err != nil {
			return domain.ToolPolicy{}, wrapRepoErr(err, "failed to update policy")
		}
	}
	uc.core.InvalidateTenant(id.TenantID)
	return out, nil
}

func (uc *PolicyAdmin) Delete(ctx context.Context, policyID string) error {
	id, err := userCaller(ctx)
	if err != nil {
		return err
	}
	if err := id.requireAdmin(); err != nil {
		return err
	}
	if _, err := uuid.Parse(policyID); err != nil {
		return domain.ErrNotFound()
	}
	evs, err := uc.policyEvents(id, domain.ToolPolicy{ID: policyID}, "delete", domain.AuditActionPolicyDelete, uc.clock.Now())
	if err != nil {
		return domain.ErrInternal("failed to build events", err)
	}
	if err := uc.repo.DeleteToolPolicy(ctx, id.TenantID, policyID, id.UserID, evs); err != nil {
		return wrapRepoErr(err, "failed to delete policy")
	}
	uc.core.InvalidateTenant(id.TenantID)
	return nil
}

// policyEvents returns the cache-invalidation event and the admin audit event.
func (uc *PolicyAdmin) policyEvents(id callerIdentity, p domain.ToolPolicy, op, action string, now time.Time) ([]domain.OutboxRecord, error) {
	changed, err := domain.NewOutboxEvent(uuid.NewString(), domain.SubjectPolicyChanged, id.TenantID, now,
		map[string]any{"policy_id": p.ID, "op": op})
	if err != nil {
		return nil, err
	}
	meta := map[string]any{"op": op, "decision": p.Decision, "version": p.Version}
	if len(p.Match.AsMap()) > 0 {
		meta["match"] = p.Match.AsMap()
	}
	audit, err := domain.NewAdminAuditEvent(uuid.NewString(), uuid.NewString(), id.TenantID, id.UserID, action, "mcp_policy", p.ID, "allowed", now, meta)
	if err != nil {
		return nil, err
	}
	return []domain.OutboxRecord{changed, audit}, nil
}
