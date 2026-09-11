//go:build integration

// Integration tests run against a real MySQL via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/mysql/...`. Mirrors
// internal/adapter/postgres's test names/shape where a Postgres test of
// the same behavior exists (repository_test.go, audit_repository_test.go)
// — auth-service's Postgres adapter has NO integration test at all for
// ServiceTokenRepository/AccessPolicyRepository/SsoIdentityRepository/
// SsoGroupRoleMappingRepository/PairingSessionRepository/
// PairedDeviceRepository (confirmed via `ls`, not assumed), so those get
// fresh MySQL-only round-trip coverage here instead of a 1:1 port.
package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

// setupMySQLDB starts a disposable MySQL container, runs every
// migrations/mysql/*.sql file against it, and returns a ready
// *sql.DB — shared by setupRepository and the PairingSessionStore/
// PairedDeviceStore tests in this package (both need the same underlying
// connection pool, unlike postgres/repository_test.go where each store
// constructor takes the same *pgxpool.Pool but this file only opens one).
func setupMySQLDB(t *testing.T) *sql.DB {
	t.Helper()
	// testutil.StartMySQL returns "mysql://root:orca@tcp(host:port)/db" —
	// valid as-is for golang-migrate's mysql driver CLI (used below), but
	// go-sql-driver/mysql's database/sql driver expects its own DSN format
	// with no "mysql://" scheme.
	rawDSN := testutil.StartMySQL(t, "auth")
	driverDSN := strings.TrimPrefix(rawDSN, "mysql://") + "?parseTime=true"

	migrationsPath, err := filepath.Abs("../../../migrations/mysql")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	cmd := exec.Command("migrate", "-path", migrationsPath, "-database", rawDSN, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running migrations: %v\n%s", err, out)
	}

	db, err := sql.Open("mysql", driverDSN)
	if err != nil {
		t.Fatalf("connecting to mysql: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("pinging mysql: %v", err)
	}
	return db
}

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	return New(setupMySQLDB(t))
}

func createTestUser(t *testing.T, repo *Repository, tenantID, email string) domain.User {
	t.Helper()
	user, err := domain.NewUser(uuid.NewString(), tenantID, email, "Test User", domain.RoleUser, true, time.Now())
	if err != nil {
		t.Fatalf("building user: %v", err)
	}
	if _, err := repo.CreateUser(context.Background(), user, "hash1"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

// --- UserRepository ---

func TestUserRepository_UpdateUser_PartialUpdatePreservesUntouchedFields(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, uuid.NewString(), "update-user@example.com")

	newEmail := "updated-email@example.com"
	updated, err := repo.UpdateUser(ctx, user.ID, &newEmail, nil, nil)
	if err != nil {
		t.Fatalf("update user: %v", err)
	}
	if updated.Email != newEmail {
		t.Errorf("expected email to be updated, got %q", updated.Email)
	}
	if updated.Name != "Test User" {
		t.Errorf("expected name to be preserved via COALESCE, got %q", updated.Name)
	}

	reread, err := repo.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("get user by id: %v", err)
	}
	if reread.Email != newEmail || reread.Name != "Test User" {
		t.Errorf("unexpected persisted user: %+v", reread)
	}
}

func TestUserRepository_UpdateUser_NonexistentUserFails(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	newName := "Ghost"
	_, err := repo.UpdateUser(ctx, uuid.NewString(), nil, &newName, nil)
	if err == nil {
		t.Fatal("expected an error for a nonexistent user")
	}
}

// TestUserRepository_UpdateUserRole_NoopRetryStillSucceeds proves the fix
// documented in user_repository.go's UpdateUserRole doc comment: setting a
// role to the value it already has must NOT report ErrUserNotFound just
// because MySQL's default RowsAffected() counts 0 for an unchanged value —
// a naive $N->? translation of the Postgres RETURNING-based version would
// fail this test.
func TestUserRepository_UpdateUserRole_NoopRetryStillSucceeds(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, uuid.NewString(), "role-retry@example.com")

	if _, err := repo.UpdateUserRole(ctx, user.ID, domain.RoleAdmin); err != nil {
		t.Fatalf("first update role: %v", err)
	}
	// Retry with the SAME role — MySQL's RowsAffected() is 0 here even
	// though the row unambiguously exists.
	got, err := repo.UpdateUserRole(ctx, user.ID, domain.RoleAdmin)
	if err != nil {
		t.Fatalf("expected idempotent no-op retry to succeed, got: %v", err)
	}
	if got.Role != domain.RoleAdmin {
		t.Errorf("expected role %q, got %q", domain.RoleAdmin, got.Role)
	}
}

