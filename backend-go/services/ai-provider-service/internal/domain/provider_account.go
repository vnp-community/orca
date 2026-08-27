// Package domain holds ai-provider-service's entities and value objects. Per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md, this
// package has zero imports outside stdlib + other domain/ packages — no
// database, no gRPC, no framework, and critically for this service: no
// secret material of any kind. A ProviderAccount never carries a plaintext
// key or ciphertext blob, only a CredentialRef pointer that
// credential-broker-service resolves — see
// specs/backend-go/services/ai-provider-service.md §4 and §9.
package domain

import (
	"errors"
	"time"
)

// ProviderType enumerates the AI provider account kinds this service tracks
// metadata for. Deliberately a separate axis from usage-service's Provider
// (which tracks AI-CLI tool usage, not provider-account quota) — see
// ai-provider-service.md's bounded-context table.
type ProviderType string

const (
	ProviderTypeAnthropic  ProviderType = "anthropic"
	ProviderTypeOpenAI     ProviderType = "openai"
	ProviderTypeGoogle     ProviderType = "google"
	ProviderTypeAzure      ProviderType = "azure"
	ProviderTypeAWSBedrock ProviderType = "aws_bedrock"
	ProviderTypeOllama     ProviderType = "ollama"
	ProviderTypeVLLM       ProviderType = "vllm"
)

// Valid reports whether p is one of the known ProviderType enum values.
func (p ProviderType) Valid() bool {
	switch p {
	case ProviderTypeAnthropic, ProviderTypeOpenAI, ProviderTypeGoogle,
		ProviderTypeAzure, ProviderTypeAWSBedrock, ProviderTypeOllama, ProviderTypeVLLM:
		return true
	default:
		return false
	}
}

// AccountStatus is the lifecycle state machine for a ProviderAccount, per
// ai-provider-service.md §4. "pending" until ciphertext push is confirmed —
// Resolve must never return a "pending" account, to avoid recreating TS
// Gap 2 (see §10's cutover-ordering note).
type AccountStatus string

const (
	AccountStatusPending  AccountStatus = "pending"
	AccountStatusActive   AccountStatus = "active"
	AccountStatusRotating AccountStatus = "rotating"
	AccountStatusRevoked  AccountStatus = "revoked"
	AccountStatusError    AccountStatus = "error"
)

// Valid reports whether s is one of the known AccountStatus enum values.
func (s AccountStatus) Valid() bool {
	switch s {
	case AccountStatusPending, AccountStatusActive, AccountStatusRotating, AccountStatusRevoked, AccountStatusError:
		return true
	default:
		return false
	}
}

// AccountScope is the resolution axis for the user->project->server
// cascade (ai-provider-service.md §4). The zero value is intentionally
// invalid (not ScopeServer) so a forgotten Scope field is always caught by
// NewProviderAccount rather than silently resolving as tenant-wide.
type AccountScope string

const (
	ScopeUser    AccountScope = "user"
	ScopeProject AccountScope = "project"
	ScopeServer  AccountScope = "server"
)

// Valid reports whether s is one of the known AccountScope enum values.
func (s AccountScope) Valid() bool {
	switch s {
	case ScopeUser, ScopeProject, ScopeServer:
		return true
	default:
		return false
	}
}

var (
	// ErrEmptyTenantID is returned when TenantID is empty — a provider
	// account with no owning tenant is never a valid domain state.
	ErrEmptyTenantID = errors.New("domain: tenant_id is required")
	// ErrInvalidProviderType is returned when ProviderType isn't one of the
	// known enum values.
	ErrInvalidProviderType = errors.New("domain: invalid provider type")
	// ErrInvalidStatus is returned when Status isn't one of the known enum
	// values.
	ErrInvalidStatus = errors.New("domain: invalid account status")
	// ErrInvalidScope is returned when Scope isn't one of the known enum
	// values.
	ErrInvalidScope = errors.New("domain: invalid account scope")
	// ErrInvalidScopeRef is returned when ScopeRefID presence doesn't match
	// Scope — mirrors the `scope_ref_matches_scope` CHECK constraint in
	// migrations/0001_init.up.sql: UserID set iff Scope == ScopeUser,
	// ProjectID set iff Scope == ScopeProject, both empty iff ScopeServer.
	ErrInvalidScopeRef = errors.New("domain: scope_ref_id must be set iff scope requires it")
	// ErrAccountNotFound is returned by the repository/usecase layer when no
	// account matches a lookup.
	ErrAccountNotFound = errors.New("domain: provider account not found")
)

