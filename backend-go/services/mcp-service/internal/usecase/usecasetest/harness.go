package usecasetest

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/mcp-service/internal/adapter/policyengine"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

// FakeClients serves configured OAuth client standings.
type FakeClients struct {
	mu       sync.Mutex
	Statuses map[string]string
}

func (f *FakeClients) ClientStatus(_ context.Context, _ string, clientID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.Statuses[clientID]; ok {
		return s, nil
	}
	return "", errors.New("unknown client")
}

func (f *FakeClients) Set(clientID, status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Statuses[clientID] = status
}

// RecordingRevoker records grants whose refresh tokens were revoked.
type RecordingRevoker struct {
	mu      sync.Mutex
	Revoked []string
}

func (r *RecordingRevoker) RevokeGrantTokens(_ context.Context, grantID, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Revoked = append(r.Revoked, grantID)
	return nil
}

// RecordingCanceller records cancelled call ids.
type RecordingCanceller struct {
	mu        sync.Mutex
	Cancelled []string
}

func (c *RecordingCanceller) Cancel(_ context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Cancelled = append(c.Cancelled, id)
	return nil
}

// Harness wires every governance usecase over a MemStore and the REAL Rego bundle.
type Harness struct {
	Clock     *Clock
	Store     *MemStore
	Engine    usecase.PolicyEngine
	Core      *usecase.GovernanceCore
	Clients   *FakeClients
	Revoker   *RecordingRevoker
	Canceller *RecordingCanceller
	Cfg       usecase.GovernanceConfig

	Evaluate  *usecase.EvaluateToolCall
	Filter    *usecase.FilterTools
	Explain   *usecase.ExplainPolicy
	Policies  *usecase.PolicyAdmin
	Settings  *usecase.SettingsAdmin
	Authorize *usecase.AuthorizeToolCall
	Complete  *usecase.CompleteToolCall
	Wait      *usecase.WaitApproval
	List      *usecase.ListApprovals
	Decide    *usecase.DecideApproval
	KillAdmin *usecase.KillSwitchAdmin
	KillState *usecase.GetKillState
	Expire    *usecase.ExpireApprovals
	Cleanup   *usecase.KillSwitchCleanup
	Maint     *usecase.ToolCallMaintenance
}

// BundlePath locates policy/orca-authz relative to this source file.
func BundlePath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "policy", "orca-authz")
}

// NewHarness uses the real bundle. tenantDefaultEnabled mirrors MCP_TENANT_DEFAULT_ENABLED.
func NewHarness(tenantDefaultEnabled bool) *Harness {
	clock := NewClock()
	engine := policyengine.New(BundlePath(), slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	return NewHarnessWithEngine(tenantDefaultEnabled, clock, engine)
}

func NewHarnessWithEngine(tenantDefaultEnabled bool, clock *Clock, engine usecase.PolicyEngine) *Harness {
	h := &Harness{Clock: clock, Store: NewMemStore(clock), Engine: engine, Clients: &FakeClients{Statuses: map[string]string{"c1": "allowed"}},
		Revoker: &RecordingRevoker{}, Canceller: &RecordingCanceller{}, Cfg: usecase.DefaultGovernanceConfig()}
	defaults := usecase.Defaults{TenantEnabled: tenantDefaultEnabled, MaxTokenDays: 90}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	h.Cfg.PolicyCacheTTL = 0 // tests change state between calls; no cache
	h.Cfg.KillStateTTL = 0
	h.Core = usecase.NewGovernanceCore(h.Store, h.Store, h.Store, engine, h.Clients, clock, defaults, h.Cfg, log)
	red := domain.SecretRedactor{}
	h.Evaluate, h.Filter, h.Explain = usecase.NewEvaluateToolCall(h.Core), usecase.NewFilterTools(h.Core), usecase.NewExplainPolicy(h.Core)
	h.Policies = usecase.NewPolicyAdmin(h.Store, engine, h.Core, clock)
	h.Settings = usecase.NewSettingsAdmin(h.Store, h.Store, h.Core, defaults, clock)
	h.Authorize = usecase.NewAuthorizeToolCall(h.Core, h.Store, h.Store, red, clock)
	h.Complete = usecase.NewCompleteToolCall(h.Core, h.Store, clock)
	h.Wait = usecase.NewWaitApproval(h.Store, clock, 5_000_000)
	h.List = usecase.NewListApprovals(h.Store, clock)
	h.Decide = usecase.NewDecideApproval(h.Core, h.Store, red, clock)
	h.KillAdmin = usecase.NewKillSwitchAdmin(h.Store, h.Clients, h.Core, red, clock)
	h.KillState = usecase.NewGetKillState(h.Core)
	h.Expire = usecase.NewExpireApprovals(h.Store, clock)
	h.Cleanup = usecase.NewKillSwitchCleanup(h.Store, h.Store, h.Store, h.Revoker, h.Canceller, clock, log)
	h.Maint = usecase.NewToolCallMaintenance(h.Store, h.Canceller, h.Cfg, 0, clock)
	return h
}

// Ctx builds a request context the way the gRPC interceptor does.
func Ctx(tenantID, userID, role string) context.Context {
	ctx := tenant.WithTenantID(context.Background(), tenantID)
	ctx = tenant.WithUserID(ctx, userID)
	if role != "" {
		ctx = tenant.WithRole(ctx, role)
	}
	return ctx
}

// Tools used across tests; fields mirror what the gateway catalog supplies.
var (
	ToolRead        = domain.ToolRef{Name: "task_list", Channel: "task.list", Namespace: "task", Risk: "read", RequiredScope: "orca:read", Title: "List tasks"}
	ToolWrite       = domain.ToolRef{Name: "task_create", Channel: "task.create", Namespace: "task", Risk: "write_reversible", RequiredScope: "orca:write", Title: "Create task"}
	ToolExec        = domain.ToolRef{Name: "terminal_send", Channel: "terminal.send", Namespace: "terminal", Risk: "exec", RequiredScope: "orca:exec", Title: "Send terminal input"}
	ToolDestructive = domain.ToolRef{Name: "worktree_remove", Channel: "worktree.remove", Namespace: "worktree", Risk: "destructive", RequiredScope: "orca:exec", Title: "Remove worktree"}
	ToolAdmin       = domain.ToolRef{Name: "tenant_update", Channel: "tenant.update", Namespace: "tenant", Risk: "admin", RequiredScope: "orca:admin", Title: "Update tenant"}
	ToolHardDenied  = domain.ToolRef{Name: "credentials_get", Channel: "credentials.get", Namespace: "credentials", Risk: "read", RequiredScope: "orca:read", Title: "Get credential"}
)

// AllScopes is a token with every scope.
var AllScopes = []string{"orca:read", "orca:write", "orca:exec", "orca:admin"}

// CC returns a call context for client c1 with all scopes.
func CC() domain.CallContext {
	return domain.CallContext{ClientID: "c1", ClientName: "Test Agent", Scopes: AllScopes, MCPSessionID: "sess-1"}
}
