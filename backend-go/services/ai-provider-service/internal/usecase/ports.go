// Package usecase holds ai-provider-service's application services and the
// ports they need — defined here, implemented in internal/adapter/*, per
// the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
//
// The single most important rule for this package, closing TS Gap 2
// (ai-provider-service.md §9): CredentialBrokerClient is the ONLY port that
// ever touches credential material, and even it only ever sees/returns
// opaque CredentialRef values, never plaintext or ciphertext bytes. No type
// in this package holds a secret.
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/adapter/eventbus"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

// ListAccountsFilter narrows ProviderAccountRepository.List. ScopeRefID is
// interpreted against Scope: a user id when Scope == ScopeUser, a project id
// when Scope == ScopeProject, ignored when Scope == ScopeServer. Leaving
// Scope at its zero value lists every scope for the tenant.
type ListAccountsFilter struct {
	TenantID     string
	Scope        domain.AccountScope // zero value = any scope
	ScopeRefID   string
	DevServerID  string              // optional — empty means no filter
	ProviderType domain.ProviderType // zero value = any provider, matches DevServerID's "empty = no filter" convention
}

// UpdateFields is UpdateAccount's usecase input — only the 3 fields
// ai-provider-service.md documents as user-editable outside the status
// machine.
type UpdateFields struct {
	TenantID  string
	AccountID string
	Label     string
	ModelHint string
	BaseURL   string
}

// UpdateStatusInput is the single mutation entry point for an existing
// account's lifecycle fields — status transitions, and (for RotateKey) the
// new CredentialRef and rotation grace deadline in the same call, so a
// rotation never leaves the row in a state where Status says "rotating" but
// CredentialRef still points at the old (about-to-be-revoked) secret.
// Zero-value CredentialRef/RotationGraceUntil means "leave unchanged".
type UpdateStatusInput struct {
	TenantID           string
	AccountID          string
	Status             domain.AccountStatus
	HealthDetail       *string // nil = leave unchanged
	CredentialRef      string  // empty = unchanged
	RotationGraceUntil *time.Time
}

// ProviderAccountRepository is the persistence port for provider account
// metadata. Implemented by internal/adapter/postgres against this service's
// own database — see architecture/05-data-architecture.md's
// database-per-service rule. Every method is tenant-scoped by parameter,
// never inferred, per that doc's tenant-isolation rule.
type ProviderAccountRepository interface {
	Create(ctx context.Context, account domain.ProviderAccount) error
	Get(ctx context.Context, tenantID, id string) (domain.ProviderAccount, error)
	List(ctx context.Context, filter ListAccountsFilter) ([]domain.ProviderAccount, error)
	UpdateStatus(ctx context.Context, in UpdateStatusInput) (domain.ProviderAccount, error)
	// Update mutates the user-editable metadata fields — Label/ModelHint/
	// BaseURL — distinct from UpdateStatus, which only mutates lifecycle
	// fields (status/credential_ref/rotation grace). Never touches Status
	// or CredentialRef.
	Update(ctx context.Context, in UpdateFields) (domain.ProviderAccount, error)
	// Delete soft-deletes: sets status='revoked' and deleted_at=now(), never
	// a hard DELETE — preserves the row for audit and usage_daily's FK.
	Delete(ctx context.Context, tenantID, accountID string) error
	// MarkQuotaWarningSent implements the 80%-warning idempotency guard —
	// see migrations/0005_health_and_usage_writes.up.sql's
	// quota_warning_sent_date column.
	MarkQuotaWarningSent(ctx context.Context, tenantID, accountID string, day time.Time) error
}

