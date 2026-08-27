//go:build integration

// Integration tests run against a real Postgres via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/postgres/...`.
//
// Every id/tenant_id below must be a syntactically valid UUID literal — the
// schema types these columns as UUID (see migrations/0001_init.up.sql), so
// Postgres rejects a plain string like "ds1" with "invalid input syntax for
// type uuid", not silently coercing it.
package postgres

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

const (
	testTenant1 = "11111111-1111-1111-1111-111111111111"
	testTenant2 = "22222222-2222-2222-2222-222222222222"

	testDevServer1  = "33333333-3333-3333-3333-333333333333"
	testDevServer2  = "44444444-4444-4444-4444-444444444444"
	testDevServerRS = "77777777-7777-7777-7777-777777777777"
	testUnknownID   = "99999999-9999-9999-9999-999999999999"

	testSshTarget1 = "55555555-5555-5555-5555-555555555555"
	testSshTarget2 = "66666666-6666-6666-6666-666666666666"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	repo, _ := setupSshTargetStore(t)
	return repo
}

// setupSshTargetStore starts a fresh Postgres container, runs every
// migration against it, and returns both Repository and SshTargetStore over
// the same pool — see internal/adapter/postgres/repository.go's doc comment
// for why they're two Go values rather than one.
func setupSshTargetStore(t *testing.T) (*Repository, *SshTargetStore) {
	t.Helper()
	dsn := testutil.StartPostgres(t, "infra")

	migrationsPath, err := filepath.Abs("../../../migrations")
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

	return New(pool), NewSshTargetStore(pool)
}

func TestRepository_ResolveConnection_FoundAndNotFound(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	ds, err := domain.NewDevServer(testDevServer1, testTenant1, "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	if _, err := repo.Register(ctx, ds); err != nil {
		t.Fatalf("registering dev server: %v", err)
	}

	connected, got, _, err := repo.ResolveConnection(ctx, testTenant1, testDevServer1)
	if err != nil {
		t.Fatalf("resolve connection: %v", err)
	}
	if !connected || got.ID != testDevServer1 {
		t.Errorf("expected connected=true, dev server %s, got connected=%v dev_server=%+v", testDevServer1, connected, got)
	}

	connected, _, _, err = repo.ResolveConnection(ctx, testTenant1, testUnknownID)
	if err != nil {
		t.Fatalf("resolve connection: %v", err)
	}
	if connected {
		t.Error("expected connected=false for an unregistered connectionId")
	}

	// Cross-tenant lookup must never succeed, even for a valid id.
	connected, _, _, err = repo.ResolveConnection(ctx, testTenant2, testDevServer1)
	if err != nil {
		t.Fatalf("resolve connection: %v", err)
	}
	if connected {
		t.Error("expected connected=false when the dev server belongs to a different tenant")
	}
}

func TestRepository_List_FiltersByTenant(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	ds1, _ := domain.NewDevServer(testDevServer1, testTenant1, "10.0.0.1", domain.ConnectionModeRelayWebSocket, "")
	ds2, _ := domain.NewDevServer(testDevServer2, testTenant2, "10.0.0.2", domain.ConnectionModeRelayWebSocket, "")
	_, _ = repo.Register(ctx, ds1)
	_, _ = repo.Register(ctx, ds2)

	got, err := repo.List(ctx, testTenant1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].TenantID != testTenant1 {
		t.Errorf("expected only tenant-1's dev server, got %+v", got)
	}
}

// TestSshTargetStore_Get_FoundAndNotFound covers usecase.SshTargetRepository.Get
// / usecase.SshTargetResolver.Get — the read path adapter/devserveragent.Client
// uses (via WithRelaySSH) to resolve a DevServer.SSHTargetID into a full
// domain.SshTarget before dialing.
func TestSshTargetStore_Get_FoundAndNotFound(t *testing.T) {
	_, store := setupSshTargetStore(t)
	ctx := context.Background()

	target, err := domain.NewSshTarget(testSshTarget1, testTenant1, "10.0.0.9", "deploy", "role-1", "", nil)
	if err != nil {
		t.Fatalf("building ssh target: %v", err)
	}
	if _, err := store.Create(ctx, target); err != nil {
		t.Fatalf("creating ssh target: %v", err)
	}

	got, err := store.Get(ctx, testTenant1, testSshTarget1)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !reflect.DeepEqual(got, target) {
		t.Errorf("expected %+v, got %+v", target, got)
	}

	// Cross-tenant lookup must never succeed, even for a valid id.
	if _, err := store.Get(ctx, testTenant2, testSshTarget1); err == nil {
		t.Error("expected an error when the ssh target belongs to a different tenant")
	}

	if _, err := store.Get(ctx, testTenant1, testUnknownID); err == nil {
		t.Error("expected an error for an unregistered ssh target id")
	}
}

