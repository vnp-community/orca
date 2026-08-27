// Package usecase holds auth-service's application services and the ports
// they need — defined here, implemented in internal/adapter/*, per the
// Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"
	"errors"
	"time"

	jose "github.com/go-jose/go-jose/v4"

	"github.com/stablyai/orca-go/common/jwtauth"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// Sentinel errors an adapter/postgres implementation wraps (fmt.Errorf
// "...: %w") and a usecase unwraps via errors.Is, so the usecase layer can
// tell "not found"/"already exists" apart from a generic infrastructure
// failure without either layer importing the other's implementation
// package — only this shared, dependency-free set of values.
var (
	ErrUserNotFound      = errors.New("usecase: user not found")
	ErrUserAlreadyExists = errors.New("usecase: a user with this email already exists in this tenant")
	ErrSessionNotFound   = errors.New("usecase: session not found")
	ErrPolicyNotFound    = errors.New("usecase: access policy not found")
)

// UserRepository is the persistence port for users. Implemented by
// internal/adapter/postgres against auth-service's own database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule.
//
// GetUserByEmail and CreateUser carry the bcrypt password hash as an extra
// return/parameter rather than a field on domain.User — the hash is
// storage-layer material the Login/CreateUser usecases need but which has
// no business meaning to any other consumer of a domain.User value (see
// domain.User's doc comment).
type UserRepository interface {
	// CreateUser persists a new user together with its password hash.
	// Returns an error satisfying errors.Is(err, ErrEmailAlreadyExists) if
	// (tenant_id, email) already exists.
	CreateUser(ctx context.Context, user domain.User, passwordHash string) (domain.User, error)
	// GetUserByEmail looks up a user by email and returns its password hash
	// alongside it, for Login to verify against. The proto's LoginRequest
	// carries no tenant discriminator (see auth-service's README "Known
	// gaps"), so lookup is by email alone.
	GetUserByEmail(ctx context.Context, email string) (domain.User, string, error)
	GetUserByID(ctx context.Context, userID string) (domain.User, error)
	ListUsers(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.User, string, error)
	UpdateUserRole(ctx context.Context, userID string, role domain.Role) (domain.User, error)
	// SetActive flips a user's is_active flag — the admin-console
	// deactivate/reactivate operations (internal/usecase/deactivate_user.go,
	// reactivate_user.go). Idempotent: setting an already-matching value is
	// not an error.
	SetActive(ctx context.Context, userID string, active bool) error
	// HasAnyUsers reports whether ANY row exists in the users table,
	// across every tenant — deliberately not tenant-scoped, since it
	// exists only for Bootstrap's "is this a completely fresh deployment"
	// check (internal/usecase/bootstrap.go), which by definition runs
	// before any tenant context exists to scope by.
	HasAnyUsers(ctx context.Context) (bool, error)
	// Count returns the total number of users across every tenant — backs
	// GetAdminStats's total_users field (internal/usecase/get_admin_stats.go).
	Count(ctx context.Context) (int32, error)
}

// SessionRepository is the persistence port for sessions. Only the SHA-256
// hash of a session token is ever passed across this interface — see
// domain.Session's doc comment.
type SessionRepository interface {
	CreateSession(ctx context.Context, session domain.Session) error
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (domain.Session, error)
	RevokeSession(ctx context.Context, tokenHash string, revokedAt time.Time) error
	// ListForUser returns every session (revoked or not, expired or not)
	// for userID — the admin-console session-inspection view
	// (internal/usecase/list_sessions_for_user.go), which needs the full
	// history, not just currently-valid sessions.
	ListForUser(ctx context.Context, userID string) ([]domain.Session, error)
	// RevokeAllForUser force-revokes every currently-unrevoked session for
	// userID and returns how many were revoked — the admin-console "kill
	// all sessions" operation (internal/usecase/force_revoke_all_sessions.go),
	// distinct from RevokeSession's single-token revoke.
	RevokeAllForUser(ctx context.Context, userID string, revokedAt time.Time) (int32, error)
	// CountActive returns the number of currently-valid (unrevoked,
	// unexpired) sessions across every tenant — backs GetAdminStats's
	// active_sessions field.
	CountActive(ctx context.Context, now time.Time) (int32, error)
}

// AccessPolicyRepository is the persistence port for admin-console access
// policies. Per auth-service.md:150, UpdateAccessPolicy never mutates a row
// in place — it always inserts a NEW version row, so this port has no
// "update" method, only InsertPolicyVersion, matching the append-only
// versioning contract every usecase in access_policy CRUD relies on.
type AccessPolicyRepository interface {
	// InsertPolicyVersion appends a new version row for p.ID (or the first
	// row, at version 1, if p.ID has no prior version).
	InsertPolicyVersion(ctx context.Context, p domain.AccessPolicy) error
	// GetLatestPolicy returns the highest-version row for id, or an error
	// satisfying errors.Is(err, ErrPolicyNotFound) if no version exists.
	GetLatestPolicy(ctx context.Context, id string) (domain.AccessPolicy, error)
	// ListLatestPolicies returns one row per policy id — its latest
	// version only, never every historical version.
	ListLatestPolicies(ctx context.Context, pageToken string, pageSize int32) ([]domain.AccessPolicy, string, error)
	// DeletePolicy removes every version row for id.
	DeletePolicy(ctx context.Context, id string) error
	// CountDistinctIDs returns the number of distinct policy ids (not the
	// number of version rows) — backs GetAdminStats's total_policies field.
	CountDistinctIDs(ctx context.Context) (int32, error)
}