// NoProviderReason distinguishes why ResolveProvider found no usable
// account — mirrors the TS resolver's distinction (ai-provider-service.md
// §4), useful when debugging a failed agent spawn.
type NoProviderReason string

const (
	// ReasonQuotaOrInactive means a matching account exists but isn't
	// currently usable (not active, or its daily quota is exhausted).
	ReasonQuotaOrInactive NoProviderReason = "quota_or_inactive"
	// ReasonNoScopeMatch means no account exists at any scope in the
	// cascade for this tenant/user/project.
	ReasonNoScopeMatch NoProviderReason = "no_scope_match"
)

// ErrNoProviderAvailable is returned by ResolveProvider when the
// user->project->server cascade yields no usable account.
type ErrNoProviderAvailable struct {
	Reason NoProviderReason
}

func (e *ErrNoProviderAvailable) Error() string {
	return "domain: no provider account available: " + string(e.Reason)
}

// ProviderAccount is the system-of-record entity for an AI provider account
// — metadata and a credential pointer ONLY. There is no field on this
// struct, in this package, or anywhere in this service that can hold a
// secret value: CredentialRef is an opaque ID credential-broker-service
// resolves, never a key. A database dump of this service must never yield a
// usable credential (ai-provider-service.md §1).
type ProviderAccount struct {
	ID                 string
	TenantID           string
	ProviderType       ProviderType
	Status             AccountStatus
	CredentialRef      string // credential-broker-service metadata id — NEVER a secret value
	Scope              AccountScope
	UserID             string // set iff Scope == ScopeUser
	ProjectID          string // set iff Scope == ScopeProject
	DevServerID        string // which dev server holds this account's pushed ciphertext; empty until first push (§9)
	RotationGraceUntil *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// NewProviderAccount constructs a ProviderAccount, enforcing the invariants
// a record must satisfy to be meaningful: non-empty tenant, a recognized
// provider type/status/scope, and ScopeRefID presence matching Scope. This
// is where "ai-provider-service owns this data's correctness" actually
// lives, not scattered validation in the gRPC handler.
func NewProviderAccount(
	id, tenantID string,
	providerType ProviderType,
	status AccountStatus,
	credentialRef string,
	scope AccountScope,
	userID, projectID string,
	devServerID string,
	rotationGraceUntil *time.Time,
	createdAt, updatedAt time.Time,
) (ProviderAccount, error) {
	if tenantID == "" {
		return ProviderAccount{}, ErrEmptyTenantID
	}
	if !providerType.Valid() {
		return ProviderAccount{}, ErrInvalidProviderType
	}
	if !status.Valid() {
		return ProviderAccount{}, ErrInvalidStatus
	}
	if !scope.Valid() {
		return ProviderAccount{}, ErrInvalidScope
	}
	switch scope {
	case ScopeUser:
		if userID == "" || projectID != "" {
			return ProviderAccount{}, ErrInvalidScopeRef
		}
	case ScopeProject:
		if projectID == "" || userID != "" {
			return ProviderAccount{}, ErrInvalidScopeRef
		}
	case ScopeServer:
		if userID != "" || projectID != "" {
			return ProviderAccount{}, ErrInvalidScopeRef
		}
	}
	return ProviderAccount{
		ID:                 id,
		TenantID:           tenantID,
		ProviderType:       providerType,
		Status:             status,
		CredentialRef:      credentialRef,
		Scope:              scope,
		UserID:             userID,
		ProjectID:          projectID,
		DevServerID:        devServerID,
		RotationGraceUntil: rotationGraceUntil,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}, nil
}

// Resolvable reports whether this account may be returned by ResolveProvider
// — only "active" accounts qualify; "pending" (ciphertext push not yet
// confirmed) must never be handed to a caller, per §10's cutover-ordering
// note, and "rotating"/"revoked"/"error" are self-explanatory exclusions.
func (a ProviderAccount) Resolvable() bool {
	return a.Status == AccountStatusActive
}

// QuotaState is the daily rollup view for one account's usage — aggregate
// quota/spend bookkeeping, NOT raw usage events (ai-provider-service.md §5).
type QuotaState struct {
	AccountID    string
	Date         time.Time // truncated to UTC midnight
	CostUSD      float64
	RequestCount int64
}

// DayKey truncates a timestamp to the UTC calendar day usage.usage buckets by.
func DayKey(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}
