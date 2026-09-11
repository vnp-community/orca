//go:build integration

// Integration tests run against a real MySQL via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/mysql/...`. Mirrors
// internal/adapter/postgres/repository_test.go's fixture IDs and test
// names/shape where a direct Postgres equivalent exists, plus
// TestRepository_List_DoesNotLeakAcrossTenants /
// TestRepository_ListConnectivitySummary_DoesNotLeakAcrossTenants
// (TASK-BE-DB-003's tenant-isolation-without-RLS pattern, mirrored per
// BE-DB-SOL-017) — dev_servers/connections are RLS-bearing tables on
// Postgres (migrations/postgres/0001, 0002), so this proves
// application-layer tenant_id scoping alone is sufficient on a dialect with
// no RLS equivalent at all.
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
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

const (
	testTenant1 = "11111111-1111-1111-1111-111111111111"
	testTenant2 = "22222222-2222-2222-2222-222222222222"

	testDevServer1 = "33333333-3333-3333-3333-333333333333"
	testDevServer2 = "44444444-4444-4444-4444-444444444444"

	testSshTarget1 = "55555555-5555-5555-5555-555555555555"
)

// setupDB starts a fresh MySQL container, runs every migration in
// migrations/mysql against it, and returns a *sql.DB ready for any store in
// this package to wrap — shared by every test file here, mirroring
// internal/adapter/postgres/repository_test.go's setupRepository/
// setupSshTargetStore split.
func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	rawDSN := testutil.StartMySQL(t, "infra")
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
	return New(setupDB(t))
}

func setupSshTargetStore(t *testing.T) (*Repository, *SshTargetStore) {
	t.Helper()
	db := setupDB(t)
	return New(db), NewSshTargetStore(db)
}