// PolicyDataPublisher publishes a newly-versioned AccessPolicy to the OPA
// bundle registry so policy-engine instances pick up the change — per
// auth-service.md:194. Kept behind this port (rather than a concrete OPA
// bundle-registry client) so UpdateAccessPolicy compiles and is swappable
// today even though a real bundle-registry integration isn't wired in this
// scaffold yet — see internal/adapter/opaclient's README "Known gaps".
type PolicyDataPublisher interface {
	PublishPolicyChange(ctx context.Context, policy domain.AccessPolicy) error
}

// AuditRepository is the persistence port for the append-only audit log.
// There is deliberately no Update/Delete method — see domain.AuditEntry's
// doc comment.
type AuditRepository interface {
	Append(ctx context.Context, entry domain.AuditEntry) error
	Query(ctx context.Context, tenantID string, since time.Time, pageToken string, pageSize int32) ([]domain.AuditEntry, string, error)
}

// PasswordHasher hashes and verifies passwords. Implemented by
// internal/adapter/bcrypt — kept behind this port so domain/ and the rest
// of usecase/ never import bcrypt directly, per auth-service.md §6.
type PasswordHasher interface {
	Hash(password string) (string, error)
	// Compare returns nil if password matches hash, a non-nil error
	// otherwise. Never distinguishes "wrong password" from other failure
	// modes in its error value in a way callers should branch on — Login
	// treats any non-nil error as invalid credentials.
	Compare(hash, password string) error
}

// Clock abstracts time.Now so session-expiry logic (Login, ValidateSession)
// is deterministically testable against fakes, per
// specs/backend-go/standards/testing-strategy.md.
type Clock interface {
	Now() time.Time
}

// SystemClock is the real Clock, wired in cmd/server/main.go.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

// OPAClient is the authorization port requireAdminActor uses for the "is
// this actor an admin" decision — implemented by internal/adapter/opaclient
// against the shared embedded OPA evaluator (common/policy), consuming
// backend-go/policy/orca-authz/admin.rego's data.orca.authz.admin.allow
// rule. Mirrors task-service/annotation-service's own OPAClient port shape.
type OPAClient interface {
	// Decision reports whether actor is authorized as admin, per
	// admin.rego's {"actor": {"id", "role"}} input contract.
	Decision(ctx context.Context, actor domain.User) (bool, error)
}

// TokenSigner is the port IssueServiceToken/GetJWKS sign and publish JWTs
// through — implemented by internal/adapter/vault against a Vault Transit
// key, kept behind this interface so this usecase package never imports
// common/secrets or Vault SDK types directly, per auth-service's other
// ports (PasswordHasher, etc.) following the same Dependency Inversion
// convention.
type TokenSigner interface {
	// Sign mints a compact-serialized RS256 JWT for claims.
	Sign(ctx context.Context, claims jwtauth.Claims) (string, error)
	// PublicJWKS returns the current+previous signing key version as an
	// RFC 7517 JWK Set, for GetJWKS to publish.
	PublicJWKS(ctx context.Context) (jose.JSONWebKeySet, error)
}

// DeviceKeyExchanger generates NaCl X25519 keypairs and computes shared
// secrets for BL-MB-01's pairing handshake. Implemented by
// internal/adapter/nacl.KeyExchanger over golang.org/x/crypto/nacl/box.
type DeviceKeyExchanger interface {
	GenerateEphemeralKeypair() (pub, priv []byte, err error)
	SharedSecret(priv, peerPub []byte) ([]byte, error)
}

// SharedSecretSealer mediates a paired device's shared secret through
// Vault Transit — never a plaintext value in this service's own Postgres
// row, mirroring notification-service's/infra-fleet-service's Vault-mediated
// secret pattern extended here to a per-device-pairing secret class.
// Implemented by internal/adapter/vault.SharedSecretSealer.
type SharedSecretSealer interface {
	Encrypt(ctx context.Context, plaintext []byte) (ciphertext []byte, keyRef string, err error)
	Decrypt(ctx context.Context, ciphertext []byte, keyRef string) ([]byte, error)
}

// PairingSessionRepository is the persistence port for the ephemeral
// server-side state of an in-progress QR pairing attempt (BL-MB-01).
// Implemented by internal/adapter/postgres.PairingSessionStore.
type PairingSessionRepository interface {
	Save(ctx context.Context, session domain.PairingSession) error
	// GetAndConsume atomically marks the session consumed and returns it —
	// the one statement that enforces BR-MB-02 (one-time use) across two
	// concurrent CompleteDevicePairing calls racing on the same token.
	// Returns an error satisfying errors.Is(err, domain.ErrPairingTokenNotFound)
	// if no unconsumed row matches id.
	GetAndConsume(ctx context.Context, id string) (domain.PairingSession, error)
}

// PairedDeviceRepository is the persistence port for durably paired mobile
// devices. Implemented by internal/adapter/postgres.PairedDeviceStore.
type PairedDeviceRepository interface {
	Save(ctx context.Context, device domain.PairedDevice) error
	// CountActive backs BR-MB-03's max-3-active-devices cap check.
	CountActive(ctx context.Context, tenantID, userID string) (int, error)
	// Get returns an error satisfying errors.Is(err, domain.ErrDeviceNotFound)
	// if id doesn't exist.
	Get(ctx context.Context, id string) (domain.PairedDevice, error)
	List(ctx context.Context, tenantID, userID string) ([]domain.PairedDevice, error)
	// RevokeAndWipeSecret marks a device revoked AND nulls its shared-secret
	// ciphertext in the same statement — BR-MB-04's enforcement mechanism.
	RevokeAndWipeSecret(ctx context.Context, id string) error
	// Touch updates last_used_at — called best-effort from
	// ResolveDeviceSharedSecret.
	Touch(ctx context.Context, id string, now time.Time) error
}
