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
	"github.com/stablyai/orca-go/services/usage-service/internal/domain"
)

// testOutboxEvent builds a minimal, valid domain.OutboxEvent for tests that
// don't care about outbox content — just that SaveSession's transactional
// enqueue doesn't break the rest of the write.
func testOutboxEvent() domain.OutboxEvent {
	return domain.OutboxEvent{ID: uuid.NewString(), Subject: "test.subject", OccurredAt: time.Now(), PayloadJSON: []byte(`{}`)}
}

// testTenant1UUID/testTenant2UUID/testUser1UUID stand in for the
// human-readable "tenant-1"/"tenant-2"/"user-1" IDs these tests used
// before TASK-BE-DB-003/007 — real Postgres rejects those outright since
// usage.sessions/daily_rollups type tenant_id/user_id as native UUID, not
// TEXT (migrations/postgres/0001_init.up.sql). Fixed here alongside the
// ListSessions $2 type-ambiguity bug (see repository.go) since both were
// blocking the same tests from ever actually running against real
// Postgres — testcontainers never caught either until TASK-BE-DB-003.
const (
	testTenant1UUID = "11111111-0000-4000-8000-000000000001"
	testTenant2UUID = "22222222-0000-4000-8000-000000000002"
	testUser1UUID   = "33333333-0000-4000-8000-000000000003"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := testutil.StartPostgres(t, "usage")

	migrationsPath, err := filepath.Abs("../../../migrations/postgres")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	// Uses the golang-migrate CLI directly rather than importing the
	// library, keeping this test's dependency footprint minimal — swap for
	// the library-based runner once the shared migration-runner helper
	// (referenced in architecture/05-data-architecture.md) exists in common/.
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

func TestRepository_SaveSession_IsIdempotentOnRequestID(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	session, err := domain.NewUsageSession(
		"s1", testTenant1UUID, testUser1UUID, domain.ProviderClaude, "wt-1",
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

	rollup, err := repo.GetDailyRollup(ctx, testTenant1UUID, testUser1UUID, domain.ProviderClaude, domain.DayKey(session.StartedAt))
	if err != nil {
		t.Fatalf("get daily rollup: %v", err)
	}
	if rollup.SessionCount != 1 {
		t.Errorf("expected idempotent save to result in SessionCount=1, got %d", rollup.SessionCount)
	}
}

func TestRepository_ListSessions_FiltersByTenant(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	s1, _ := domain.NewUsageSession("s1", testTenant1UUID, testUser1UUID, domain.ProviderClaude, "wt-1", 10, 5, 0, 0, 0.01, time.Now(), time.Time{}, "req-1")
	s2, _ := domain.NewUsageSession("s2", testTenant2UUID, testUser1UUID, domain.ProviderClaude, "wt-1", 10, 5, 0, 0, 0.01, time.Now(), time.Time{}, "req-2")
	if err := repo.SaveSession(ctx, s1, testOutboxEvent()); err != nil {
		t.Fatalf("saving tenant-1 session: %v", err)
	}
	if err := repo.SaveSession(ctx, s2, testOutboxEvent()); err != nil {
		t.Fatalf("saving tenant-2 session: %v", err)
	}

	sessions, _, err := repo.ListSessions(ctx, testTenant1UUID, "", "", 50)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0].TenantID != testTenant1UUID {
		t.Errorf("expected only tenant-1's session, got %+v", sessions)
	}
}

// TestRepository_ListSessions_DoesNotLeakAcrossTenants and
// TestRepository_GetDailyRollup_DoesNotLeakAcrossTenants confirm tenant
// isolation holds from application-layer scoping alone — see
// specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-001.md §4:
// no code here ever calls `SET LOCAL app.tenant_id`, so the migration's RLS
// policy never actually activates against this pool's connections (the
// pool connects as the table owner, which bypasses RLS) — it was never a
// real backstop in this environment. These tests prove the WHERE
// tenant_id = $N clauses in ListSessions/GetDailyRollup are sufficient on
// their own, which is what TASK-BE-DB-005's MySQL adapter (no RLS
// equivalent at all) must also satisfy.
// tenantAUUID/tenantBUUID/tenantIsolationUserUUID are valid-format UUID
// strings — required because the Postgres migration types
// usage.sessions/daily_rollups' tenant_id/user_id columns as native UUID,
// not TEXT (see migrations/postgres/0001_init.up.sql). The task doc's
// original draft used human-readable IDs like "tenant-a"/"user-1", which
// real Postgres rejects outright (`invalid input syntax for type uuid`) —
// adjusted here per the "code thật khác với mô tả" rule. The other
// pre-existing tests in this file (written before this task) had the same
// problem — fixed alongside ListSessions's $2 type-ambiguity bug once
// both were confirmed to be the only things standing between this suite
// and actually passing against real Postgres.
const (
	tenantAUUID           = "aaaaaaaa-0000-4000-8000-000000000001"
	tenantBUUID           = "bbbbbbbb-0000-4000-8000-000000000002"
	tenantIsolationUserID = "cccccccc-0000-4000-8000-000000000003"
)

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
			t.Fatalf("ListSessions(tenant-a) leaked a row from tenant %q — application-layer scoping failed, RLS is not a backstop here (see BE-DB-SOL-001 §4)", s.TenantID)
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

// TestRepository_Outbox_EnqueueFetchMarkPublished exercises the Epic G
// transactional-outbox round trip: SaveSession enqueues a row in the same
// tx as the session write, FetchUnpublished sees it, and MarkPublished
// removes it from future fetches — the exact cycle common/outbox.Relay
// drives in production.
func TestRepository_Outbox_EnqueueFetchMarkPublished(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	session, err := domain.NewUsageSession(
		"s1", testTenant1UUID, testUser1UUID, domain.ProviderClaude, "wt-1",
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
