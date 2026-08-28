//go:build integration

// Integration tests run against a real Postgres via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/postgres/...`.
package postgres

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := testutil.StartPostgres(t, "auth")

	migrationsPath, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	// Uses the golang-migrate CLI directly rather than importing the
	// library, matching usage-service's reference integration test.
	cmd := exec.Command("migrate", "-path", migrationsPath, "-database", dsn, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running migrations: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting to postgres: %v", err)
	}
	t.Cleanup(pool.Close)

	return New(pool)
}

func TestUserRepository_UpdateUser_PartialUpdatePreservesUntouchedFields(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenantID := uuid.NewString()
	user, err := domain.NewUser(uuid.NewString(), tenantID, "update-user@example.com", "Original Name", domain.RoleUser, true, time.Now())
	if err != nil {
		t.Fatalf("building user: %v", err)
	}
	if _, err := repo.CreateUser(ctx, user, "hash1"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	newEmail := "updated-email@example.com"
	updated, err := repo.UpdateUser(ctx, user.ID, &newEmail, nil, nil)
	if err != nil {
		t.Fatalf("update user: %v", err)
	}
	if updated.Email != newEmail {
		t.Errorf("expected email to be updated, got %q", updated.Email)
	}
	if updated.Name != "Original Name" {
		t.Errorf("expected name to be preserved via COALESCE, got %q", updated.Name)
	}
	if updated.Role != domain.RoleUser {
		t.Errorf("expected role to be preserved via COALESCE, got %q", updated.Role)
	}

	// Re-read to confirm the partial update actually persisted, not just
	// the RETURNING clause's in-memory value.
	reread, err := repo.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("get user by id: %v", err)
	}
	if reread.Email != newEmail || reread.Name != "Original Name" || reread.Role != domain.RoleUser {
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

func TestRepository_CreateUser_RejectsDuplicateEmailInTenant(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenantID := uuid.NewString()
	u1, err := domain.NewUser(uuid.NewString(), tenantID, "dup@example.com", "First", domain.RoleUser, true, time.Now())
	if err != nil {
		t.Fatalf("building user: %v", err)
	}
	if _, err := repo.CreateUser(ctx, u1, "hash1"); err != nil {
		t.Fatalf("first create: %v", err)
	}

	u2, err := domain.NewUser(uuid.NewString(), tenantID, "dup@example.com", "Second", domain.RoleUser, true, time.Now())
	if err != nil {
		t.Fatalf("building user: %v", err)
	}
	if _, err := repo.CreateUser(ctx, u2, "hash2"); err == nil {
		t.Fatal("expected an error for a duplicate (tenant_id, email)")
	}
}

func TestRepository_SessionLifecycle(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenantID := uuid.NewString()
	user, err := domain.NewUser(uuid.NewString(), tenantID, "session-user@example.com", "User", domain.RoleUser, true, time.Now())
	if err != nil {
		t.Fatalf("building user: %v", err)
	}
	if _, err := repo.CreateUser(ctx, user, "hash1"); err != nil {
		t.Fatalf("create user: %v", err)
	}

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

func TestSessionRepository_RoundTripsClientInfo(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenantID := uuid.NewString()
	user, err := domain.NewUser(uuid.NewString(), tenantID, "client-info-user@example.com", "User", domain.RoleUser, true, time.Now())
	if err != nil {
		t.Fatalf("building user: %v", err)
	}
	if _, err := repo.CreateUser(ctx, user, "hash1"); err != nil {
		t.Fatalf("create user: %v", err)
	}

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
	if got.LastSeenAt != nil {
		t.Error("expected LastSeenAt to be nil until first touch")
	}

	// A session created with no IP/UserAgent round-trips as empty strings,
	// not the literal string "<nil>" or a scan error against INET.
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

func TestSessionRepository_TouchLastSeen(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenantID := uuid.NewString()
	user, err := domain.NewUser(uuid.NewString(), tenantID, "touch-user@example.com", "User", domain.RoleUser, true, time.Now())
	if err != nil {
		t.Fatalf("building user: %v", err)
	}
	if _, err := repo.CreateUser(ctx, user, "hash1"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	session, err := domain.NewSession(domain.HashSessionToken("raw-token-touch"), user.ID, tenantID, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("building session: %v", err)
	}
	if err := repo.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	touchedAt := now.Add(5 * time.Minute)
	if err := repo.TouchLastSeen(ctx, session.TokenHash, touchedAt); err != nil {
		t.Fatalf("touch last seen: %v", err)
	}
	got, err := repo.GetSessionByTokenHash(ctx, session.TokenHash)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.LastSeenAt == nil || !got.LastSeenAt.Equal(touchedAt) {
		t.Errorf("expected LastSeenAt = %v, got %v", touchedAt, got.LastSeenAt)
	}

	// A nonexistent token hash is a no-op, not an error.
	if err := repo.TouchLastSeen(ctx, "does-not-exist", touchedAt); err != nil {
		t.Errorf("expected touching an unknown token hash to be a no-op, got %v", err)
	}
}

func TestSessionRepository_DeleteExpiredBefore(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenantID := uuid.NewString()
	user, err := domain.NewUser(uuid.NewString(), tenantID, "reap-user@example.com", "User", domain.RoleUser, true, time.Now())
	if err != nil {
		t.Fatalf("building user: %v", err)
	}
	if _, err := repo.CreateUser(ctx, user, "hash1"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)

	// Long expired — should be reaped.
	expiredSession, err := domain.NewSession(domain.HashSessionToken("raw-token-expired"), user.ID, tenantID, now.Add(-48*time.Hour), now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("building expired session: %v", err)
	}
	if err := repo.CreateSession(ctx, expiredSession); err != nil {
		t.Fatalf("create expired session: %v", err)
	}

	// Still active — must survive the reap.
	activeSession, err := domain.NewSession(domain.HashSessionToken("raw-token-active"), user.ID, tenantID, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("building active session: %v", err)
	}
	if err := repo.CreateSession(ctx, activeSession); err != nil {
		t.Fatalf("create active session: %v", err)
	}

	cutoff := now.Add(-time.Hour)
	n, err := repo.DeleteExpiredBefore(ctx, cutoff)
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

func TestSessionRepository_ListForTenant(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenant1 := uuid.NewString()
	tenant2 := uuid.NewString()

	user1, err := domain.NewUser(uuid.NewString(), tenant1, "tenant1-user@example.com", "User1", domain.RoleUser, true, time.Now())
	if err != nil {
		t.Fatalf("building user1: %v", err)
	}
	if _, err := repo.CreateUser(ctx, user1, "hash1"); err != nil {
		t.Fatalf("create user1: %v", err)
	}
	user2, err := domain.NewUser(uuid.NewString(), tenant2, "tenant2-user@example.com", "User2", domain.RoleUser, true, time.Now())
	if err != nil {
		t.Fatalf("building user2: %v", err)
	}
	if _, err := repo.CreateUser(ctx, user2, "hash2"); err != nil {
		t.Fatalf("create user2: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	session1, err := domain.NewSession(domain.HashSessionToken("raw-token-t1"), user1.ID, tenant1, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("building session1: %v", err)
	}
	if err := repo.CreateSession(ctx, session1); err != nil {
		t.Fatalf("create session1: %v", err)
	}
	session2, err := domain.NewSession(domain.HashSessionToken("raw-token-t2"), user2.ID, tenant2, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("building session2: %v", err)
	}
	if err := repo.CreateSession(ctx, session2); err != nil {
		t.Fatalf("create session2: %v", err)
	}

	rows, next, err := repo.ListForTenant(ctx, tenant1, "", 50)
	if err != nil {
		t.Fatalf("list for tenant: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 session for tenant1, got %d: %+v", len(rows), rows)
	}
	if rows[0].Session.TenantID != tenant1 {
		t.Errorf("expected only tenant1's session, got %+v", rows[0])
	}
	if rows[0].UserEmail != "tenant1-user@example.com" {
		t.Errorf("expected joined user email, got %q", rows[0].UserEmail)
	}
	if next != "" {
		t.Errorf("expected no next page token when under page size, got %q", next)
	}

	// Pagination: page size 1 with only 1 matching row should NOT return a
	// next-page token equal to the last row seen when there's nothing more.
	rows, next, err = repo.ListForTenant(ctx, tenant1, "", 1)
	if err != nil {
		t.Fatalf("list for tenant page 1: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if next != rows[0].Session.TokenHash {
		t.Errorf("expected next page token = last row's token hash when page is full, got %q", next)
	}
}

func TestRepository_AuditLog_AppendAndQueryFiltersByTenant(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenant1 := uuid.NewString()
	tenant2 := uuid.NewString()

	e1, _ := domain.NewAuditEntry(uuid.NewString(), tenant1, uuid.NewString(), "user.login", "user", "target-1", nil, "", time.Now())
	e2, _ := domain.NewAuditEntry(uuid.NewString(), tenant2, uuid.NewString(), "user.login", "user", "target-2", nil, "", time.Now())
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

	entry, err := domain.NewAuditEntry(uuid.NewString(), tenantID, actorID, "user.role_updated", "user", "u2",
		map[string]any{"from": "user", "to": "admin", "nested": map[string]any{"a": float64(1)}}, "203.0.113.7", now)
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
		t.Errorf("expected IPAddress to round-trip without a /32 suffix, got %q", got.IPAddress)
	}
	if got.Metadata["from"] != "user" || got.Metadata["to"] != "admin" {
		t.Errorf("expected metadata to round-trip through JSONB, got %+v", got.Metadata)
	}
	nested, ok := got.Metadata["nested"].(map[string]any)
	if !ok || nested["a"] != float64(1) {
		t.Errorf("expected nested metadata values to round-trip, got %+v", got.Metadata["nested"])
	}
}

func TestAuditRepository_Query_FiltersByActionActorIDAndTo(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenantID := uuid.NewString()
	actor1 := uuid.NewString()
	actor2 := uuid.NewString()
	now := time.Now().UTC().Truncate(time.Second)

	entries := []struct {
		action  string
		actorID string
		when    time.Time
	}{
		{"user.created", actor1, now},
		{"user.deactivated", actor1, now.Add(time.Minute)},
		{"user.created", actor2, now.Add(2 * time.Minute)},
	}
	for _, e := range entries {
		entry, err := domain.NewAuditEntry(uuid.NewString(), tenantID, e.actorID, e.action, "user", "target", nil, "", e.when)
		if err != nil {
			t.Fatalf("building entry: %v", err)
		}
		if err := repo.Append(ctx, entry); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	// Filter by action alone.
	byAction, _, err := repo.Query(ctx, usecase.AuditQueryFilter{TenantID: tenantID, Action: "user.created"}, "", 50)
	if err != nil {
		t.Fatalf("query by action: %v", err)
	}
	if len(byAction) != 2 {
		t.Errorf("expected 2 user.created entries, got %d", len(byAction))
	}

	// Filter by actor_id alone.
	byActor, _, err := repo.Query(ctx, usecase.AuditQueryFilter{TenantID: tenantID, ActorID: actor1}, "", 50)
	if err != nil {
		t.Fatalf("query by actor_id: %v", err)
	}
	if len(byActor) != 2 {
		t.Errorf("expected 2 entries for actor1, got %d", len(byActor))
	}

	// Combined action + actor_id.
	combined, _, err := repo.Query(ctx, usecase.AuditQueryFilter{TenantID: tenantID, Action: "user.created", ActorID: actor1}, "", 50)
	if err != nil {
		t.Fatalf("query combined: %v", err)
	}
	if len(combined) != 1 {
		t.Errorf("expected 1 entry matching both filters, got %d", len(combined))
	}

	// `to` upper bound excludes the later two entries.
	byTo, _, err := repo.Query(ctx, usecase.AuditQueryFilter{TenantID: tenantID, To: now.Add(30 * time.Second)}, "", 50)
	if err != nil {
		t.Fatalf("query by to: %v", err)
	}
	if len(byTo) != 1 {
		t.Errorf("expected 1 entry before the `to` bound, got %d", len(byTo))
	}
}
