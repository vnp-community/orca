//go:build integration

// Integration tests run against a real MySQL via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/mysql/...`. Mirrors
// internal/adapter/postgres/repository_test.go's test names/shape 1:1,
// plus the 2 tenant-isolation tests from TASK-BE-DB-003 mirrored for
// MySQL — CR-DB-002's acceptance criterion is that tenant isolation holds
// on a dialect with NO RLS equivalent at all, not just Postgres.
package mysql

import (
	"context"
	"database/sql"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/usage-service/internal/domain"
)

// tenantAUUID/tenantBUUID/tenantIsolationUserID mirror
// internal/adapter/postgres/repository_test.go's constants of the same
// purpose — MySQL's tenant_id/user_id columns are CHAR(36), which (unlike
// Postgres's native UUID type) don't validate UUID format, but using real
// UUID-shaped values keeps this test's data realistic and consistent with
// the Postgres mirror.
const (
	tenantAUUID           = "aaaaaaaa-0000-4000-8000-000000000001"
	tenantBUUID           = "bbbbbbbb-0000-4000-8000-000000000002"
	tenantIsolationUserID = "cccccccc-0000-4000-8000-000000000003"
)

func testOutboxEvent() domain.OutboxEvent {
	return domain.OutboxEvent{ID: uuid.NewString(), Subject: "test.subject", OccurredAt: time.Now(), PayloadJSON: []byte(`{}`)}
}

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	// testutil.StartMySQL returns "mysql://root:orca@tcp(host:port)/db" —
	// valid as-is for golang-migrate's mysql driver CLI (used below), but
	// go-sql-driver/mysql's database/sql driver expects its OWN DSN format
	// with no "mysql://" scheme (see TASK-BE-DB-006's toMySQLDriverDSN for
	// the general-purpose conversion this test doesn't need — a fixed-shape
	// test DSN only needs the prefix stripped). parseTime=true is required
	// for TIMESTAMP columns to scan into time.Time/sql.NullTime instead of
	// []byte.
	rawDSN := testutil.StartMySQL(t, "usage")
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

	return New(db)
}

func TestRepository_SaveSession_IsIdempotentOnRequestID(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	session, err := domain.NewUsageSession(
		"s1", tenantAUUID, tenantIsolationUserID, domain.ProviderClaude, "wt-1",
		100, 50, 0, 0, 0.05, time.Now(), time.Now(), "req-1",
	)
	if err != nil {
		t.Fatalf("building session: %v", err)
	}

	if err := repo.SaveSession(ctx, session, testOutboxEvent()); err != nil {
		t.Fatalf("first save: %v", err)
	}
	// Same RequestID, different ID — simulates a client retry after a
	// timed-out-but-actually-succeeded write. Must not double-count.
	session.ID = "s1-retry"
	if err := repo.SaveSession(ctx, session, testOutboxEvent()); err != nil {
		t.Fatalf("second save (retry): %v", err)
	}

	rollup, err := repo.GetDailyRollup(ctx, tenantAUUID, tenantIsolationUserID, domain.ProviderClaude, domain.DayKey(session.StartedAt))
	if err != nil {
		t.Fatalf("get daily rollup: %v", err)
	}
	if rollup.SessionCount != 1 {
		t.Errorf("expected idempotent save to result in SessionCount=1, got %d", rollup.SessionCount)
	}
}

func TestRepository_ListSessions_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	sessionA, _ := domain.NewUsageSession("s-a", tenantAUUID, tenantIsolationUserID, domain.ProviderClaude, "wt-1",
		100, 50, 0, 0, 0.05, time.Now(), time.Now(), "req-a")
	sessionB, _ := domain.NewUsageSession("s-b", tenantBUUID, tenantIsolationUserID, domain.ProviderClaude, "wt-1",
		200, 100, 0, 0, 0.10, time.Now(), time.Now(), "req-b")

	if err := repo.SaveSession(ctx, sessionA, testOutboxEvent()); err != nil {
		t.Fatalf("saving tenant-a session: %v", err)
	}
	if err := repo.SaveSession(ctx, sessionB, testOutboxEvent()); err != nil {
		t.Fatalf("saving tenant-b session: %v", err)
	}

	sessions, _, err := repo.ListSessions(ctx, tenantAUUID, "", "", 100)
	if err != nil {
		t.Fatalf("listing tenant-a sessions: %v", err)
	}
	for _, s := range sessions {
		if s.TenantID != tenantAUUID {
			t.Fatalf("ListSessions(tenant-a) leaked a row from tenant %q — application-layer scoping failed, MySQL has no RLS backstop at all (see BE-DB-SOL-001 §4, TASK-BE-DB-003)", s.TenantID)
		}
	}
	if len(sessions) != 1 || sessions[0].ID != "s-a" {
		t.Fatalf("expected exactly session s-a for tenant-a, got %+v", sessions)
	}
}

func TestRepository_GetDailyRollup_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	day := time.Now()

	sessionA, _ := domain.NewUsageSession("s-a2", tenantAUUID, tenantIsolationUserID, domain.ProviderClaude, "wt-1",
		100, 50, 0, 0, 1.00, day, day, "req-a2")
	sessionB, _ := domain.NewUsageSession("s-b2", tenantBUUID, tenantIsolationUserID, domain.ProviderClaude, "wt-1",
		999, 999, 0, 0, 99.00, day, day, "req-b2")
	if err := repo.SaveSession(ctx, sessionA, testOutboxEvent()); err != nil {
		t.Fatalf("saving tenant-a session: %v", err)
	}
	if err := repo.SaveSession(ctx, sessionB, testOutboxEvent()); err != nil {
		t.Fatalf("saving tenant-b session: %v", err)
	}

	rollup, err := repo.GetDailyRollup(ctx, tenantAUUID, tenantIsolationUserID, domain.ProviderClaude, day)
	if err != nil {
		t.Fatalf("getting tenant-a rollup: %v", err)
	}
	if rollup.TotalCostUSD != 1.00 {
		t.Fatalf("tenant-a rollup polluted by tenant-b data: got cost %v, want 1.00 — application-layer scoping failed", rollup.TotalCostUSD)
	}
}