func TestRepository_RegisterAndGet_RoundTripsFieldsIncludingTags(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	ds, err := domain.NewDevServer(testDevServer1, testTenant1, "10.0.0.1", domain.ConnectionModeRelayWebSocket, "", []string{"gpu", "region:us-east"})
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	if _, err := repo.Register(ctx, ds); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, err := repo.Get(ctx, testTenant1, testDevServer1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Host != "10.0.0.1" {
		t.Errorf("want Host=10.0.0.1, got %q", got.Host)
	}
	if got.Mode != domain.ConnectionModeRelayWebSocket {
		t.Errorf("want Mode=%q, got %q", domain.ConnectionModeRelayWebSocket, got.Mode)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "gpu" || got.Tags[1] != "region:us-east" {
		t.Errorf("want Tags=[gpu region:us-east] (JSON round-trip), got %+v", got.Tags)
	}
}

// TestRepository_ListByTag_UsesJSONContains proves migrations/mysql/0023's
// TEXT[]->JSON translation's read path — JSON_CONTAINS, not the Postgres
// `= ANY(tags)` array operator — still finds the right rows and excludes
// non-matches.
func TestRepository_ListByTag_UsesJSONContains(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tagged, err := domain.NewDevServer(uuid.NewString(), testTenant1, "gpu-1", domain.ConnectionModeRelayWebSocket, "", []string{"gpu"})
	if err != nil {
		t.Fatalf("building tagged dev server: %v", err)
	}
	if _, err := repo.Register(ctx, tagged); err != nil {
		t.Fatalf("register tagged: %v", err)
	}
	untagged, err := domain.NewDevServer(uuid.NewString(), testTenant1, "cpu-1", domain.ConnectionModeRelayWebSocket, "", []string{"cpu"})
	if err != nil {
		t.Fatalf("building untagged dev server: %v", err)
	}
	if _, err := repo.Register(ctx, untagged); err != nil {
		t.Fatalf("register untagged: %v", err)
	}

	got, err := repo.ListByTag(ctx, testTenant1, "gpu")
	if err != nil {
		t.Fatalf("ListByTag: %v", err)
	}
	if len(got) != 1 || got[0].ID != tagged.ID {
		t.Fatalf("want exactly the gpu-tagged dev server, got %+v", got)
	}
}

func TestRepository_UpdateApprovalStatus_And_AssignGroup(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	groupStore := NewDevServerGroupStore(repo.db)

	group, err := domain.NewDevServerGroup(uuid.NewString(), testTenant1, "Backend Team", "")
	if err != nil {
		t.Fatalf("building group: %v", err)
	}
	if _, err := groupStore.Create(ctx, group); err != nil {
		t.Fatalf("creating group: %v", err)
	}

	ds, err := domain.NewDevServer(testDevServer2, testTenant1, "10.0.0.2", domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	if _, err := repo.Register(ctx, ds); err != nil {
		t.Fatalf("registering dev server: %v", err)
	}

	approved, err := repo.UpdateApprovalStatus(ctx, testTenant1, testDevServer2, domain.DevServerStatusApproved)
	if err != nil {
		t.Fatalf("update approval status: %v", err)
	}
	if approved.Status != domain.DevServerStatusApproved {
		t.Errorf("want Status=approved, got %q", approved.Status)
	}

	assigned, err := repo.AssignGroup(ctx, testTenant1, testDevServer2, group.ID)
	if err != nil {
		t.Fatalf("assign group: %v", err)
	}
	if assigned.GroupID != group.ID {
		t.Errorf("want GroupID=%q, got %q", group.ID, assigned.GroupID)
	}

	unassigned, err := repo.AssignGroup(ctx, testTenant1, testDevServer2, "")
	if err != nil {
		t.Fatalf("unassign group: %v", err)
	}
	if unassigned.GroupID != "" {
		t.Errorf("want GroupID cleared, got %q", unassigned.GroupID)
	}
}

// TestRepository_List_DoesNotLeakAcrossTenants — TASK-BE-DB-003's pattern:
// dev_servers has an RLS policy on Postgres (migrations/postgres/0001) that
// (per BE-DB-SOL-001 §4) never actually activates; this proves the
// application-layer `WHERE tenant_id = ?` scoping alone is sufficient on
// MySQL, which has no RLS-equivalent backstop at all.
func TestRepository_List_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	dsA, _ := domain.NewDevServer(uuid.NewString(), testTenant1, "tenant-a-host", domain.ConnectionModeRelayWebSocket, "", nil)
	if _, err := repo.Register(ctx, dsA); err != nil {
		t.Fatalf("register tenant1 dev server: %v", err)
	}
	dsB, _ := domain.NewDevServer(uuid.NewString(), testTenant2, "tenant-b-host", domain.ConnectionModeRelayWebSocket, "", nil)
	if _, err := repo.Register(ctx, dsB); err != nil {
		t.Fatalf("register tenant2 dev server: %v", err)
	}

	got, err := repo.List(ctx, testTenant1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].ID != dsA.ID {
		t.Fatalf("tenant1's List leaked or missed rows: got %+v", got)
	}

	if _, err := repo.Get(ctx, testTenant1, dsB.ID); err == nil {
		t.Fatalf("Get must not resolve another tenant's dev server across the tenant boundary")
	}
}

func TestSshTargetStore_CreateGetListDelete(t *testing.T) {
	_, store := setupSshTargetStore(t)
	ctx := context.Background()

	target, err := domain.NewSshTarget(testSshTarget1, testTenant1, "bastion.example.com", 22, "orca", "role-1", "", "", "", nil)
	if err != nil {
		t.Fatalf("building ssh target: %v", err)
	}
	if _, err := store.Create(ctx, target); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.Get(ctx, testTenant1, testSshTarget1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Host != "bastion.example.com" || got.Port != 22 {
		t.Errorf("unexpected round-tripped ssh target: %+v", got)
	}

	list, err := store.List(ctx, testTenant1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 ssh target, got %d", len(list))
	}

	if err := store.Delete(ctx, testTenant1, testSshTarget1); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, _, err := store.GetByHostUser(ctx, testTenant1, "bastion.example.com", "orca"); err != nil {
		t.Fatalf("GetByHostUser after delete should not error (found=false): %v", err)
	}
}

// TestSshTargetStore_Upsert_InsertThenUpdate proves the
// affected-rows-based insert-vs-update signal (repository.go's Upsert doc
// comment) is correct in both directions against a real MySQL 8 server —
// not just a theoretical reading of the docs.
func TestSshTargetStore_Upsert_InsertThenUpdate(t *testing.T) {
	_, store := setupSshTargetStore(t)
	ctx := context.Background()

	first, err := domain.NewSshTarget(uuid.NewString(), testTenant1, "host-a", 22, "orca", "role-1", "", "", "proj-1", []string{"a"})
	if err != nil {
		t.Fatalf("building first target: %v", err)
	}
	saved1, updated1, err := store.Upsert(ctx, first)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if updated1 {
		t.Errorf("want updated=false for a fresh insert")
	}

	second, err := domain.NewSshTarget(uuid.NewString(), testTenant1, "host-a", 22, "orca", "role-2", "", "", "proj-2", []string{"b"})
	if err != nil {
		t.Fatalf("building second target: %v", err)
	}
	saved2, updated2, err := store.Upsert(ctx, second)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if !updated2 {
		t.Errorf("want updated=true when (tenant_id, host, user_name) already exists")
	}
	if saved2.ID != saved1.ID {
		t.Errorf("want the EXISTING row's id (%q) to win on conflict, got %q", saved1.ID, saved2.ID)
	}
	if saved2.VaultSSHRole != "role-2" || saved2.Project != "proj-2" {
		t.Errorf("want the new values to have overwritten the old row, got %+v", saved2)
	}
}

func TestRepository_ResolveConnection_And_CreateConnectionWithOutbox(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	ds, _ := domain.NewDevServer(testDevServer1, testTenant1, "10.0.0.9", domain.ConnectionModeRelayWebSocket, "", nil)
	if _, err := repo.Register(ctx, ds); err != nil {
		t.Fatalf("register dev server: %v", err)
	}

	connID := uuid.NewString()
	conn := domain.Connection{
		ID: connID, TenantID: testTenant1, DevServerID: testDevServer1,
		RepoPath: "/repo", WorktreeID: "wt-1", Status: "established",
	}
	event := domain.OutboxEvent{
		ID: uuid.NewString(), TenantID: testTenant1, Subject: "orca.infra.connection.established",
		OccurredAt: time.Now().UTC(), PayloadJSON: []byte(`{"ok":true}`),
	}
	if _, err := repo.CreateConnectionWithOutbox(ctx, conn, event); err != nil {
		t.Fatalf("CreateConnectionWithOutbox: %v", err)
	}

	connected, gotDS, gotConn, err := repo.ResolveConnection(ctx, testTenant1, connID)
	if err != nil {
		t.Fatalf("ResolveConnection: %v", err)
	}
	if !connected {
		t.Fatalf("want connected=true")
	}
	if gotDS.ID != testDevServer1 || gotConn.WorktreeID != "wt-1" {
		t.Errorf("unexpected resolve result: ds=%+v conn=%+v", gotDS, gotConn)
	}

	unpublished, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished: %v", err)
	}
	if len(unpublished) != 1 || unpublished[0].ID != event.ID {
		t.Fatalf("want the outbox event enqueued in the same tx, got %+v", unpublished)
	}
	if err := repo.MarkPublished(ctx, []string{event.ID}); err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}
	afterMark, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished after mark: %v", err)
	}
	if len(afterMark) != 0 {
		t.Errorf("want 0 unpublished after MarkPublished, got %d", len(afterMark))
	}
}