// TestSshTargetStore_Upsert covers the (tenant_id, host, user_name) upsert
// path migrations/0007's unique index enables — first call inserts
// (updated=false), second call with a changed vault_ssh_role updates the
// same row in place (updated=true), row count stays 1.
func TestSshTargetStore_Upsert(t *testing.T) {
	_, store := setupSshTargetStore(t)
	ctx := context.Background()

	first, err := domain.NewSshTarget(uuid.NewString(), testTenant1, "10.0.0.42", "deploy", "role-1", "team-a", []string{"prod"})
	if err != nil {
		t.Fatalf("building ssh target: %v", err)
	}
	saved, updated, err := store.Upsert(ctx, first)
	if err != nil {
		t.Fatalf("upsert (insert): %v", err)
	}
	if updated {
		t.Error("expected updated=false on first insert")
	}

	second, err := domain.NewSshTarget(uuid.NewString(), testTenant1, "10.0.0.42", "deploy", "role-2", "team-b", []string{"prod", "canary"})
	if err != nil {
		t.Fatalf("building ssh target: %v", err)
	}
	saved2, updated2, err := store.Upsert(ctx, second)
	if err != nil {
		t.Fatalf("upsert (update): %v", err)
	}
	if !updated2 {
		t.Error("expected updated=true on second upsert with same (tenant_id,host,user_name)")
	}
	if saved2.ID != saved.ID {
		t.Errorf("expected the conflicting row's id %q to be preserved, got %q", saved.ID, saved2.ID)
	}
	if saved2.VaultSSHRole != "role-2" || saved2.Project != "team-b" {
		t.Errorf("expected updated fields to stick, got %+v", saved2)
	}

	got, found, err := store.GetByHostUser(ctx, testTenant1, "10.0.0.42", "deploy")
	if err != nil {
		t.Fatalf("get by host/user: %v", err)
	}
	if !found {
		t.Fatal("expected the upserted row to be found")
	}
	if got.VaultSSHRole != "role-2" || !reflect.DeepEqual(got.Tags, []string{"prod", "canary"}) {
		t.Errorf("expected latest values to round-trip, got %+v", got)
	}

	_, found, err = store.GetByHostUser(ctx, testTenant1, "10.0.0.42", "no-such-user")
	if err != nil {
		t.Fatalf("get by host/user (miss): %v", err)
	}
	if found {
		t.Error("expected found=false for an unregistered host/user pair")
	}
}

// TestRepository_RegisterAndGet_PersistsSSHTargetID is the round-trip
// regression for the ssh_target_id column added in
// migrations/0003_dev_server_ssh_target — a relay-ssh DevServer must come
// back out of Register/Get/List with the same SSHTargetID it was created
// with, not silently dropped.
func TestRepository_RegisterAndGet_PersistsSSHTargetID(t *testing.T) {
	repo, store := setupSshTargetStore(t)
	ctx := context.Background()

	target, err := domain.NewSshTarget(testSshTarget2, testTenant1, "10.0.0.9", "deploy", "role-1", "", nil)
	if err != nil {
		t.Fatalf("building ssh target: %v", err)
	}
	if _, err := store.Create(ctx, target); err != nil {
		t.Fatalf("creating ssh target: %v", err)
	}

	ds, err := domain.NewDevServer(testDevServerRS, testTenant1, "10.0.0.5", domain.ConnectionModeRelaySSH, testSshTarget2)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	if _, err := repo.Register(ctx, ds); err != nil {
		t.Fatalf("registering dev server: %v", err)
	}

	got, err := repo.Get(ctx, testTenant1, testDevServerRS)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.SSHTargetID != testSshTarget2 {
		t.Errorf("expected SSHTargetID %q to round-trip, got %q", testSshTarget2, got.SSHTargetID)
	}

	list, err := repo.List(ctx, testTenant1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].SSHTargetID != testSshTarget2 {
		t.Errorf("expected List to also carry SSHTargetID, got %+v", list)
	}
}

