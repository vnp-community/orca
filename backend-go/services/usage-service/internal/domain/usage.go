// Package domain holds usage-service's entities and value objects. Per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md, this
// package has zero imports outside stdlib + other domain/ packages — no
// database, no gRPC, no framework.
package domain

import (
	"errors"
	"time"
)

// Provider distinguishes which AI-CLI tool a usage session belongs to.
// Deliberately a separate axis from ai-provider-service's ProviderType
// (Anthropic/OpenAI/... API accounts) — see usage-service.md's bounded
// context section: this tracks CLI tool usage, not provider-account quota.
type Provider string

const (
	ProviderClaude   Provider = "claude"
	ProviderCodex    Provider = "codex"
	ProviderOpenCode Provider = "opencode"
	providerUnknown  Provider = ""
)

func (p Provider) Valid() bool {
	switch p {
	case ProviderClaude, ProviderCodex, ProviderOpenCode:
		return true
	default:
		return false
	}
}

var (
	// ErrInvalidProvider is returned by NewUsageSession when Provider isn't
	// one of the known enum values.
	ErrInvalidProvider = errors.New("domain: invalid provider")
	// ErrEmptyTenant is returned when TenantID/UserID are empty — a usage
	// session with no owning tenant/user is never a valid domain state.
	ErrEmptyTenant = errors.New("domain: tenant_id and user_id are required")
	// ErrNegativeTokens guards against a caller passing negative token
	// counts, which would silently corrupt daily rollups.
	ErrNegativeTokens = errors.New("domain: token counts must be non-negative")
	// ErrEndBeforeStart guards session time-range consistency.
	ErrEndBeforeStart = errors.New("domain: ended_at must not be before started_at")
)

// UsageSession is one recorded AI-CLI invocation's token/cost accounting.
type UsageSession struct {
	ID               string
	TenantID         string
	UserID           string
	Provider         Provider
	WorktreeID       string
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	CostUSD          float64
	StartedAt        time.Time
	EndedAt          time.Time
	RequestID        string // idempotency key, see standards/api-design-guidelines.md
}

// NewUsageSession constructs a UsageSession, enforcing the invariants a
// record must satisfy to be meaningful — this is where "usage-service owns
// this data's correctness" actually lives, not scattered validation in the
// gRPC handler.
func NewUsageSession(
	id, tenantID, userID string,
	provider Provider,
	worktreeID string,
	inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int64,
	costUSD float64,
	startedAt, endedAt time.Time,
	requestID string,
) (UsageSession, error) {
	if tenantID == "" || userID == "" {
		return UsageSession{}, ErrEmptyTenant
	}
	if !provider.Valid() {
		return UsageSession{}, ErrInvalidProvider
	}
	if inputTokens < 0 || outputTokens < 0 || cacheReadTokens < 0 || cacheWriteTokens < 0 {
		return UsageSession{}, ErrNegativeTokens
	}
	if !endedAt.IsZero() && endedAt.Before(startedAt) {
		return UsageSession{}, ErrEndBeforeStart
	}
	return UsageSession{
		ID:               id,
		TenantID:         tenantID,
		UserID:           userID,
		Provider:         provider,
		WorktreeID:       worktreeID,
		InputTokens:      inputTokens,
		OutputTokens:     outputTokens,
		CacheReadTokens:  cacheReadTokens,
		CacheWriteTokens: cacheWriteTokens,
		CostUSD:          costUSD,
		StartedAt:        startedAt,
		EndedAt:          endedAt,
		RequestID:        requestID,
	}, nil
}

// DailyUsageRollup is the aggregate view over a tenant/user/provider/day —
// computed on-write (see usage-service.md §6) rather than by a separate
// batch job, with a periodic reconciliation pass as a drift safety net
// (ReconcileDailyRollups usecase, not part of this pure domain package).
type DailyUsageRollup struct {
	TenantID          string
	UserID            string
	Provider          Provider
	Day               time.Time // truncated to UTC midnight
	TotalInputTokens  int64
	TotalOutputTokens int64
	TotalCostUSD      float64
	SessionCount      int64
}

// ApplySession folds one session's numbers into the rollup — a pure,
// side-effect-free accumulation, unit-testable without touching Postgres.
func (r DailyUsageRollup) ApplySession(s UsageSession) DailyUsageRollup {
	r.TotalInputTokens += s.InputTokens
	r.TotalOutputTokens += s.OutputTokens
	r.TotalCostUSD += s.CostUSD
	r.SessionCount++
	return r
}

// DayKey truncates a timestamp to the UTC calendar day the rollup buckets by.
func DayKey(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}
