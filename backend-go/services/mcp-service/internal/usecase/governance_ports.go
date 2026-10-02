package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// PolicyEngine decides a tool call. Evaluate never returns an error: any
// engine failure is reported as a deny (fail closed).
type PolicyEngine interface {
	Evaluate(ctx context.Context, in domain.PolicyInput) domain.PolicyDecision
	// HardDenySets exposes the static hard-deny channels and channel prefixes
	// so upserts can refuse policies that would loosen them.
	HardDenySets(ctx context.Context) (channels, prefixes []string, err error)
	IsHardDenied(ctx context.Context, channel string) (bool, error)
}

// PolicySnapshot is everything the decision needs from the database.
type PolicySnapshot struct {
	Settings domain.TenantSettings
	Policies []domain.ToolPolicy
	Epoch    int64
}

// SettingsPatch: nil fields keep the stored value.
type SettingsPatch struct {
	Enabled, DCREnabled              *bool
	MaxTokenDays, ApprovalTTLSeconds *int
}

// PolicyRepository persists tenant settings and tool policies. Every change
// bumps policy_epoch and enqueues its events in the same transaction.
type PolicyRepository interface {
	// LoadPolicySnapshot lazily creates the tenant's settings row from defaults.
	LoadPolicySnapshot(ctx context.Context, defaults domain.TenantSettings) (PolicySnapshot, error)
	ListToolPolicies(ctx context.Context, tenantID string) ([]domain.ToolPolicy, error)
	CreateToolPolicy(ctx context.Context, tenantID string, p domain.ToolPolicy, events []domain.OutboxRecord) (domain.ToolPolicy, error)
	// UpdateToolPolicy applies only when the stored version equals p.Version;
	// the stored version then becomes p.Version+1.
	UpdateToolPolicy(ctx context.Context, tenantID string, p domain.ToolPolicy, events []domain.OutboxRecord) (domain.ToolPolicy, error)
	DeleteToolPolicy(ctx context.Context, tenantID, id, actor string, events []domain.OutboxRecord) error
	PatchTenantSettings(ctx context.Context, defaults domain.TenantSettings, patch SettingsPatch, actor string, events []domain.OutboxRecord) (domain.TenantSettings, error)
}

// ApprovalLimits are the anti-spam caps for creating approvals.
type ApprovalLimits struct {
	MaxPendingPerClient int
	MaxCreatedPerHour   int
}

// ApprovalRepository persists approvals. State changes enqueue their own
// events (resolved, audit) in the same transaction.
type ApprovalRepository interface {
	// FindOrCreatePendingApproval returns the open approval for the same
	// (tenant,user,client,params_hash) or creates one. ErrApprovalFlood when capped.
	FindOrCreatePendingApproval(ctx context.Context, a domain.Approval, limits ApprovalLimits, now time.Time) (domain.Approval, bool, error)
	// DecideApproval is the single atomic statement from the spec; failures are
	// diagnosed with domain.DecideDiagnosis.
	DecideApproval(ctx context.Context, in DecideApprovalRepoInput) (domain.Approval, error)
	GetApproval(ctx context.Context, tenantID, id string) (domain.Approval, error)
	ListApprovals(ctx context.Context, q ApprovalQuery) ([]domain.Approval, error)
	ListExpiredApprovalRefs(ctx context.Context, now time.Time, limit int) ([]Ref, error)
	ExpireApproval(ctx context.Context, tenantID, id string, now time.Time) (bool, error)
	CancelApprovals(ctx context.Context, tenantID, scope, target string, now time.Time) (int, error)
}

var ErrApprovalFlood = errorString("usecase: approval flood")

type errorString string

func (e errorString) Error() string { return string(e) }

type Ref struct{ TenantID, ID string }

type DecideApprovalRepoInput struct {
	TenantID, UserID, ApprovalID string
	Approve                      bool
	ParamsHash, Note, Via        string
	Now                          time.Time
}

type ApprovalQuery struct {
	TenantID, UserID string
	PendingOnly      bool
	CursorAt         time.Time // zero = first page
	CursorID         string
	Limit            int
	Now              time.Time
}

// RateLimits: sliding-window budgets per (tenant,user,client).
type RateLimits struct {
	PerMinute      map[string]int // by risk class: read, write, exec
	TotalPerMinute int
	DailyPerUser   int
	LoopSlowDown   int // identical calls within 60s
	LoopBlock      int // identical calls within 5m
}

