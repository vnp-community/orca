// Package domain holds credential-broker-service's entities and value
// objects. Per specs/backend-go/architecture/03-clean-architecture-guidelines.md,
// this package has zero imports outside stdlib + other domain/ packages —
// no database, no gRPC, no Vault SDK.
//
// The single most important property of every type in this package: NONE of
// them has a field capable of holding a secret value, ciphertext, or
// decryption key. This is enforced by never adding such a field, not merely
// by review discipline — see credential-broker-service.md §5 ("no secret
// columns, ever") and §9 ("plaintext-in-memory-only-for-request-duration").
package domain

import (
	"errors"
	"time"
)

// Category is the kind of secret material a CredentialMetadata row points
// at — mirrors credentialbrokerv1.CredentialCategory 1:1 (see
// proto/orca/credentialbroker/v1/credentialbroker.proto), just as a
// lowercase-snake-case domain value rather than a generated Go enum, so
// domain/ stays free of the generated-proto import per the package doc
// above.
type Category string

const (
	CategoryScmOAuth          Category = "scm_oauth"
	CategoryIssueTrackerOAuth Category = "issue_tracker_oauth"
	CategoryAiProviderKey     Category = "ai_provider_key"
	CategorySsh               Category = "ssh"
	CategoryServiceSecret     Category = "service_secret"
)

// Valid reports whether c is one of the known category values.
func (c Category) Valid() bool {
	switch c {
	case CategoryScmOAuth, CategoryIssueTrackerOAuth, CategoryAiProviderKey, CategorySsh, CategoryServiceSecret:
		return true
	default:
		return false
	}
}

// VaultEngine identifies which Vault secrets engine a category's material is
// ultimately protected by.
type VaultEngine string

const (
	VaultEngineTransit VaultEngine = "transit"
	VaultEngineKV2     VaultEngine = "kv2"
)

// Engine derives this category's primary Vault engine from Category alone —
// never independently settable, per credential-broker-service.md §4's
// invariant ("VaultEngine is derived from Category in the constructor...
// never independently settable — prevents a caller from requesting a
// category/engine mismatch"). AI_PROVIDER_KEY is the category that exists
// specifically to round-trip through Transit encrypt/decrypt (see
// ai-provider-service.md §9's execution-plane decrypt pattern); every other
// category's material is modeled as KV v2-addressable.
//
// NOTE: internal/usecase's current WriteCredential/ResolveCredential
// implementation does not yet branch its Vault call pattern on Engine() —
// see this service's README "Known gaps" for why, and what wiring the SSH
// secrets engine and an execution-plane-only Transit decrypt path would
// need. Engine() exists today as the enforced domain invariant this design
// requires, ready for that usecase-level branching to consume.
func (c Category) Engine() VaultEngine {
	if c == CategoryAiProviderKey {
		return VaultEngineTransit
	}
	return VaultEngineKV2
}

// Status is a CredentialMetadata's lifecycle state.
type Status string

const (
	StatusPending  Status = "pending"
	StatusActive   Status = "active"
	StatusRotating Status = "rotating"
	StatusRevoked  Status = "revoked"
	StatusError    Status = "error"
)

// Valid reports whether s is one of the known status values.
func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusActive, StatusRotating, StatusRevoked, StatusError:
		return true
	default:
		return false
	}
}

// Domain errors — the small, closed set internal/usecase and
// internal/adapter/grpc map to gRPC status codes via common/apperrors.
var (
	ErrEmptyID                = errors.New("domain: id is required")
	ErrEmptyTenant            = errors.New("domain: tenant_id is required")
	ErrEmptyOwner             = errors.New("domain: owner_id is required")
	ErrInvalidCategory        = errors.New("domain: invalid credential category")
	ErrInvalidStatus          = errors.New("domain: invalid credential status")
	ErrEmptyVaultPath         = errors.New("domain: vault_path is required")
	ErrCredentialNotFound     = errors.New("domain: credential not found")
	ErrCredentialRevoked      = errors.New("domain: credential is revoked")
	ErrCategoryEngineMismatch = errors.New("domain: category/vault engine mismatch")
)

// CredentialMetadata is a pointer to secret material that lives in Vault.
// Every field is a pointer, a status enum, or a timestamp — NEVER a secret
// value. See the package doc comment above; this invariant is why this
// struct has no field like "Value", "Ciphertext", or "Key".
type CredentialMetadata struct {
	ID        string
	TenantID  string
	OwnerID   string // user id or service name, per credentialbroker.proto
	Category  Category
	Status    Status
	VaultPath string // Vault KV v2 path reference only, never a value
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewCredentialMetadata constructs a CredentialMetadata, enforcing the
// invariants a row must satisfy to be meaningful. VaultPath is required
// because a metadata row that doesn't point anywhere in Vault isn't a valid
// pointer — see credential-broker-service.md §10's cutover-ordering rule: a
// metadata row is only ever created AFTER the Vault write it points at has
// been confirmed, so a partially-written credential never has a row
// pointing at invalid or missing ciphertext.
func NewCredentialMetadata(
	id, tenantID, ownerID string,
	category Category,
	vaultPath string,
	status Status,
	now time.Time,
) (CredentialMetadata, error) {
	if id == "" {
		return CredentialMetadata{}, ErrEmptyID
	}
	if tenantID == "" {
		return CredentialMetadata{}, ErrEmptyTenant
	}
	if ownerID == "" {
		return CredentialMetadata{}, ErrEmptyOwner
	}
	if !category.Valid() {
		return CredentialMetadata{}, ErrInvalidCategory
	}
	if vaultPath == "" {
		return CredentialMetadata{}, ErrEmptyVaultPath
	}
	if status == "" {
		status = StatusPending
	}
	if !status.Valid() {
		return CredentialMetadata{}, ErrInvalidStatus
	}
	return CredentialMetadata{
		ID:        id,
		TenantID:  tenantID,
		OwnerID:   ownerID,
		Category:  category,
		Status:    status,
		VaultPath: vaultPath,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// IsRevoked reports whether this credential can no longer be resolved.
func (m CredentialMetadata) IsRevoked() bool {
	return m.Status == StatusRevoked
}

// Revoke returns a copy with Status transitioned to revoked — pure,
// side-effect-free, unit-testable without touching Vault or Postgres. The
// usecase layer is responsible for actually invalidating the Vault-side
// material (see internal/usecase/revoke_credential.go); this method only
// updates the in-memory representation of the row the usecase then persists.
func (m CredentialMetadata) Revoke(now time.Time) CredentialMetadata {
	m.Status = StatusRevoked
	m.UpdatedAt = now
	return m
}

// WithStatus returns a copy with Status (and UpdatedAt) transitioned —
// used by RotateCredential to move a row through rotating -> active.
func (m CredentialMetadata) WithStatus(status Status, now time.Time) CredentialMetadata {
	m.Status = status
	m.UpdatedAt = now
	return m
}
