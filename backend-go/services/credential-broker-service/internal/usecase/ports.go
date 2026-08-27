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
	// GetByOwner fetches the most recent non-revoked row for
	// (tenantID, category, ownerID) — the lookup key
	// ResolveCredentialByOwner uses for callers that know which
	// tenant+category+logical-owner they need (e.g.
	// scm-integration-service resolving "this tenant's github token")
	// rather than an opaque credential_id (see credentialbroker.proto's
	// ResolveCredentialByOwner doc comment). Returns
	// domain.ErrCredentialNotFound (wrapped) when no matching row exists.
	GetByOwner(ctx context.Context, tenantID string, category domain.Category, ownerID string) (domain.CredentialMetadata, error)
	// ListByCategory returns every non-revoked row for (tenantID, category)
	// — backs ListCredentialsByCategory ("which owner_ids have a credential
	// in this category").
	ListByCategory(ctx context.Context, tenantID string, category domain.Category) ([]domain.CredentialMetadata, error)
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
	// RevokeSecret permanently deletes the material at path — metadata and
	// every version — so a subsequent KVRead can never return usable data
	// again, including via KV v2's version history. The real adapter
	// (internal/adapter/vault) implements this via
	// common/secrets.Client.KVDestroyMetadata, Vault's native
	// `DELETE <mount>/metadata/<path>` call — see that package's doc
	// comment.
	RevokeSecret(ctx context.Context, mount, path string) error
}

// TxRunner wraps a metadata mutation and its audit-log append in one
// Postgres transaction — added to satisfy credential-broker-service.md
// §8's requirement that the two land in "the same Postgres transaction,"
// not as two independent statements against the same pool (this package's
// prior shape).
//
// Design choice: rather than inventing separate transaction-scoped
// interfaces, RunInTx hands fn a CredentialMetadataRepository/
// AuditRepository pair already scoped to the transaction it opened —
// reusing the exact same port shapes callers already know. WriteCredential/
// RotateCredential/RevokeCredential (the three usecases that mutate
// metadata and append an audit row) call through the repos fn receives
// instead of the ones injected at construction time; usecases that only
// read (ResolveCredential and friends) never need this port and are
// unaffected. If fn returns a non-nil error the transaction rolls back —
// both the metadata mutation and the audit append vanish together — so the
// existing "an unaudited access/mutation is a failed operation" invariant
// becomes a real atomicity guarantee instead of just an error-propagation
// convention. Implemented by internal/adapter/postgres.Repository via
// pgx.BeginFunc.
type TxRunner interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context, metadataRepo CredentialMetadataRepository, auditRepo AuditRepository) error) error
}