// TestRepository_UpdateProvisionResult covers migrations/0008's status/
// platform columns — persists status/platform/node_version/
// last_provisioned_at, and a second call updates the same row in place
// (idempotent, no duplicate row).
func TestRepository_UpdateProvisionResult(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	ds, err := domain.NewDevServer(testDevServer1, testTenant1, "10.0.0.9", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	if _, err := repo.Register(ctx, ds); err != nil {
		t.Fatalf("registering dev server: %v", err)
	}

	provisionedAt := time.Now().UTC().Truncate(time.Millisecond)
	info := usecase.HandshakeInfo{Platform: "linux", Arch: "x64", NodeVersion: "v22.0.0", AgentVersion: "5.0.0"}
	if err := repo.UpdateProvisionResult(ctx, testTenant1, testDevServer1, domain.DevServerStatusHealthy, info, provisionedAt); err != nil {
		t.Fatalf("update provision result: %v", err)
	}

	var status, platform, arch, nodeVersion, agentVersion string
	var lastProvisionedAt time.Time
	row := repo.pool.QueryRow(ctx, `SELECT status, platform, arch, node_version, agent_version, last_provisioned_at FROM infra.dev_servers WHERE tenant_id = $1 AND id = $2`, testTenant1, testDevServer1)
	if err := row.Scan(&status, &platform, &arch, &nodeVersion, &agentVersion, &lastProvisionedAt); err != nil {
		t.Fatalf("scanning updated row: %v", err)
	}
	if status != string(domain.DevServerStatusHealthy) || platform != "linux" || nodeVersion != "v22.0.0" {
		t.Errorf("expected persisted status/platform/node_version, got status=%q platform=%q node_version=%q", status, platform, nodeVersion)
	}
	if !lastProvisionedAt.Equal(provisionedAt) {
		t.Errorf("expected last_provisioned_at %v, got %v", provisionedAt, lastProvisionedAt)
	}

	// Second call updates the same row in place — no duplicate row, status
	// transitions cleanly.
	if err := repo.UpdateProvisionResult(ctx, testTenant1, testDevServer1, domain.DevServerStatusDegraded, info, provisionedAt.Add(time.Minute)); err != nil {
		t.Fatalf("second update provision result: %v", err)
	}
	var count int
	if err := repo.pool.QueryRow(ctx, `SELECT count(*) FROM infra.dev_servers WHERE tenant_id = $1 AND id = $2`, testTenant1, testDevServer1).Scan(&count); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 row after two updates, got %d", count)
	}
	var status2 string
	if err := repo.pool.QueryRow(ctx, `SELECT status FROM infra.dev_servers WHERE tenant_id = $1 AND id = $2`, testTenant1, testDevServer1).Scan(&status2); err != nil {
		t.Fatalf("scanning updated status: %v", err)
	}
	if status2 != string(domain.DevServerStatusDegraded) {
		t.Errorf("expected status to have transitioned to degraded, got %q", status2)
	}
}

// registerTestDevServer is a small helper for the fleet-health/advisory-lock
// tests below — they need a real infra.dev_servers row to satisfy
// fleet_health's FK, but don't care about its ssh_target/mode details.
func registerTestDevServer(t *testing.T, repo *Repository, id string) domain.DevServer {
	t.Helper()
	ds, err := domain.NewDevServer(id, testTenant1, "10.0.0.77", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	if _, err := repo.Register(context.Background(), ds); err != nil {
		t.Fatalf("registering dev server: %v", err)
	}
	return ds
}

// TestUpsertFleetHealthAndGetPrevious covers the upsert-by-PK round trip
// (dev_server_id is fleet_health's primary key) and GetPrevious's
// found=false-on-no-prior-sample case.
func TestUpsertFleetHealthAndGetPrevious(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	ds := registerTestDevServer(t, repo, testDevServer1)

	if _, found, err := repo.GetPrevious(ctx, ds.ID); err != nil || found {
		t.Fatalf("expected found=false before any sample exists, got found=%v err=%v", found, err)
	}

	sample := domain.DevServerHealth{
		DevServerID: ds.ID, Reachable: true, CPUPercent: 42.5, RAMPercent: 30, DiskPercent: 10,
		LatencyMS: 12, Status: domain.HealthStatusHealthy,
	}
	if err := repo.UpsertFleetHealth(ctx, sample); err != nil {
		t.Fatalf("upsert fleet health: %v", err)
	}

	got, found, err := repo.GetPrevious(ctx, ds.ID)
	if err != nil {
		t.Fatalf("get previous: %v", err)
	}
	if !found {
		t.Fatal("expected found=true after an upsert")
	}
	if got.Status != domain.HealthStatusHealthy || got.CPUPercent != 42.5 {
		t.Errorf("expected the upserted sample to round-trip, got %+v", got)
	}

	// Second upsert with a changed status updates the same row in place —
	// dev_server_id is the PK, so no duplicate row is possible.
	sample.Status = domain.HealthStatusDegraded
	sample.CPUPercent = 90
	if err := repo.UpsertFleetHealth(ctx, sample); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	got2, _, err := repo.GetPrevious(ctx, ds.ID)
	if err != nil {
		t.Fatalf("get previous (2nd): %v", err)
	}
	if got2.Status != domain.HealthStatusDegraded || got2.CPUPercent != 90 {
		t.Errorf("expected the second upsert's values, got %+v", got2)
	}

	var count int
	if err := repo.pool.QueryRow(ctx, `SELECT count(*) FROM infra.fleet_health WHERE dev_server_id = $1`, ds.ID).Scan(&count); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 row after two upserts, got %d", count)
	}
}