// TestRepository_ListConnectivitySummary_DoesNotLeakAcrossTenants —
// TASK-BE-DB-003's pattern applied to connections (also RLS-bearing on
// Postgres, migrations/postgres/0002).
func TestRepository_ListConnectivitySummary_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	dsA, _ := domain.NewDevServer(uuid.NewString(), testTenant1, "host-a", domain.ConnectionModeRelayWebSocket, "", nil)
	if _, err := repo.Register(ctx, dsA); err != nil {
		t.Fatalf("register: %v", err)
	}
	dsB, _ := domain.NewDevServer(uuid.NewString(), testTenant2, "host-b", domain.ConnectionModeRelayWebSocket, "", nil)
	if _, err := repo.Register(ctx, dsB); err != nil {
		t.Fatalf("register: %v", err)
	}

	if _, err := repo.CreateConnection(ctx, domain.Connection{ID: uuid.NewString(), TenantID: testTenant1, DevServerID: dsA.ID, WorktreeID: "wt-a"}); err != nil {
		t.Fatalf("create connection tenant1: %v", err)
	}
	if _, err := repo.CreateConnection(ctx, domain.Connection{ID: uuid.NewString(), TenantID: testTenant2, DevServerID: dsB.ID, WorktreeID: "wt-b"}); err != nil {
		t.Fatalf("create connection tenant2: %v", err)
	}

	got, err := repo.ListConnectivitySummary(ctx, testTenant1)
	if err != nil {
		t.Fatalf("ListConnectivitySummary: %v", err)
	}
	if len(got) != 1 || got[0].WorktreeID != "wt-a" {
		t.Fatalf("tenant1's connectivity summary leaked or missed rows: got %+v", got)
	}
}

