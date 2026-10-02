package usecase

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// GovernanceCore holds the collaborators and short-lived caches shared by the
// governance usecases. Only INPUTS are cached (settings, policies, kill
// state, client standing); decisions are never cached because a cached
// "allow" is exactly what a tightened policy or kill switch must not serve.
type GovernanceCore struct {
	policies PolicyRepository
	calls    ToolCallRepository
	kills    KillSwitchRepository
	engine   PolicyEngine
	clients  ClientStatusReader
	clock    Clock
	defaults Defaults
	cfg      GovernanceConfig
	log      *slog.Logger

	mu      sync.Mutex
	snaps   map[string]snapEntry
	kstates map[string]killEntry
	cstatus map[string]clientEntry
}

type snapEntry struct {
	snap PolicySnapshot
	at   time.Time
}
type killEntry struct {
	state domain.KillState
	at    time.Time
}
type clientEntry struct {
	status string
	at     time.Time
}

func NewGovernanceCore(policies PolicyRepository, calls ToolCallRepository, kills KillSwitchRepository, engine PolicyEngine,
	clients ClientStatusReader, clock Clock, defaults Defaults, cfg GovernanceConfig, log *slog.Logger) *GovernanceCore {
	if log == nil {
		log = slog.Default()
	}
	return &GovernanceCore{
		policies: policies, calls: calls, kills: kills, engine: engine, clients: clients, clock: clock,
		defaults: defaults, cfg: cfg, log: log,
		snaps: map[string]snapEntry{}, kstates: map[string]killEntry{}, cstatus: map[string]clientEntry{},
	}
}

// InvalidateTenant drops cached inputs after a local change or a
// policy/kill-switch event from another replica.
func (g *GovernanceCore) InvalidateTenant(tenantID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.snaps, tenantID)
	delete(g.kstates, tenantID)
	for k := range g.cstatus {
		if len(k) > len(tenantID) && k[:len(tenantID)] == tenantID {
			delete(g.cstatus, k)
		}
	}
}

func (g *GovernanceCore) snapshot(ctx context.Context, tenantID string) (PolicySnapshot, error) {
	g.mu.Lock()
	if e, ok := g.snaps[tenantID]; ok && g.clock.Now().Sub(e.at) < g.cfg.PolicyCacheTTL {
		g.mu.Unlock()
		return e.snap, nil
	}
	g.mu.Unlock()
	snap, err := g.policies.LoadPolicySnapshot(ctx, g.defaults.For(tenantID))
	if err != nil {
		return PolicySnapshot{}, err
	}
	g.mu.Lock()
	g.snaps[tenantID] = snapEntry{snap: snap, at: g.clock.Now()}
	g.mu.Unlock()
	return snap, nil
}

// KillState returns the tenant's active switches (cache TTL bounds propagation
// when the event bus is down; events invalidate it sooner).
func (g *GovernanceCore) KillState(ctx context.Context, tenantID string) (domain.KillState, error) {
	g.mu.Lock()
	if e, ok := g.kstates[tenantID]; ok && g.clock.Now().Sub(e.at) < g.cfg.KillStateTTL {
		g.mu.Unlock()
		return e.state, nil
	}
	g.mu.Unlock()
	entries, err := g.kills.ListKillSwitches(ctx, tenantID, true)
	if err != nil {
		return domain.KillState{}, err
	}
	st := domain.KillState{Entries: entries}
	g.mu.Lock()
	g.kstates[tenantID] = killEntry{state: st, at: g.clock.Now()}
	g.mu.Unlock()
	return st, nil
}

func (g *GovernanceCore) clientStatus(ctx context.Context, tenantID string, cc domain.CallContext) string {
	if cc.TokenKind == "mcp_pat" {
		return "allowed" // PATs are not OAuth clients
	}
	if g.clients == nil || cc.ClientID == "" {
		return "pending"
	}
	key := tenantID + "|" + cc.ClientID
	g.mu.Lock()
	if e, ok := g.cstatus[key]; ok && g.clock.Now().Sub(e.at) < g.cfg.PolicyCacheTTL {
		g.mu.Unlock()
		return e.status
	}
	g.mu.Unlock()
	st, err := g.clients.ClientStatus(ctx, tenantID, cc.ClientID)
	if err != nil || st == "" {
		return "pending" // unknown standing is not allowed
	}
	g.mu.Lock()
	g.cstatus[key] = clientEntry{status: st, at: g.clock.Now()}
	g.mu.Unlock()
	return st
}

// evalBatch evaluates tools against ONE snapshot. Any infrastructure error
// yields deny decisions (reason policy_unavailable) rather than an error, so a
// caller can never mistake a failure for an allow.
func (g *GovernanceCore) evalBatch(ctx context.Context, id callerIdentity, tools []domain.ToolRef, cc domain.CallContext, forExplain bool) ([]domain.PolicyDecision, PolicySnapshot, domain.KillSwitchEntry, bool) {
	denyAll := func() ([]domain.PolicyDecision, PolicySnapshot, domain.KillSwitchEntry, bool) {
		out := make([]domain.PolicyDecision, len(tools))
		for i := range out {
			out[i] = domain.DenyUnavailable()
		}
		return out, PolicySnapshot{}, domain.KillSwitchEntry{}, false
	}
	snap, err := g.snapshot(ctx, id.TenantID)
	if err != nil {
		g.log.ErrorContext(ctx, "policy snapshot unavailable", slog.Any("error", err))
		return denyAll()
	}
	ks, err := g.KillState(ctx, id.TenantID)
	if err != nil {
		g.log.ErrorContext(ctx, "kill state unavailable", slog.Any("error", err))
		return denyAll()
	}
	entry, killed := ks.Blocked(cc.ClientID, cc.GrantID, cc.MCPSessionID)
	tainted := false
	if !forExplain {
		tainted, err = g.calls.IsTainted(ctx, id.TenantID, id.UserID, cc.ClientID, g.clock.Now())
		if err != nil {
			g.log.ErrorContext(ctx, "taint lookup failed", slog.Any("error", err))
			return denyAll()
		}
	}
	clientStatus := g.clientStatus(ctx, id.TenantID, cc)
	if forExplain && cc.ClientID == "" {
		clientStatus = "allowed"
	}
	out := make([]domain.PolicyDecision, len(tools))
	for i, t := range tools {
		in := domain.PolicyInput{
			UserID: id.UserID, UserRole: id.Role, ClientID: cc.ClientID, ClientStatus: clientStatus,
			Scopes: cc.Scopes, Enabled: snap.Settings.Enabled, MaxDepth: g.cfg.MaxDepth, KillActive: killed,
			Depth: cc.Depth, UntrustedRead: tainted, Policies: snap.Policies, Tool: t,
		}
		d := g.engine.Evaluate(ctx, in)
		d.Epoch = snap.Epoch
		out[i] = d
	}
	return out, snap, entry, killed
}