func TestRepository_CreateUser_RejectsDuplicateEmailInTenant(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenantID := uuid.NewString()
	createTestUser(t, repo, tenantID, "dup@example.com")

	u2, err := domain.NewUser(uuid.NewString(), tenantID, "dup@example.com", "Second", domain.RoleUser, true, time.Now())
	if err != nil {
		t.Fatalf("building user: %v", err)
	}
	if _, err := repo.CreateUser(ctx, u2, "hash2"); err == nil {
		t.Fatal("expected an error for a duplicate (tenant_id, email)")
	}
}

func TestUserRepository_HasAnyUsersAndCount(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	has, err := repo.HasAnyUsers(ctx)
	if err != nil {
		t.Fatalf("has any users: %v", err)
	}
	if has {
		t.Fatal("expected no users on a fresh database")
	}
	createTestUser(t, repo, uuid.NewString(), "count-1@example.com")
	createTestUser(t, repo, uuid.NewString(), "count-2@example.com")

	has, err = repo.HasAnyUsers(ctx)
	if err != nil || !has {
		t.Fatalf("expected HasAnyUsers true, got %v, err %v", has, err)
	}
	n, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 users, got %d", n)
	}
}

// --- SessionRepository ---

func TestRepository_SessionLifecycle(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenantID := uuid.NewString()
	user := createTestUser(t, repo, tenantID, "session-user@example.com")

	now := time.Now().UTC().Truncate(time.Second)
	session, err := domain.NewSession(domain.HashSessionToken("raw-token"), user.ID, tenantID, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("building session: %v", err)
	}
	if err := repo.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	got, err := repo.GetSessionByTokenHash(ctx, session.TokenHash)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.UserID != user.ID || got.RevokedAt != nil {
		t.Errorf("unexpected session: %+v", got)
	}

	if err := repo.RevokeSession(ctx, session.TokenHash, now.Add(time.Minute)); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	got, err = repo.GetSessionByTokenHash(ctx, session.TokenHash)
	if err != nil {
		t.Fatalf("get session after revoke: %v", err)
	}
	if got.RevokedAt == nil {
		t.Error("expected RevokedAt to be set after revoke")
	}
}

// TestSessionRepository_RevokeSession_NoopRetryStillSucceeds proves the
// fix documented in session_repository.go's RevokeSession doc comment: a
// second revoke of an already-revoked session (same revokedAt value) must
// NOT report ErrSessionNotFound — a false negative on this security path
// would be worse than a correctness nit (see this rollout's brief).
// Additionally covers revoking with a genuinely unknown token_hash, which
// MUST still report ErrSessionNotFound.
func TestSessionRepository_RevokeSession_NoopRetryStillSucceeds(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenantID := uuid.NewString()
	user := createTestUser(t, repo, tenantID, "revoke-retry@example.com")
	now := time.Now().UTC().Truncate(time.Second)
	session, err := domain.NewSession(domain.HashSessionToken("raw-token-revoke-retry"), user.ID, tenantID, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("building session: %v", err)
	}
	if err := repo.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	revokedAt := now.Add(time.Minute)
	if err := repo.RevokeSession(ctx, session.TokenHash, revokedAt); err != nil {
		t.Fatalf("first revoke: %v", err)
	}
	// Same exact revokedAt value on retry — RowsAffected() is 0 under
	// MySQL's default semantics even though the session unambiguously
	// exists.
	if err := repo.RevokeSession(ctx, session.TokenHash, revokedAt); err != nil {
		t.Fatalf("expected idempotent revoke retry to succeed, got: %v", err)
	}

	if err := repo.RevokeSession(ctx, "does-not-exist-token-hash", revokedAt); err == nil {
		t.Fatal("expected revoking an unknown token_hash to fail")
	}
}

