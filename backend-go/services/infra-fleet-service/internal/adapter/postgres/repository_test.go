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
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
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
	testSshTarget3 = "88888888-8888-8888-8888-888888888888" // bastion
	testSshTarget4 = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" // target behind testSshTarget3
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

	target, err := domain.NewSshTarget(testSshTarget1, testTenant1, "10.0.0.9", 0, "deploy", "role-1", "", "")
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
	if got != target {
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

// TestSshTargetStore_PersistsPortKnownHostsAndJumpHost is the round-trip
// regression for the port/known_hosts_fingerprint/jump_host_target_id
// columns added in migrations/0007_ssh_targets_port_knownhosts_jumphost —
// TASK-SSH-01-04.
func TestSshTargetStore_PersistsPortKnownHostsAndJumpHost(t *testing.T) {
	_, store := setupSshTargetStore(t)
	ctx := context.Background()

	bastion, err := domain.NewSshTarget(testSshTarget3, testTenant1, "10.0.0.10", 2200, "deploy", "role-1", "SHA256:bastionfingerprint", "")
	if err != nil {
		t.Fatalf("building bastion ssh target: %v", err)
	}
	if _, err := store.Create(ctx, bastion); err != nil {
		t.Fatalf("creating bastion ssh target: %v", err)
	}

	behindBastion, err := domain.NewSshTarget(testSshTarget4, testTenant1, "192.168.1.5", 2222, "deploy", "role-2", "SHA256:targetfingerprint", testSshTarget3)
	if err != nil {
		t.Fatalf("building ssh target behind bastion: %v", err)
	}
	if _, err := store.Create(ctx, behindBastion); err != nil {
		t.Fatalf("creating ssh target behind bastion: %v", err)
	}

	gotBastion, err := store.Get(ctx, testTenant1, testSshTarget3)
	if err != nil {
		t.Fatalf("get bastion: %v", err)
	}
	if gotBastion != bastion {
		t.Errorf("expected bastion %+v, got %+v", bastion, gotBastion)
	}
	if gotBastion.JumpHostTargetID != "" {
		t.Errorf("expected bastion to have no jump host, got %q", gotBastion.JumpHostTargetID)
	}

	gotBehind, err := store.Get(ctx, testTenant1, testSshTarget4)
	if err != nil {
		t.Fatalf("get target behind bastion: %v", err)
	}
	if gotBehind != behindBastion {
		t.Errorf("expected %+v, got %+v", behindBastion, gotBehind)
	}
	if gotBehind.Port != 2222 {
		t.Errorf("expected port 2222 to round-trip, got %d", gotBehind.Port)
	}
	if gotBehind.KnownHostsFingerprint != "SHA256:targetfingerprint" {
		t.Errorf("expected known-hosts fingerprint to round-trip, got %q", gotBehind.KnownHostsFingerprint)
	}
	if gotBehind.JumpHostTargetID != testSshTarget3 {
		t.Errorf("expected jump_host_target_id to round-trip, got %q", gotBehind.JumpHostTargetID)
	}

	list, err := store.List(ctx, testTenant1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	foundBehind := false
	for _, target := range list {
		if target.ID == testSshTarget4 {
			foundBehind = true
			if target.JumpHostTargetID != testSshTarget3 {
				t.Errorf("expected List to also carry jump_host_target_id, got %+v", target)
			}
		}
	}
	if !foundBehind {
		t.Errorf("expected List to include %q, got %+v", testSshTarget4, list)
	}
}

// TestRepository_UpdateStatusAndGetDevServerByConnection is
// TASK-SSH-03-07's regression: TeardownConnection's two new repository
// methods against a real connections/dev_servers join.
func TestRepository_UpdateStatusAndGetDevServerByConnection(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	ds, err := domain.NewDevServer(testDevServer1, testTenant1, "10.0.0.9", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	if _, err := repo.Register(ctx, ds); err != nil {
		t.Fatalf("registering dev server: %v", err)
	}

	conn, err := domain.NewConnection("cccccccc-cccc-cccc-cccc-cccccccccccc", testTenant1, testDevServer1, "", "")
	if err != nil {
		t.Fatalf("building connection: %v", err)
	}
	created, err := repo.CreateConnection(ctx, conn)
	if err != nil {
		t.Fatalf("creating connection: %v", err)
	}

	gotDS, found, err := repo.GetDevServerByConnection(ctx, testTenant1, created.ID)
	if err != nil {
		t.Fatalf("GetDevServerByConnection: %v", err)
	}
	if !found || gotDS.ID != testDevServer1 {
		t.Errorf("expected to resolve dev server %q, got found=%v ds=%+v", testDevServer1, found, gotDS)
	}

	if err := repo.UpdateStatus(ctx, testTenant1, created.ID, "closed"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	// A closed connection no longer shows up as "active" (matches
	// GetActiveByDevServer's status <> 'closed' filter).
	if _, found, err := repo.GetActiveByDevServer(ctx, testTenant1, testDevServer1); err != nil || found {
		t.Errorf("expected no active connection after closing, found=%v err=%v", found, err)
	}

	// GetDevServerByConnection should still resolve the (now-closed)
	// connection — TeardownConnection's idempotent-close path relies on this.
	if _, found, err := repo.GetDevServerByConnection(ctx, testTenant1, created.ID); err != nil || !found {
		t.Errorf("expected GetDevServerByConnection to still resolve a closed connection, found=%v err=%v", found, err)
	}

	// An unknown connection id is a clean not-found, not an error.
	if _, found, err := repo.GetDevServerByConnection(ctx, testTenant1, testUnknownID); err != nil || found {
		t.Errorf("expected not-found for an unknown connection id, found=%v err=%v", found, err)
	}
}

// TestPortForwardStore_CreateThenListActiveByConnection_RoundTripsProcessNameAndStatus
// is TASK-SSH-04-03's regression: PortForwardStore.Create then
// ListActiveByConnection must round-trip ProcessName/Status, and a
// UpdateStatus(closed) row must drop out of ListActiveByConnection.
func TestPortForwardStore_CreateThenListActiveByConnection_RoundTripsProcessNameAndStatus(t *testing.T) {
	repo, sshTargetStore := setupSshTargetStore(t)
	portForwardStore := NewPortForwardStore(repo.pool)
	ctx := context.Background()

	// infra.port_forwards.connection_id FKs to infra.connections, which FKs
	// to infra.dev_servers — build the full chain, same as
	// TestRepository_RegisterAndGet_PersistsSSHTargetID does.
	sshTarget, err := domain.NewSshTarget(testSshTarget1, testTenant1, "10.0.0.9", 0, "deploy", "role-1", "", "")
	if err != nil {
		t.Fatalf("building ssh target: %v", err)
	}
	if _, err := sshTargetStore.Create(ctx, sshTarget); err != nil {
		t.Fatalf("creating ssh target: %v", err)
	}
	ds, err := domain.NewDevServer(testDevServerRS, testTenant1, "10.0.0.5", domain.ConnectionModeRelaySSH, testSshTarget1)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	if _, err := repo.Register(ctx, ds); err != nil {
		t.Fatalf("registering dev server: %v", err)
	}
	conn, err := domain.NewConnection("dddddddd-dddd-dddd-dddd-dddddddddddd", testTenant1, testDevServerRS, "", "")
	if err != nil {
		t.Fatalf("building connection: %v", err)
	}
	createdConn, err := repo.CreateConnection(ctx, conn)
	if err != nil {
		t.Fatalf("creating connection: %v", err)
	}

	pf := domain.PortForward{
		ID: "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", TenantID: testTenant1, ConnectionID: createdConn.ID,
		LocalPort: 3001, RemotePort: 3000, ProcessName: "node", Status: domain.PortForwardStatusActive,
	}
	if _, err := portForwardStore.Create(ctx, pf); err != nil {
		t.Fatalf("creating port forward: %v", err)
	}

	active, err := portForwardStore.ListActiveByConnection(ctx, testTenant1, createdConn.ID)
	if err != nil {
		t.Fatalf("ListActiveByConnection: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active port forward, got %d: %+v", len(active), active)
	}
	if active[0].ProcessName != "node" || active[0].Status != domain.PortForwardStatusActive {
		t.Errorf("expected ProcessName=node Status=active to round-trip, got %+v", active[0])
	}
	if active[0].LocalPort != 3001 || active[0].RemotePort != 3000 {
		t.Errorf("expected LocalPort/RemotePort to round-trip, got %+v", active[0])
	}

	if err := portForwardStore.UpdateStatus(ctx, testTenant1, pf.ID, domain.PortForwardStatusClosed); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	afterClose, err := portForwardStore.ListActiveByConnection(ctx, testTenant1, createdConn.ID)
	if err != nil {
		t.Fatalf("ListActiveByConnection after close: %v", err)
	}
	if len(afterClose) != 0 {
		t.Errorf("expected a closed port forward to drop out of ListActiveByConnection, got %+v", afterClose)
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

	target, err := domain.NewSshTarget(testSshTarget2, testTenant1, "10.0.0.9", 0, "deploy", "role-1", "", "")
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