// TestTryLock_MutualExclusionAndReleaseAllowsReacquire is the concurrency
// property TASK-FLEET-03-04 specifically calls out: two concurrent TryLock
// calls for the same devServerID from two separate connections — exactly
// one succeeds; after unlock(), a subsequent TryLock succeeds again.
func TestTryLock_MutualExclusionAndReleaseAllowsReacquire(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	devServerID := "advisory-lock-test-target"

	locked1, unlock1, err := repo.TryLock(ctx, devServerID)
	if err != nil {
		t.Fatalf("first TryLock: %v", err)
	}
	if !locked1 {
		t.Fatal("expected the first TryLock to succeed")
	}

	locked2, unlock2, err := repo.TryLock(ctx, devServerID)
	if err != nil {
		t.Fatalf("second TryLock: %v", err)
	}
	if locked2 {
		t.Error("expected the second concurrent TryLock for the same devServerID to fail")
	}
	if unlock2 != nil {
		t.Error("expected a nil unlock func when locked=false")
	}

	unlock1()

	locked3, unlock3, err := repo.TryLock(ctx, devServerID)
	if err != nil {
		t.Fatalf("third TryLock (after release): %v", err)
	}
	if !locked3 {
		t.Fatal("expected TryLock to succeed again after the first lock was released")
	}
	unlock3()
}

// TestListAllForPolling_IsCrossTenant covers the one thing that
// distinguishes this method from List: no tenant_id filter — dev servers
// from every tenant come back.
func TestListAllForPolling_IsCrossTenant(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	ds1, err := domain.NewDevServer(testDevServer1, testTenant1, "10.0.0.1", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server 1: %v", err)
	}
	if _, err := repo.Register(ctx, ds1); err != nil {
		t.Fatalf("registering dev server 1: %v", err)
	}
	ds2, err := domain.NewDevServer(testDevServer2, testTenant2, "10.0.0.2", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server 2: %v", err)
	}
	if _, err := repo.Register(ctx, ds2); err != nil {
		t.Fatalf("registering dev server 2: %v", err)
	}

	got, err := repo.ListAllForPolling(ctx)
	if err != nil {
		t.Fatalf("list all for polling: %v", err)
	}
	tenants := map[string]bool{}
	for _, ds := range got {
		tenants[ds.TenantID] = true
	}
	if !tenants[testTenant1] || !tenants[testTenant2] {
		t.Errorf("expected dev servers from both tenants, got %+v", got)
	}
}

// TestOutboxEnqueueFetchMarkPublished covers the round trip
// EnqueueOutboxEvent -> FetchUnpublished -> MarkPublished ->
// FetchUnpublished (empty) that outbox.Relay drives in production.
func TestOutboxEnqueueFetchMarkPublished(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	id := uuid.NewString()
	occurredAt := time.Now().UTC().Truncate(time.Millisecond)
	payload := []byte(`{"devServerId":"ds1","from":"healthy","to":"degraded"}`)
	if err := repo.EnqueueOutboxEvent(ctx, id, testTenant1, "dev_server.health_degraded", occurredAt, 1, payload); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	unpublished, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("fetch unpublished: %v", err)
	}
	if len(unpublished) != 1 {
		t.Fatalf("expected exactly 1 unpublished row, got %d", len(unpublished))
	}
	rec := unpublished[0]
	if rec.ID != id || rec.Subject != "dev_server.health_degraded" {
		t.Errorf("unexpected record: %+v", rec)
	}
	if rec.Event.TenantID != testTenant1 {
		t.Errorf("unexpected tenant id: %+v", rec.Event)
	}
	// JSONB round-trips semantically, not byte-for-byte (Postgres
	// normalizes key order/whitespace) — compare decoded values instead.
	var gotPayload, wantPayload map[string]any
	if err := json.Unmarshal(rec.Event.Payload, &gotPayload); err != nil {
		t.Fatalf("unmarshaling returned payload: %v", err)
	}
	if err := json.Unmarshal(payload, &wantPayload); err != nil {
		t.Fatalf("unmarshaling expected payload: %v", err)
	}
	if !reflect.DeepEqual(gotPayload, wantPayload) {
		t.Errorf("expected payload %+v, got %+v", wantPayload, gotPayload)
	}

	if err := repo.MarkPublished(ctx, []string{id}); err != nil {
		t.Fatalf("mark published: %v", err)
	}

	stillUnpublished, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("fetch unpublished (2nd): %v", err)
	}
	if len(stillUnpublished) != 0 {
		t.Errorf("expected zero unpublished rows after MarkPublished, got %d", len(stillUnpublished))
	}
}