func TestRepository_RecomputeDailyRollup_MatchesSessionSums(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	day := time.Now()

	s1, _ := domain.NewUsageSession("s1", tenantAUUID, tenantIsolationUserID, domain.ProviderClaude, "wt-1", 100, 50, 0, 0, 1.00, day, day, "req-1")
	s2, _ := domain.NewUsageSession("s2", tenantAUUID, tenantIsolationUserID, domain.ProviderClaude, "wt-1", 200, 75, 0, 0, 2.00, day, day, "req-2")
	if err := repo.SaveSession(ctx, s1, testOutboxEvent()); err != nil {
		t.Fatalf("saving s1: %v", err)
	}
	if err := repo.SaveSession(ctx, s2, testOutboxEvent()); err != nil {
		t.Fatalf("saving s2: %v", err)
	}

	if err := repo.RecomputeDailyRollup(ctx, tenantAUUID, tenantIsolationUserID, domain.ProviderClaude, day); err != nil {
		t.Fatalf("recompute: %v", err)
	}

	rollup, err := repo.GetDailyRollup(ctx, tenantAUUID, tenantIsolationUserID, domain.ProviderClaude, day)
	if err != nil {
		t.Fatalf("get daily rollup: %v", err)
	}
	if rollup.TotalInputTokens != 300 || rollup.TotalOutputTokens != 125 || rollup.SessionCount != 2 {
		t.Fatalf("recomputed rollup doesn't match session sums: %+v", rollup)
	}
	if rollup.TotalCostUSD != 3.00 {
		t.Fatalf("recomputed cost mismatch: got %v, want 3.00", rollup.TotalCostUSD)
	}
}

func TestRepository_FetchUnpublished_ReturnsOldestFirst(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	first := domain.OutboxEvent{ID: uuid.NewString(), Subject: "first", OccurredAt: time.Now(), PayloadJSON: []byte(`{}`)}
	s1, _ := domain.NewUsageSession("s1", tenantAUUID, tenantIsolationUserID, domain.ProviderClaude, "wt-1", 1, 1, 0, 0, 0.01, time.Now(), time.Now(), "req-1")
	if err := repo.SaveSession(ctx, s1, first); err != nil {
		t.Fatalf("saving s1: %v", err)
	}

	time.Sleep(10 * time.Millisecond) // ensure a distinguishable created_at ordering

	second := domain.OutboxEvent{ID: uuid.NewString(), Subject: "second", OccurredAt: time.Now(), PayloadJSON: []byte(`{}`)}
	s2, _ := domain.NewUsageSession("s2", tenantAUUID, tenantIsolationUserID, domain.ProviderClaude, "wt-1", 1, 1, 0, 0, 0.01, time.Now(), time.Now(), "req-2")
	if err := repo.SaveSession(ctx, s2, second); err != nil {
		t.Fatalf("saving s2: %v", err)
	}

	unpublished, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("fetch unpublished: %v", err)
	}
	if len(unpublished) != 2 {
		t.Fatalf("expected 2 unpublished events, got %d", len(unpublished))
	}
	if unpublished[0].Subject != "first" || unpublished[1].Subject != "second" {
		t.Fatalf("expected oldest-first order [first, second], got [%s, %s]", unpublished[0].Subject, unpublished[1].Subject)
	}
}

func TestRepository_MarkPublished_EmptyIDsIsNoop(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	if err := repo.MarkPublished(ctx, nil); err != nil {
		t.Fatalf("MarkPublished(nil) should be a no-op, got error: %v", err)
	}
	if err := repo.MarkPublished(ctx, []string{}); err != nil {
		t.Fatalf("MarkPublished([]string{}) should be a no-op, got error: %v", err)
	}
}

// TestRepository_Outbox_EnqueueFetchMarkPublished exercises the Epic G
// transactional-outbox round trip, mirroring
// internal/adapter/postgres/repository_test.go's test of the same name.
func TestRepository_Outbox_EnqueueFetchMarkPublished(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	session, err := domain.NewUsageSession(
		"s1", tenantAUUID, tenantIsolationUserID, domain.ProviderClaude, "wt-1",
		100, 50, 0, 0, 0.05, time.Now(), time.Now(), "req-1",
	)
	if err != nil {
		t.Fatalf("building session: %v", err)
	}
	event := domain.OutboxEvent{ID: uuid.NewString(), Subject: "orca.usage.session.recorded", OccurredAt: time.Now(), PayloadJSON: []byte(`{"session_id":"s1"}`)}

	if err := repo.SaveSession(ctx, session, event); err != nil {
		t.Fatalf("save session: %v", err)
	}

	unpublished, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("fetch unpublished: %v", err)
	}
	if len(unpublished) != 1 || unpublished[0].ID != event.ID || unpublished[0].Subject != event.Subject {
		t.Fatalf("expected exactly the just-enqueued event, got %+v", unpublished)
	}

	if err := repo.MarkPublished(ctx, []string{event.ID}); err != nil {
		t.Fatalf("mark published: %v", err)
	}

	stillUnpublished, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("fetch unpublished after mark: %v", err)
	}
	if len(stillUnpublished) != 0 {
		t.Errorf("expected no unpublished events after MarkPublished, got %+v", stillUnpublished)
	}
}
