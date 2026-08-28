//go:build integration

// Integration tests run against a real Postgres via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag, see repository_test.go's package doc comment for
// the full rationale and how to invoke these explicitly.
package postgres

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

const (
	testScrollbackWorktree1 = "88888888-8888-8888-8888-888888888888"
	testScrollbackWorktree2 = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
)

func setupTerminalScrollbackSnapshotStore(t *testing.T) *TerminalScrollbackSnapshotStore {
	t.Helper()
	dsn := testutil.StartPostgres(t, "infra")

	migrationsPath, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
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

	return NewTerminalScrollbackSnapshotStore(pool)
}

func TestTerminalScrollbackSnapshot_SumUncompressedBytes_ExcludesReplacedPane(t *testing.T) {
	store := setupTerminalScrollbackSnapshotStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := store.Upsert(ctx, domain.TerminalScrollbackSnapshot{
		TenantID: testTenant1, WorktreeID: testScrollbackWorktree1, PaneKey: "pane-a",
		Cols: 80, Rows: 24, DataGzip: []byte("gzA"), UncompressedBytes: 1000, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert pane-a: %v", err)
	}
	if err := store.Upsert(ctx, domain.TerminalScrollbackSnapshot{
		TenantID: testTenant1, WorktreeID: testScrollbackWorktree1, PaneKey: "pane-b",
		Cols: 80, Rows: 24, DataGzip: []byte("gzB"), UncompressedBytes: 2000, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert pane-b: %v", err)
	}

	// Sum excluding pane-a should only count pane-b's 2000 bytes — a second
	// save to pane-a itself must never double-count toward BR-TM-10's cap.
	total, err := store.SumUncompressedBytes(ctx, testTenant1, testScrollbackWorktree1, "pane-a")
	if err != nil {
		t.Fatalf("sum: %v", err)
	}
	if total != 2000 {
		t.Errorf("expected sum excluding pane-a to be 2000, got %d", total)
	}

	// Re-upserting pane-a with a larger payload must not change pane-b's
	// contribution.
	if err := store.Upsert(ctx, domain.TerminalScrollbackSnapshot{
		TenantID: testTenant1, WorktreeID: testScrollbackWorktree1, PaneKey: "pane-a",
		Cols: 80, Rows: 24, DataGzip: []byte("gzA2"), UncompressedBytes: 5000, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("re-upsert pane-a: %v", err)
	}
	total, err = store.SumUncompressedBytes(ctx, testTenant1, testScrollbackWorktree1, "pane-a")
	if err != nil {
		t.Fatalf("sum after re-upsert: %v", err)
	}
	if total != 2000 {
		t.Errorf("expected sum excluding pane-a to remain 2000 after re-upsert, got %d", total)
	}
}

func TestTerminalScrollbackSnapshot_Get_ScopedByTenant(t *testing.T) {
	store := setupTerminalScrollbackSnapshotStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := store.Upsert(ctx, domain.TerminalScrollbackSnapshot{
		TenantID: testTenant1, WorktreeID: testScrollbackWorktree1, PaneKey: "pane-a",
		Cols: 80, Rows: 24, DataGzip: []byte("gz"), UncompressedBytes: 10, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// A different tenant's Get for the same (worktree, pane) key must return
	// found=false, not tenant-1's row — the RLS policy (migrations/0007) and
	// this query's explicit tenant_id join both enforce this.
	found, _, err := store.Get(ctx, testTenant2, testScrollbackWorktree1, "pane-a")
	if err != nil {
		t.Fatalf("cross-tenant get: %v", err)
	}
	if found {
		t.Error("expected found=false for a different tenant's Get, got true")
	}

	found, snap, err := store.Get(ctx, testTenant1, testScrollbackWorktree1, "pane-a")
	if err != nil {
		t.Fatalf("same-tenant get: %v", err)
	}
	if !found {
		t.Fatal("expected found=true for the owning tenant's Get")
	}
	if snap.TenantID != testTenant1 {
		t.Errorf("expected TenantID %q, got %q", testTenant1, snap.TenantID)
	}
}

func TestTerminalScrollbackSnapshot_DeleteExpired_RespectsCutoff(t *testing.T) {
	store := setupTerminalScrollbackSnapshotStore(t)
	ctx := context.Background()

	old := time.Now().UTC().Add(-31 * 24 * time.Hour)
	fresh := time.Now().UTC().Add(-1 * time.Hour)

	if err := store.Upsert(ctx, domain.TerminalScrollbackSnapshot{
		TenantID: testTenant1, WorktreeID: testScrollbackWorktree1, PaneKey: "pane-old",
		Cols: 80, Rows: 24, DataGzip: []byte("gz"), UncompressedBytes: 10, UpdatedAt: old,
	}); err != nil {
		t.Fatalf("upsert old: %v", err)
	}
	if err := store.Upsert(ctx, domain.TerminalScrollbackSnapshot{
		TenantID: testTenant1, WorktreeID: testScrollbackWorktree2, PaneKey: "pane-fresh",
		Cols: 80, Rows: 24, DataGzip: []byte("gz"), UncompressedBytes: 10, UpdatedAt: fresh,
	}); err != nil {
		t.Fatalf("upsert fresh: %v", err)
	}

	cutoff := time.Now().UTC().Add(-domain.ScrollbackSnapshotTTL)
	deleted, err := store.DeleteExpired(ctx, cutoff)
	if err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected exactly 1 expired row deleted, got %d", deleted)
	}

	foundOld, _, err := store.Get(ctx, testTenant1, testScrollbackWorktree1, "pane-old")
	if err != nil {
		t.Fatalf("get old: %v", err)
	}
	if foundOld {
		t.Error("expected the expired row to be gone")
	}
	foundFresh, _, err := store.Get(ctx, testTenant1, testScrollbackWorktree2, "pane-fresh")
	if err != nil {
		t.Fatalf("get fresh: %v", err)
	}
	if !foundFresh {
		t.Error("expected the fresh row to survive the expiry sweep")
	}
}