// UsageRepository is the persistence port for the daily quota/spend rollup
// (ai_provider.usage) — a wholly separate axis from ProviderAccountRepository
// per ai-provider-service.md §5, kept as its own interface so a future
// implementation can back it with a different store (e.g. a cache) without
// touching account CRUD.
type UsageRepository interface {
	GetToday(ctx context.Context, tenantID, accountID string, day time.Time) (domain.QuotaState, error)
	// IncrementUsage upserts today's rollup row (tokens/requests/cost added
	// to whatever's already there) and returns the POST-increment state, so
	// the caller can immediately compare against quota without a second read.
	IncrementUsage(ctx context.Context, tenantID, accountID string, day time.Time, tokensUsed int64, requestCount int64, costUSD float64) (domain.QuotaState, error)
}

// ClaimedHealthCheckBatch mirrors automation-service's ClaimedBatch — the
// claim transaction stays open across dispatch (at-least-once: a crash
// mid-batch must not silently skip the next tick's retry).
type ClaimedHealthCheckBatch interface {
	Accounts() []domain.ProviderAccount
	// RecordResult persists one account's classification within the SAME
	// transaction the claim lock is held in.
	RecordResult(ctx context.Context, accountID string, status domain.AccountStatus, healthDetail *string, latencyMs *int, checkedAt time.Time) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// DueHealthCheckClaimer — same SELECT...FOR UPDATE SKIP LOCKED shape as
// automation-service's DueAutomationClaimer.
type DueHealthCheckClaimer interface {
	ClaimDue(ctx context.Context, now time.Time, staleness time.Duration, limit int32) (ClaimedHealthCheckBatch, error)
}

// OutboxEnqueuer is the narrow write port both ReconcileProviderHealth and
// RecordTokenUsage use to write into ai_provider.outbox within their own
// already-open transaction — same table SOL-AIP-01's CreateAccount writes
// its registration event into (TASK-AIP-01-07).
type OutboxEnqueuer interface {
	Enqueue(ctx context.Context, subject string, tenantID string, payload map[string]any) error
}

// CredentialRef is what CredentialBrokerClient returns — an opaque pointer
// credential-broker-service resolves later, on the execution plane, never a
// secret value. Mirrors the WriteCredentialResponse/RotateKeyResponse
// metadata shape from ai-provider-service.md §3 without any of the fields
// that would carry actual key material.
type CredentialRef struct {
	ID     string // credential-broker-service's opaque metadata id
	Status string // e.g. "pending_push", "active" — never a secret
}

// WriteCredentialInput carries the transport-layer-encrypted blob the
// browser already produced, unopened, to credential-broker-service — this
// service forwards it without ever decrypting it (ai-provider-service.md
// §3's WriteCredentialRequest note). EncryptedBlob is opaque ciphertext
// bytes from the caller's perspective; nothing in this package or its
// callers inspects or decrypts it.
type WriteCredentialInput struct {
	TenantID string
	// OwnerID mirrors credentialbroker.proto's WriteCredentialRequest.owner_id
	// ("user id or service name") — CreateAccount.Execute populates it from
	// the account's UserID/ProjectID scope, falling back to this service's
	// own name for server-scoped accounts with no specific owning user (see
	// create_account.go). credential-broker-service rejects an empty
	// OwnerID (WriteCredentialInput's own validation, "tenant_id and
	// owner_id are required"), so this must never be left blank.
	OwnerID       string
	EncryptedBlob []byte
}

// CredentialBrokerClient is the ONLY way this service's usecase layer ever
// touches credential material — satisfied by
// internal/adapter/grpcclient, which calls credential-broker-service's gRPC
// API. This service has no adapter/vault/ package at all (§6, §9): only
// credential-broker-service talks to Vault directly.
//
// STUB NOTICE: credential-broker-service does not exist yet in this
// workspace (see that service's own scaffold directory, currently a
// placeholder). internal/adapter/grpcclient's implementation of this
// interface is a stub that returns synthesized-but-opaque CredentialRef
// values — never plaintext, never fake ciphertext — so the security
// property "this service never sees a secret" holds even before the real
// dependency is wired in. Replace the stub's dial target and remove its
// TODOs once credential-broker-service ships (ai-provider-service.md §10:
// phase 2, built and cut over together).
type CredentialBrokerClient interface {
	// WriteCredential forwards an encrypted blob unopened and returns a
	// pointer to where credential-broker-service stored it.
	WriteCredential(ctx context.Context, in WriteCredentialInput) (CredentialRef, error)
	// RotateCredential asks credential-broker-service to rotate the secret
	// behind an existing ref, returning the new ref. The old ref remains
	// valid until RotationGraceUntil (see UpdateStatusInput).
	RotateCredential(ctx context.Context, credentialRef string) (CredentialRef, error)
	// ResolveCredential asks credential-broker-service for the current
	// metadata/status of a credential ref (e.g. whether ciphertext push to
	// the target dev server has been confirmed). It intentionally returns
	// only a CredentialRef — status metadata — never the underlying secret;
	// resolving to plaintext only ever happens on the execution plane
	// (ai-provider-service.md §9), which this service is not.
	ResolveCredential(ctx context.Context, credentialRef string) (CredentialRef, error)
	// RevokeCredential — needed for CreateAccount's test-before-save
	// rollback path (TASK-AIP-01-06). Mirrors credential-broker-service.md
	// §3's RevokeCredential RPC (RevokeCredentialRequest -> Empty).
	RevokeCredential(ctx context.Context, credentialRef string) error
}

// ConnectionTestResult is TestConnection's usecase-level result — mirrors
// TestConnectionResponse{success, message}. No field here can ever hold
// key material — the agent decrypts locally and returns only a
// success/failure verdict.
type ConnectionTestResult struct {
	Success bool
	Message string
	// RateLimited/ResetAtMs surface a rate-limit signal from the agent's
	// result map (SOL-MB-02) — see TestConnection.Execute, which publishes
	// RateLimitEventPublisher.PublishRateLimited when RateLimited is true.
	RateLimited bool
	ResetAtMs   *int64
}

// InfraFleetClient is the narrow port TestConnection uses to reach
// infra-fleet-service's Relay RPC — NOT to be confused with
// infra-fleet-service's own Relay RPC on the other side of this call.
// Implemented by internal/adapter/grpcclient against a real
// infra-fleet-service gRPC connection. ai-provider-service never gains its
// own Dev Server Agent adapter (§6) — this is the one indirect path to the
// execution plane, entirely mediated by infra-fleet-service.
type InfraFleetClient interface {
	// Relay resolves devServerID to its current active connectionId (via
	// infra-fleet-service's ResolveConnection with dev_server_id set — see
	// specs/backend-go/bugs/missing-v1/tasks/TASK-025-add-infrafleet-connection-resolve-by-devserver-worktree.md)
	// then calls infra-fleet-service's Relay RPC with method/params.
	// Returns the agent's decoded JSON-RPC result.
	Relay(ctx context.Context, devServerID, method string, params map[string]any) (map[string]any, error)
}

// RateLimitEventPublisher is the port TestConnection uses to notify
// notification-service (SOL-MB-02) when a provider connection test comes
// back rate-limited. Implemented by internal/adapter/eventbus against NATS
// JetStream. A nil value (e.g. NATS unavailable at startup) means
// TestConnection degrades to skipping the publish — see its Execute.
type RateLimitEventPublisher interface {
	PublishRateLimited(ctx context.Context, tenantID string, payload eventbus.RateLimitPayload) error
}

// ProviderResolutionPort is the interface the ResolveProvider usecase
// implements over ProviderAccountRepository — extracted as a port (rather
// than callers depending on *ResolveProvider directly) so this service's
// hot-path spawn-time cascade (ai-provider-service.md §4, §8) can be
// exercised through a narrow, mockable interface by any future in-process
// caller or test double. Deliberately makes no cross-service call: the
// cascade reads only this service's own accounts table, which is what keeps
// Resolve's p99 < 20ms target achievable (§8).
type ProviderResolutionPort interface {
	Resolve(ctx context.Context, in ResolveProviderInput) (domain.ProviderAccount, error)
}