func TestSessionRepository_RoundTripsClientInfo(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenantID := uuid.NewString()
	user := createTestUser(t, repo, tenantID, "client-info-user@example.com")

	now := time.Now().UTC().Truncate(time.Second)
	session, err := domain.NewSession(domain.HashSessionToken("raw-token-with-info"), user.ID, tenantID, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("building session: %v", err)
	}
	session = session.WithClientInfo("203.0.113.7", "test-agent/1.0")
	if err := repo.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	got, err := repo.GetSessionByTokenHash(ctx, session.TokenHash)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.IP != "203.0.113.7" {
		t.Errorf("expected IP to round-trip, got %q", got.IP)
	}
	if got.UserAgent != "test-agent/1.0" {
		t.Errorf("expected UserAgent to round-trip, got %q", got.UserAgent)
	}

	bare, err := domain.NewSession(domain.HashSessionToken("raw-token-bare"), user.ID, tenantID, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("building bare session: %v", err)
	}
	if err := repo.CreateSession(ctx, bare); err != nil {
		t.Fatalf("create bare session: %v", err)
	}
	gotBare, err := repo.GetSessionByTokenHash(ctx, bare.TokenHash)
	if err != nil {
		t.Fatalf("get bare session: %v", err)
	}
	if gotBare.IP != "" || gotBare.UserAgent != "" {
		t.Errorf("expected empty IP/UserAgent for a bare session, got %+v", gotBare)
	}
}

func TestSessionRepository_DeleteExpiredBefore(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenantID := uuid.NewString()
	user := createTestUser(t, repo, tenantID, "reap-user@example.com")
	now := time.Now().UTC().Truncate(time.Second)

	expiredSession, err := domain.NewSession(domain.HashSessionToken("raw-token-expired"), user.ID, tenantID, now.Add(-48*time.Hour), now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("building expired session: %v", err)
	}
	if err := repo.CreateSession(ctx, expiredSession); err != nil {
		t.Fatalf("create expired session: %v", err)
	}
	activeSession, err := domain.NewSession(domain.HashSessionToken("raw-token-active"), user.ID, tenantID, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("building active session: %v", err)
	}
	if err := repo.CreateSession(ctx, activeSession); err != nil {
		t.Fatalf("create active session: %v", err)
	}

	n, err := repo.DeleteExpiredBefore(ctx, now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("delete expired before: %v", err)
	}
	if n != 1 {
		t.Errorf("expected exactly 1 row removed, got %d", n)
	}
	if _, err := repo.GetSessionByTokenHash(ctx, expiredSession.TokenHash); err == nil {
		t.Error("expected the expired session to have been removed")
	}
	if _, err := repo.GetSessionByTokenHash(ctx, activeSession.TokenHash); err != nil {
		t.Errorf("expected the active session to survive the reap, got %v", err)
	}
}

// --- ServiceTokenRepository ---

func TestServiceTokenRepository_Lifecycle(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, uuid.NewString(), "svc-token-user@example.com")
	now := time.Now().UTC().Truncate(time.Second)
	token := domain.IssuedServiceToken{JTI: uuid.NewString(), UserID: user.ID, Audience: "cli", IssuedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := repo.RecordIssuedToken(ctx, token); err != nil {
		t.Fatalf("record issued token: %v", err)
	}

	revoked, err := repo.IsRevoked(ctx, token.JTI)
	if err != nil || revoked {
		t.Fatalf("expected fresh token to not be revoked, got %v, err %v", revoked, err)
	}
	// Unknown jti reports false, not an error.
	revoked, err = repo.IsRevoked(ctx, "unknown-jti")
	if err != nil || revoked {
		t.Fatalf("expected unknown jti to report false/no error, got %v, err %v", revoked, err)
	}

	list, err := repo.ListServiceTokensForUser(ctx, user.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 token for user, got %d, err %v", len(list), err)
	}

	if err := repo.Revoke(ctx, token.JTI, now.Add(time.Minute)); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	revoked, err = repo.IsRevoked(ctx, token.JTI)
	if err != nil || !revoked {
		t.Fatalf("expected token to be revoked, got %v, err %v", revoked, err)
	}

	if err := repo.Revoke(ctx, "unknown-jti", now); err == nil {
		t.Fatal("expected revoking an unknown jti to fail")
	}
}

// TestServiceTokenRepository_Revoke_NoopRetryStillSucceeds — same class of
// RowsAffected pitfall as RevokeSession, for CLI-token revocation
// (RevokeCliToken usecase).
func TestServiceTokenRepository_Revoke_NoopRetryStillSucceeds(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, uuid.NewString(), "svc-token-retry@example.com")
	now := time.Now().UTC().Truncate(time.Second)
	token := domain.IssuedServiceToken{JTI: uuid.NewString(), UserID: user.ID, Audience: "cli", IssuedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := repo.RecordIssuedToken(ctx, token); err != nil {
		t.Fatalf("record issued token: %v", err)
	}

	revokedAt := now.Add(time.Minute)
	if err := repo.Revoke(ctx, token.JTI, revokedAt); err != nil {
		t.Fatalf("first revoke: %v", err)
	}
	if err := repo.Revoke(ctx, token.JTI, revokedAt); err != nil {
		t.Fatalf("expected idempotent revoke retry to succeed, got: %v", err)
	}
}