type AdmitRequest struct {
	Call domain.ToolCall // decision allow|approved; ID pre-generated
	// ConsumeApprovalHash, when set, requires (and consumes) an approved,
	// unconsumed approval with this hash for the same user and client.
	ConsumeApprovalHash string
	Limits              RateLimits
	Now                 time.Time
}

type AdmitResult struct {
	Admitted        bool
	DenyReason      string
	ApprovalMissing bool // ConsumeApprovalHash set but none was available
	ApprovalID      string
	ApproverID      string
}

// ToolCallRepository is the call journal, limiter and taint store.
type ToolCallRepository interface {
	AdmitToolCall(ctx context.Context, req AdmitRequest) (AdmitResult, error)
	// RecordFinalCall writes an already-final row (deny/denied/expired) and its audit event.
	RecordFinalCall(ctx context.Context, c domain.ToolCall, now time.Time) error
	FinalizeToolCall(ctx context.Context, tenantID, userID, callID, result, reason string, durationMs int64, now time.Time, taintTTL time.Duration) (domain.ToolCall, error)
	IsTainted(ctx context.Context, tenantID, userID, clientID string, now time.Time) (bool, error)
	ListStaleCalls(ctx context.Context, startedBefore time.Time, limit int) ([]Ref, error)
	InterruptCall(ctx context.Context, tenantID, callID, reason string, now time.Time) (bool, error)
	// InterruptCallsInScope finalizes started calls of a kill-switch scope and returns their ids.
	InterruptCallsInScope(ctx context.Context, tenantID, scope, target, reason string, now time.Time) ([]string, error)
	PurgeFinishedCalls(ctx context.Context, before time.Time, limit int) (int, error)
}

// KillSwitchRepository persists kill switches.
type KillSwitchRepository interface {
	UpsertKillSwitch(ctx context.Context, e domain.KillSwitchEntry, events []domain.OutboxRecord) (domain.KillSwitchEntry, error)
	ListKillSwitches(ctx context.Context, tenantID string, activeOnly bool) ([]domain.KillSwitchEntry, error)
	PendingKillCleanups(ctx context.Context, limit int) ([]domain.KillSwitchEntry, error)
	ClearKillCleanup(ctx context.Context, tenantID, id string) error
	GrantIDsInScope(ctx context.Context, tenantID, scope, target string) ([]string, error)
	GrantExists(ctx context.Context, tenantID, grantID string) (bool, error)
}

// ClientStatusReader returns an OAuth client's standing in the tenant
// (allowed|blocked|pending). An error is treated as "not allowed".
type ClientStatusReader interface {
	ClientStatus(ctx context.Context, tenantID, clientID string) (string, error)
}

// RefreshTokenRevoker revokes the OAuth refresh tokens of a grant via auth-service.
type RefreshTokenRevoker interface {
	RevokeGrantTokens(ctx context.Context, grantID, reason string) error
}

// ToolCanceller stops a running tool. The default is a no-op because running
// tools live in the gateway, which also stops them itself when its kill-state
// poll flips.
type ToolCanceller interface {
	Cancel(ctx context.Context, callID string) error
}

type noopCanceller struct{}

func (noopCanceller) Cancel(context.Context, string) error { return nil }

// GovernanceConfig carries tunables (env names in config/governance.go).
type GovernanceConfig struct {
	MaxDepth            int
	PolicyCacheTTL      time.Duration
	KillStateTTL        time.Duration
	TaintTTL            time.Duration
	ToolCallMaxAge      time.Duration
	MaxPendingPerClient int
	MaxApprovalsPerHour int
	Rate                RateLimits
}

func DefaultGovernanceConfig() GovernanceConfig {
	return GovernanceConfig{
		MaxDepth: 1, PolicyCacheTTL: 5 * time.Second, KillStateTTL: 5 * time.Second,
		TaintTTL: 30 * time.Minute, ToolCallMaxAge: 15 * time.Minute,
		MaxPendingPerClient: 10, MaxApprovalsPerHour: 30,
		Rate: RateLimits{
			PerMinute:      map[string]int{"exec": 30, "write": 120, "read": 300},
			TotalPerMinute: 300, DailyPerUser: 5000, LoopSlowDown: 5, LoopBlock: 20,
		},
	}
}
