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

func TestAutomationRepository_List_ScopesToTenant(t *testing.T) {
	automations, _ := setupRepositories(t)
	ctx := context.Background()

	tenantA := "11111111-1111-1111-1111-111111111111"
	tenantB := "22222222-2222-2222-2222-222222222222"
	mustCreateAutomation(t, automations, "00000000-0000-0000-0000-0000000000a1", tenantA)
	mustCreateAutomation(t, automations, "00000000-0000-0000-0000-0000000000a2", tenantA)
	mustCreateAutomation(t, automations, "00000000-0000-0000-0000-0000000000b1", tenantB)

	got, _, err := automations.List(ctx, tenantA, "", 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 automations for tenant A, got %d", len(got))
	}
	for _, a := range got {
		if a.TenantID != tenantA {
			t.Errorf("expected only tenant A rows, got tenant_id=%q", a.TenantID)
		}
	}
}

func TestAutomationRepository_List_PaginatesWithoutDuplicatesOrGaps(t *testing.T) {
	automations, _ := setupRepositories(t)
	ctx := context.Background()
	tenantID := "11111111-1111-1111-1111-111111111111"

	ids := []string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
		"00000000-0000-0000-0000-000000000003",
		"00000000-0000-0000-0000-000000000004",
		"00000000-0000-0000-0000-000000000005",
	}
	for _, id := range ids {
		mustCreateAutomation(t, automations, id, tenantID)
	}

	seen := map[string]bool{}
	pageToken := ""
	for i := 0; i < 10; i++ { // bounded loop guards against an infinite-pagination bug
		page, next, err := automations.List(ctx, tenantID, pageToken, 2)
		if err != nil {
			t.Fatalf("list page: %v", err)
		}
		for _, a := range page {
			if seen[a.ID] {
				t.Fatalf("duplicate id %s returned across pages", a.ID)
			}
			seen[a.ID] = true
		}
		if next == "" {
			break
		}
		pageToken = next
	}
	if len(seen) != len(ids) {
		t.Fatalf("expected all %d automations covered across pages, got %d", len(ids), len(seen))
	}
}

func TestAutomationRepository_Update_PersistsFieldsAndFailsForWrongTenant(t *testing.T) {
	automations, _ := setupRepositories(t)
	ctx := context.Background()
	tenantA := "11111111-1111-1111-1111-111111111111"
	tenantB := "22222222-2222-2222-2222-222222222222"

	a := mustCreateAutomation(t, automations, "00000000-0000-0000-0000-0000000000c1", tenantA)
	a.Name = "renamed"
	a.Enabled = false
	if err := automations.Update(ctx, tenantA, a); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := automations.Get(ctx, tenantA, a.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Name != "renamed" || got.Enabled {
		t.Errorf("expected the update to persist, got %+v", got)
	}

	// Updating tenant A's row while scoped to tenant B must affect 0 rows
	// and surface as an error — the tenant-isolation guarantee.
	a.Name = "should-not-persist"
	if err := automations.Update(ctx, tenantB, a); err == nil {
		t.Error("expected an error when updating another tenant's automation")
	}
}

func TestAutomationRepository_Delete_CascadesToRunsAndFailsForWrongTenant(t *testing.T) {
	automations, runs := setupRepositories(t)
	ctx := context.Background()
	tenantA := "11111111-1111-1111-1111-111111111111"
	tenantB := "22222222-2222-2222-2222-222222222222"

	a := mustCreateAutomation(t, automations, "00000000-0000-0000-0000-0000000000d2", tenantA)
	now := time.Now().UTC().Truncate(time.Second)
	run, err := domain.NewPendingRun("00000000-0000-0000-0000-000000000d1a", a.ID, tenantA, "req-cascade", domain.StepTypeAgent, domain.RunTriggerManual, a.StepConfigJSON, now)
	if err != nil {
		t.Fatalf("building run: %v", err)
	}
	if err := runs.Create(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	// Deleting tenant A's automation while scoped to tenant B must fail
	// (0 rows affected) and leave the row + its run intact.
	if err := automations.Delete(ctx, tenantB, a.ID); err == nil {
		t.Error("expected an error when deleting another tenant's automation")
	}

	if err := automations.Delete(ctx, tenantA, a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := automations.Get(ctx, tenantA, a.ID); err == nil {
		t.Error("expected the automation to be gone after delete")
	}

	// ON DELETE CASCADE (migrations/0001_init.up.sql) must have removed the
	// automation_runs row too — checked with a direct SELECT against the
	// table, not through the repository API.
	var count int
	if err := automations.pool.QueryRow(ctx, `SELECT count(*) FROM automation.automation_runs WHERE id = $1`, run.ID).Scan(&count); err != nil {
		t.Fatalf("querying automation_runs directly: %v", err)
	}
	if count != 0 {
		t.Errorf("expected the run row to be cascade-deleted, found %d matching rows", count)
	}
}

func mustCreateAutomation(t *testing.T, repo *AutomationRepository, id, tenantID string) domain.Automation {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	a, err := domain.NewAutomation(id, tenantID, "nightly-report", "FREQ=DAILY;INTERVAL=1", domain.StepTypeAgent, `{"prompt":"summarize"}`, now, "UTC", true, now)
	if err != nil {
		t.Fatalf("building automation: %v", err)
	}
	if err := repo.Create(context.Background(), a); err != nil {
		t.Fatalf("create automation %s: %v", id, err)
	}
	return a
}
