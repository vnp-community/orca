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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

func setupRepositories(t *testing.T) (*AutomationRepository, *AutomationRunRepository) {
	t.Helper()
	dsn := testutil.StartPostgres(t, "automation")

	migrationsPath, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	// Uses the golang-migrate CLI directly rather than importing the
	// library, keeping this test's dependency footprint minimal — matches
	// usage-service's reference pattern.
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

	return NewAutomationRepository(pool), NewAutomationRunRepository(pool)
}

func TestAutomationRepository_CreateAndGet(t *testing.T) {
	automations, _ := setupRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	a, err := domain.NewAutomation("00000000-0000-0000-0000-000000000001", "11111111-1111-1111-1111-111111111111", "nightly-report", "FREQ=DAILY;INTERVAL=1", domain.StepTypeAgent, `{"prompt":"summarize"}`, now, "UTC", true, now)
	if err != nil {
		t.Fatalf("building automation: %v", err)
	}
	a.NextRunAt = now
	if err := automations.Create(ctx, a); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := automations.Get(ctx, a.TenantID, a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != a.Name || got.RRule != a.RRule {
		t.Errorf("expected round-tripped automation to match, got %+v", got)
	}
	if got.StepType != domain.StepTypeAgent {
		t.Errorf("expected round-tripped StepType=agent, got %v", got.StepType)
	}
	if !got.Enabled {
		t.Error("expected round-tripped Enabled=true")
	}
	if got.Timezone != "UTC" {
		t.Errorf("expected round-tripped Timezone=UTC, got %q", got.Timezone)
	}
	if !got.NextRunAt.Equal(now) {
		t.Errorf("expected round-tripped NextRunAt=%v, got %v", now, got.NextRunAt)
	}
}

func TestAutomationRepository_ClaimDue_LocksAndAdvancesNextRunAt(t *testing.T) {
	automations, _ := setupRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	due := mustNewAutomation(t, "00000000-0000-0000-0000-0000000000d1", "11111111-1111-1111-1111-111111111111", now.Add(-time.Minute))
	notYetDue := mustNewAutomation(t, "00000000-0000-0000-0000-0000000000f1", "11111111-1111-1111-1111-111111111111", now.Add(time.Hour))
	disabled := mustNewAutomation(t, "00000000-0000-0000-0000-0000000000d2", "11111111-1111-1111-1111-111111111111", now.Add(-time.Minute))
	disabled.Enabled = false

	for _, a := range []domain.Automation{due, notYetDue, disabled} {
		if err := automations.Create(ctx, a); err != nil {
			t.Fatalf("create automation %s: %v", a.ID, err)
		}
	}

	batch, err := automations.ClaimDue(ctx, now, 10)
	if err != nil {
		t.Fatalf("claim due: %v", err)
	}
	claimed := batch.Automations()
	if len(claimed) != 1 || claimed[0].ID != "00000000-0000-0000-0000-0000000000d1" {
		t.Fatalf("expected only the due, enabled automation to be claimed, got %+v", claimed)
	}

	next := now.Add(24 * time.Hour)
	if err := batch.Advance(ctx, "00000000-0000-0000-0000-0000000000d1", next, true); err != nil {
		t.Fatalf("advance: %v", err)
	}
	if err := batch.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	got, err := automations.Get(ctx, due.TenantID, "00000000-0000-0000-0000-0000000000d1")
	if err != nil {
		t.Fatalf("get after claim: %v", err)
	}
	if !got.NextRunAt.Equal(next) {
		t.Errorf("expected NextRunAt advanced to %v, got %v", next, got.NextRunAt)
	}
}

func TestAutomationRepository_ClaimDue_SkipsRowsLockedByAnotherClaim(t *testing.T) {
	automations, _ := setupRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	due := mustNewAutomation(t, "00000000-0000-0000-0000-0000000000d1", "11111111-1111-1111-1111-111111111111", now.Add(-time.Minute))
	if err := automations.Create(ctx, due); err != nil {
		t.Fatalf("create automation: %v", err)
	}

	// Simulate a second replica's concurrent tick: the first ClaimDue's
	// transaction is still open (not committed/rolled back yet), holding
	// the row lock — a second ClaimDue call must SKIP LOCKED past it
	// instead of blocking or double-claiming, per automation-service.md §7.
	firstBatch, err := automations.ClaimDue(ctx, now, 10)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if len(firstBatch.Automations()) != 1 {
		t.Fatalf("expected the first claim to lock the due row, got %d", len(firstBatch.Automations()))
	}

	secondBatch, err := automations.ClaimDue(ctx, now, 10)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if len(secondBatch.Automations()) != 0 {
		t.Errorf("expected the second concurrent claim to skip the locked row, got %d", len(secondBatch.Automations()))
	}
	if err := secondBatch.Rollback(ctx); err != nil {
		t.Fatalf("rollback second batch: %v", err)
	}
	if err := firstBatch.Rollback(ctx); err != nil {
		t.Fatalf("rollback first batch: %v", err)
	}
}

func mustNewAutomation(t *testing.T, id, tenantID string, nextRunAt time.Time) domain.Automation {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	a, err := domain.NewAutomation(id, tenantID, "nightly-report", "FREQ=DAILY;INTERVAL=1", domain.StepTypeAgent, `{"prompt":"summarize"}`, now, "UTC", true, now)
	if err != nil {
		t.Fatalf("building automation: %v", err)
	}
	a.NextRunAt = nextRunAt
	return a
}

func TestAutomationRunRepository_FindByRequestID_IsIdempotencyBackstop(t *testing.T) {
	automations, runs := setupRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	a, _ := domain.NewAutomation("00000000-0000-0000-0000-000000000001", "11111111-1111-1111-1111-111111111111", "nightly-report", "FREQ=DAILY;INTERVAL=1", domain.StepTypeAgent, `{"prompt":"summarize"}`, now, "UTC", true, now)
	if err := automations.Create(ctx, a); err != nil {
		t.Fatalf("create automation: %v", err)
	}

	run, err := domain.NewPendingRun("00000000-0000-0000-0000-000000000101", a.ID, a.TenantID, "req-1", domain.StepTypeAgent, domain.RunTriggerManual, a.StepConfigJSON, now)
	if err != nil {
		t.Fatalf("building run: %v", err)
	}
	if err := runs.Create(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	// A second Create with the same (tenant_id, request_id) must violate the
	// unique index — the DB-level idempotency backstop from
	// automation-service.md §8.
	dup, _ := domain.NewPendingRun("00000000-0000-0000-0000-000000000102", a.ID, a.TenantID, "req-1", domain.StepTypeAgent, domain.RunTriggerManual, a.StepConfigJSON, now)
	if err := runs.Create(ctx, dup); err == nil {
		t.Fatal("expected a unique constraint violation for a duplicate (tenant_id, request_id)")
	}

	found, ok, err := runs.FindByRequestID(ctx, a.TenantID, a.ID, "req-1")
	if err != nil {
		t.Fatalf("find by request id: %v", err)
	}
	if !ok || found.ID != "00000000-0000-0000-0000-000000000101" {
		t.Errorf("expected to find run r1, got found=%v ok=%v", found, ok)
	}
}
