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
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// testTenant1/testTenant2/testDevServer1/testDevServer2 are shared fixture
// IDs referenced by several sibling integration test files in this package
// (dev_server_group_repository_test.go, dev_server_group_grant_repository_test.go,
// dev_server_access_request_repository_test.go) but never actually defined
// anywhere — this whole package failed to even compile under
// `-tags=integration` before this file existed (confirmed: `go vet
// -tags=integration ./internal/adapter/postgres/...` failed with "undefined:
// testTenant1" pre-existing, unrelated to this file's own new tests).
// Defined here since repository_test.go is this package's base/shared test
// file, matching e.g. testGroupParent/testGroupChild's placement in
// dev_server_group_repository_test.go.
const (
	testTenant1    = "11111111-1111-1111-1111-111111111111"
	testTenant2    = "22222222-2222-2222-2222-222222222222"
	testDevServer1 = "33333333-3333-3333-3333-333333333333"
	testDevServer2 = "44444444-4444-4444-4444-444444444444"
	testUnknownID  = "55555555-5555-5555-5555-555555555555"
)

func setupRepository(t *testing.T) *Repository {
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

	return &Repository{pool: pool}
}

// setupSshTargetStore is another shared fixture referenced by a sibling
// integration test file (dev_server_group_repository_test.go) but never
// defined anywhere — same pre-existing-gap situation as the test* ID
// constants above.
func setupSshTargetStore(t *testing.T) (*Repository, *SshTargetStore) {
	t.Helper()
	repo := setupRepository(t)
	return repo, NewSshTargetStore(repo.pool)
}

// TestFindByHostAndMode_DirectWebSocketWithNullSSHTargetID is the live-bug
// regression: a direct-websocket dev server always has ssh_target_id NULL
// (that column only applies to relay-ssh mode) — scanning it directly into
// a plain string field made this call error on EVERY direct-websocket
// lookup, which made ResolveDirectWebSocketDevServer silently fall back to
// the raw external devServerID string as the agent session's registry key
// instead of the row's real UUID. Confirmed live in production: 3
// genuinely-connected, handshaked agents were invisible to
// IsDevServerConnected/ListDevServers for the entire session because of
// this exact error ("cannot scan NULL into *string").
func TestFindByHostAndMode_DirectWebSocketWithNullSSHTargetID(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	registered, err := repo.Register(ctx, domain.DevServer{
		ID:       uuid.NewString(),
		TenantID: tenantID,
		Host:     "dev-01",
		Mode:     domain.ConnectionModeDirectWebSocket,
		Status:   domain.DevServerStatusApproved,
		// SSHTargetID intentionally empty — Register must persist this as
		// SQL NULL for direct-websocket mode, matching production data.
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	found, ok, err := repo.FindByHostAndMode(ctx, tenantID, "dev-01", domain.ConnectionModeDirectWebSocket)
	if err != nil {
		t.Fatalf("FindByHostAndMode returned an error instead of resolving the row (this is the exact live bug): %v", err)
	}
	if !ok {
		t.Fatal("FindByHostAndMode: want found=true")
	}
	if found.ID != registered.ID {
		t.Errorf("want resolved ID=%q (the real row, reused across reconnects), got %q", registered.ID, found.ID)
	}
	if found.SSHTargetID != "" {
		t.Errorf("want SSHTargetID empty for a direct-websocket row, got %q", found.SSHTargetID)
	}
}

func TestFindByHostAndMode_NoMatchReturnsNotFound(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	_, ok, err := repo.FindByHostAndMode(ctx, uuid.NewString(), "no-such-host", domain.ConnectionModeDirectWebSocket)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("want found=false for a host with no registered dev server")
	}
}
