// Package usecase holds credential-broker-service's application services
// and the ports they need — defined here, implemented in
// internal/adapter/*, per the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

// CredentialMetadataRepository is the persistence port for credential
// pointers — implemented by internal/adapter/postgres against this
// service's own `credential` schema. No method on this interface ever
// takes or returns a secret value; see migrations/0001_init.up.sql's "no
// secret columns, ever" comment.
type CredentialMetadataRepository interface {
	// Create persists a new metadata row. Callers create a row ONLY after
	// the Vault write it points at has been confirmed — see
	// write_credential.go's doc comment and credential-broker-service.md
	// §10's cutover-ordering rule.
	Create(ctx context.Context, metadata domain.CredentialMetadata) error
	// Get fetches a metadata row by id. Returns domain.ErrCredentialNotFound
	// (wrapped) when no row exists.
	Get(ctx context.Context, id string) (domain.CredentialMetadata, error)
	// UpdateStatus transitions a row's lifecycle status — used by
	// RotateCredential (-> active) and RevokeCredential (-> revoked).
	UpdateStatus(ctx context.Context, id string, status domain.Status, now time.Time) error
}

// AuditRepository is the append-only access-audit-log port. Per
// credential-broker-service.md §8, "the audit log write must never be
// best-effort or droppable... if the audit write fails, the operation
// fails." Every usecase in this package calls Append synchronously and
// propagates its error — see audit.go's appendAudit helper, which is the
// only call site.
type AuditRepository interface {
	Append(ctx context.Context, entry domain.AccessAuditEntry) error
}

// SecretStore abstracts over Vault's engines. Its method shapes mirror
// common/secrets.Client's actual methods 1:1 (TransitEncrypt/TransitDecrypt/
// KVWrite/KVRead) so internal/adapter/vault's real implementation is a thin
// pass-through, not a reinterpretation — see that package's doc comment.
// usecase/ depends only on this port, never on the Vault SDK or
// common/secrets directly, per the Dependency Inversion rule this package's
// doc comment describes.
type SecretStore interface {
	// TransitEncrypt/TransitDecrypt mirror common/secrets.Client's methods
	// of the same name exactly (parameter order, return shape).
	TransitEncrypt(ctx context.Context, keyName string, plaintext []byte) (ciphertext string, err error)
	TransitDecrypt(ctx context.Context, keyName string, ciphertext string) (plaintext []byte, err error)
	// KVWrite/KVRead mirror common/secrets.Client's methods of the same
	// name exactly.
	KVWrite(ctx context.Context, mount, path string, data map[string]any) error
	KVRead(ctx context.Context, mount, path string) (map[string]any, error)
	// RevokeSecret invalidates the material at path so a subsequent KVRead
	// no longer returns usable data. common/secrets.Client has no native
	// KV-delete-version or Transit-key-delete method today, so the real
	// adapter (internal/adapter/vault) implements this as a genuine
	// overwrite-with-empty-payload KVWrite call — not a stub, but also not
	// equivalent to Vault's native "destroy version" API. See that
	// package's doc comment and this service's README "Known gaps".
	RevokeSecret(ctx context.Context, mount, path string) error
}
