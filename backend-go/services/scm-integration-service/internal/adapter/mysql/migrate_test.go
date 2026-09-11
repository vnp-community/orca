//go:build integration

// Integration tests run against a real MySQL via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/mysql/...`. Mirrors
// internal/adapter/postgres's own integration test shape (setupRepository
// per test, testTenant1/testTenant2 constants), split into one file per
// repository to match this package's one-file-per-table layout.
package mysql

import (
	"context"
	"database/sql"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/stablyai/orca-go/common/testutil"
)

// testTenant1/testTenant2 — every table's tenant_id column is CHAR(36)
// (matches every other service's tenant_id column in this codebase), so
// tests need real UUID strings, not short "tenant-1"-style IDs.
const (
	testTenant1 = "11111111-1111-1111-1111-111111111111"
	testTenant2 = "22222222-2222-2222-2222-222222222222"
)

// setupMySQLDB starts a disposable MySQL container, runs every
// migrations/mysql/*.up.sql against it via the golang-migrate CLI (same
// tool, not a hand-rolled runner, as internal/adapter/postgres's own
// integration test), and returns a ready *sql.DB. Each repository test
// file wraps this in its own setupXRepository(t) that also constructs the
// repository under test.
func setupMySQLDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := testutil.StartMySQL(t, "scm")

	migrationsPath, err := filepath.Abs("../../../migrations/mysql")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	cmd := exec.Command("migrate", "-path", migrationsPath, "-database", dsn, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running migrations: %v\n%s", err, out)
	}

	driverDSN, err := toMySQLDriverDSN(dsn)
	if err != nil {
		t.Fatalf("converting mysql dsn: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := sql.Open("mysql", driverDSN)
	if err != nil {
		t.Fatalf("opening mysql connection: %v", err)
	}
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("pinging mysql: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return db
}

// toMySQLDriverDSN is a minimal copy of cmd/server/main.go's own helper —
// this test binary doesn't import package main, and testutil.StartMySQL's
// DSN always uses the "tcp(host:port)" shape (see that function's doc
// comment), so this only needs the prefix-strip branch, not the
// net/url-parsing fallback the real one carries for arbitrary production
// DSNs.
func toMySQLDriverDSN(dsn string) (string, error) {
	const prefix = "mysql://"
	driverDSN := dsn[len(prefix):]
	return driverDSN + "?parseTime=true", nil
}
