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

	a, err := domain.NewAutomation("a1", "11111111-1111-1111-1111-111111111111", "nightly-report", "FREQ=DAILY;INTERVAL=1", `{"step_type":"agent"}`, now, now)
	if err != nil {
		t.Fatalf("building automation: %v", err)
	}
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
}

func TestAutomationRunRepository_FindByRequestID_IsIdempotencyBackstop(t *testing.T) {
	automations, runs := setupRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	a, _ := domain.NewAutomation("a1", "11111111-1111-1111-1111-111111111111", "nightly-report", "FREQ=DAILY;INTERVAL=1", `{"step_type":"agent"}`, now, now)
	if err := automations.Create(ctx, a); err != nil {
		t.Fatalf("create automation: %v", err)
	}

	run, err := domain.NewPendingRun("r1", a.ID, a.TenantID, "req-1", domain.StepTypeAgent, a.StepConfigJSON, now)
	if err != nil {
		t.Fatalf("building run: %v", err)
	}
	if err := runs.Create(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	// A second Create with the same (tenant_id, request_id) must violate the
	// unique index — the DB-level idempotency backstop from
	// automation-service.md §8.
	dup, _ := domain.NewPendingRun("r2", a.ID, a.TenantID, "req-1", domain.StepTypeAgent, a.StepConfigJSON, now)
	if err := runs.Create(ctx, dup); err == nil {
		t.Fatal("expected a unique constraint violation for a duplicate (tenant_id, request_id)")
	}

	found, ok, err := runs.FindByRequestID(ctx, a.TenantID, a.ID, "req-1")
	if err != nil {
		t.Fatalf("find by request id: %v", err)
	}
	if !ok || found.ID != "r1" {
		t.Errorf("expected to find run r1, got found=%v ok=%v", found, ok)
	}
}