// --- AccessPolicyRepository ---

func TestAccessPolicyRepository_VersioningAndListLatest(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	p1v1, err := domain.NewAccessPolicy(uuid.NewString(), "policy-1", "role-definition", `{"a":1}`, 1, uuid.NewString(), now)
	if err != nil {
		t.Fatalf("building policy: %v", err)
	}
	if err := repo.InsertPolicyVersion(ctx, p1v1); err != nil {
		t.Fatalf("insert v1: %v", err)
	}
	p1v2 := p1v1
	p1v2.Version = 2
	p1v2.DocumentJSON = `{"a":2}`
	if err := repo.InsertPolicyVersion(ctx, p1v2); err != nil {
		t.Fatalf("insert v2: %v", err)
	}

	p2, err := domain.NewAccessPolicy(uuid.NewString(), "policy-2", "rate-tier", `{"b":1}`, 1, uuid.NewString(), now)
	if err != nil {
		t.Fatalf("building policy 2: %v", err)
	}
	if err := repo.InsertPolicyVersion(ctx, p2); err != nil {
		t.Fatalf("insert policy 2: %v", err)
	}

	latest, err := repo.GetLatestPolicy(ctx, p1v1.ID)
	if err != nil {
		t.Fatalf("get latest policy: %v", err)
	}
	// Compare semantically, not byte-for-byte: MySQL's JSON column type
	// canonicalizes whitespace on storage (`{"a":2}` round-trips as
	// `{"a": 2}`, with a space after the colon) — a harmless, well-known
	// dialect difference, not an application bug (DocumentJSON is treated
	// as an opaque blob by every caller, never string-compared). A
	// byte-exact assertion here would be testing MySQL's JSON
	// canonicalization behavior, not this repository's correctness.
	if latest.Version != 2 || !jsonEqual(t, latest.DocumentJSON, `{"a":2}`) {
		t.Errorf("expected version 2 with updated document, got %+v", latest)
	}

	// ListLatestPolicies must return exactly one row per distinct id — the
	// ROW_NUMBER() OVER (PARTITION BY id ...) translation of Postgres's
	// DISTINCT ON (id).
	list, _, err := repo.ListLatestPolicies(ctx, "", 50)
	if err != nil {
		t.Fatalf("list latest policies: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 distinct policies (latest version each), got %d: %+v", len(list), list)
	}
	for _, p := range list {
		if p.ID == p1v1.ID && p.Version != 2 {
			t.Errorf("expected policy-1's latest listed version to be 2, got %d", p.Version)
		}
	}

	count, err := repo.CountDistinctIDs(ctx)
	if err != nil || count != 2 {
		t.Fatalf("expected 2 distinct policy ids, got %d, err %v", count, err)
	}

	if err := repo.DeletePolicy(ctx, p1v1.ID); err != nil {
		t.Fatalf("delete policy: %v", err)
	}
	if _, err := repo.GetLatestPolicy(ctx, p1v1.ID); err == nil {
		t.Fatal("expected policy to be gone after DeletePolicy")
	}
}

// --- AuditRepository ---

func TestRepository_AuditLog_AppendAndQueryFiltersByTenant(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenant1 := uuid.NewString()
	tenant2 := uuid.NewString()

	e1, _ := domain.NewAuditEntry(uuid.NewString(), tenant1, uuid.NewString(), "user.login", "", "user", "target-1", nil, domain.OutcomeAllowed, "", time.Now())
	e2, _ := domain.NewAuditEntry(uuid.NewString(), tenant2, uuid.NewString(), "user.login", "", "user", "target-2", nil, domain.OutcomeAllowed, "", time.Now())
	if err := repo.Append(ctx, e1); err != nil {
		t.Fatalf("append e1: %v", err)
	}
	if err := repo.Append(ctx, e2); err != nil {
		t.Fatalf("append e2: %v", err)
	}

	entries, _, err := repo.Query(ctx, usecase.AuditQueryFilter{TenantID: tenant1}, "", 50)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(entries) != 1 || entries[0].TenantID != tenant1 {
		t.Errorf("expected only tenant1's entry, got %+v", entries)
	}
}