func TestRepository_UpsertFleetHealth_And_TryLock(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	ds, _ := domain.NewDevServer(testDevServer1, testTenant1, "10.0.0.5", domain.ConnectionModeRelayWebSocket, "", nil)
	if _, err := repo.Register(ctx, ds); err != nil {
		t.Fatalf("register: %v", err)
	}

	if _, found, err := repo.GetDevServerHealth(ctx, testDevServer1); err != nil || found {
		t.Fatalf("want no health sample yet, found=%v err=%v", found, err)
	}

	if err := repo.UpsertFleetHealth(ctx, domain.DevServerHealth{
		DevServerID: testDevServer1, Reachable: true, CPUPercent: 12.5, RAMPercent: 40, DiskPercent: 10, LatencyMS: 20, Status: domain.HealthStatusHealthy,
	}); err != nil {
		t.Fatalf("UpsertFleetHealth (insert): %v", err)
	}
	// Upsert again — ON DUPLICATE KEY UPDATE path (dev_server_id is the PK).
	if err := repo.UpsertFleetHealth(ctx, domain.DevServerHealth{
		DevServerID: testDevServer1, Reachable: false, CPUPercent: 99, RAMPercent: 90, DiskPercent: 80, LatencyMS: 500, Status: domain.HealthStatusUnreachable,
	}); err != nil {
		t.Fatalf("UpsertFleetHealth (update): %v", err)
	}

	got, found, err := repo.GetDevServerHealth(ctx, testDevServer1)
	if err != nil || !found {
		t.Fatalf("want a health sample now, found=%v err=%v", found, err)
	}
	if got.Reachable || got.Status != domain.HealthStatusUnreachable {
		t.Errorf("want the SECOND upsert's values to have overwritten the first, got %+v", got)
	}

	summary, err := repo.GetFleetHealth(ctx, testTenant1)
	if err != nil {
		t.Fatalf("GetFleetHealth: %v", err)
	}
	if len(summary) != 1 {
		t.Fatalf("want 1 fleet health row for tenant1, got %+v", summary)
	}

	locked, unlock, err := repo.TryLock(ctx, testDevServer1)
	if err != nil {
		t.Fatalf("TryLock: %v", err)
	}
	if !locked {
		t.Fatalf("want the first TryLock to succeed")
	}
	defer unlock()
}

func TestPortForwardStore_CreateAndListActive(t *testing.T) {
	repo := setupRepository(t)
	db := repo.db
	pfStore := NewPortForwardStore(db)
	ctx := context.Background()

	ds, _ := domain.NewDevServer(testDevServer1, testTenant1, "10.0.0.7", domain.ConnectionModeRelayWebSocket, "", nil)
	if _, err := repo.Register(ctx, ds); err != nil {
		t.Fatalf("register: %v", err)
	}
	connID := uuid.NewString()
	if _, err := repo.CreateConnection(ctx, domain.Connection{ID: connID, TenantID: testTenant1, DevServerID: testDevServer1, WorktreeID: "wt-pf"}); err != nil {
		t.Fatalf("create connection: %v", err)
	}

	pf := domain.PortForward{
		ID: uuid.NewString(), TenantID: testTenant1, ConnectionID: connID,
		LocalPort: 3000, RemotePort: 3000, ProcessName: "vite", Status: domain.PortForwardStatusActive,
	}
	if _, err := pfStore.Create(ctx, pf); err != nil {
		t.Fatalf("Create: %v", err)
	}

	active, err := pfStore.ListActiveByConnection(ctx, testTenant1, connID)
	if err != nil {
		t.Fatalf("ListActiveByConnection: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("want 1 active port forward, got %d", len(active))
	}

	if err := pfStore.UpdateStatus(ctx, testTenant1, pf.ID, domain.PortForwardStatusClosed); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	afterClose, err := pfStore.ListActiveByConnection(ctx, testTenant1, connID)
	if err != nil {
		t.Fatalf("ListActiveByConnection after close: %v", err)
	}
	if len(afterClose) != 0 {
		t.Errorf("want 0 active port forwards after closing, got %d", len(afterClose))
	}
}