func TestAuditRepository_AppendRoundTripsMetadataAndIP(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenantID := uuid.NewString()
	actorID := uuid.NewString()
	now := time.Now()

	entry, err := domain.NewAuditEntry(uuid.NewString(), tenantID, actorID, "user.role_updated", "", "user", "u2",
		map[string]any{"from": "user", "to": "admin", "nested": map[string]any{"a": float64(1)}}, domain.OutcomeAllowed, "203.0.113.7", now)
	if err != nil {
		t.Fatalf("building entry: %v", err)
	}
	if err := repo.Append(ctx, entry); err != nil {
		t.Fatalf("append: %v", err)
	}

	entries, _, err := repo.Query(ctx, usecase.AuditQueryFilter{TenantID: tenantID}, "", 50)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	got := entries[0]
	if got.TargetType != "user" || got.TargetID != "u2" {
		t.Errorf("expected TargetType/TargetID to round-trip, got %q/%q", got.TargetType, got.TargetID)
	}
	if got.IPAddress != "203.0.113.7" {
		t.Errorf("expected IPAddress to round-trip, got %q", got.IPAddress)
	}
	if got.Metadata["from"] != "user" || got.Metadata["to"] != "admin" {
		t.Errorf("expected metadata to round-trip through JSON, got %+v", got.Metadata)
	}
	nested, ok := got.Metadata["nested"].(map[string]any)
	if !ok || nested["a"] != float64(1) {
		t.Errorf("expected nested metadata values to round-trip, got %+v", got.Metadata["nested"])
	}
}

// --- SsoIdentityRepository ---

func TestSsoIdentityRepository_LinkFindTouch(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, uuid.NewString(), "sso-user@example.com")
	now := time.Now().UTC().Truncate(time.Second)

	identity, err := domain.NewSsoIdentity(uuid.NewString(), user.ID, user.TenantID, domain.SsoProviderGitHub, "gh-12345", "sso-user@example.com", now)
	if err != nil {
		t.Fatalf("building sso identity: %v", err)
	}
	if err := repo.Link(ctx, identity); err != nil {
		t.Fatalf("link: %v", err)
	}

	got, err := repo.FindByProviderSubject(ctx, domain.SsoProviderGitHub, "gh-12345")
	if err != nil {
		t.Fatalf("find by provider subject: %v", err)
	}
	if got.UserID != user.ID {
		t.Errorf("expected UserID %q, got %q", user.ID, got.UserID)
	}

	if _, err := repo.FindByProviderSubject(ctx, domain.SsoProviderGitHub, "does-not-exist"); err == nil {
		t.Fatal("expected an error for an unknown (provider, external_subject)")
	}

	if err := repo.TouchLastLogin(ctx, identity.ID, now.Add(time.Hour)); err != nil {
		t.Fatalf("touch last login: %v", err)
	}
	got, err = repo.FindByProviderSubject(ctx, domain.SsoProviderGitHub, "gh-12345")
	if err != nil {
		t.Fatalf("re-find: %v", err)
	}
	if got.LastLoginAt == nil || !got.LastLoginAt.Equal(now.Add(time.Hour)) {
		t.Errorf("expected LastLoginAt to be updated, got %+v", got.LastLoginAt)
	}

	// A second Link for the same (provider, external_subject) must fail —
	// UNIQUE(provider, external_subject).
	other := createTestUser(t, repo, uuid.NewString(), "sso-user-2@example.com")
	dup, err := domain.NewSsoIdentity(uuid.NewString(), other.ID, other.TenantID, domain.SsoProviderGitHub, "gh-12345", "sso-user-2@example.com", now)
	if err != nil {
		t.Fatalf("building duplicate sso identity: %v", err)
	}
	if err := repo.Link(ctx, dup); err == nil {
		t.Fatal("expected linking a duplicate (provider, external_subject) to fail")
	}
}

// jsonEqual compares two JSON strings by decoded value, not byte content —
// see TestAccessPolicyRepository_VersioningAndListLatest's comment for why
// a byte-exact comparison is wrong for a value that passed through a
// MySQL JSON column.
func jsonEqual(t *testing.T, a, b string) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal([]byte(a), &av); err != nil {
		t.Fatalf("unmarshal %q: %v", a, err)
	}
	if err := json.Unmarshal([]byte(b), &bv); err != nil {
		t.Fatalf("unmarshal %q: %v", b, err)
	}
	return reflect.DeepEqual(av, bv)
}
