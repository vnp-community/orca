package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// StartMySQL launches a disposable MySQL container for the duration of a
// test and returns its connection DSN in the same URL form
// dbcapability.DetectDialectFromDSN recognizes:
// "mysql://root:orca@tcp(host:port)/dbName". Callers must strip the
// "mysql://" scheme prefix before handing this to
// database/sql.Open("mysql", ...) — the go-sql-driver/mysql driver expects
// its own "user:pass@tcp(host:port)/db" format, not a URL (see
// TASK-BE-DB-006's toMySQLDriverDSN for the general-purpose conversion;
// this fixed-shape test DSN only needs a prefix strip, not full URL
// parsing). The container is terminated automatically via t.Cleanup —
// callers don't need their own defer. Mirrors StartPostgres's shape/use of
// testcontainers-go's GenericContainer rather than a dedicated module
// package, so no new dependency is added to common/go.mod.
func StartMySQL(t *testing.T, dbName string) string {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "mysql:8",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "orca",
			"MYSQL_DATABASE":      dbName,
		},
		// A log-line wait (even waiting for the 2nd "ready for
		// connections", since MySQL's entrypoint restarts once after
		// initial bootstrap) still races the server actually accepting
		// TCP connections — observed in practice as `migrate`'s first
		// connection attempt getting "unexpected EOF"/"bad connection"
		// immediately after the container was reported ready. ForSQL
		// instead retries a real `SELECT 1` over database/sql until it
		// succeeds, which is what actually matters. This relies on the
		// "mysql" driver already being registered in the CALLING test
		// binary's process (every caller blank-imports
		// github.com/go-sql-driver/mysql for its own use already, e.g.
		// internal/adapter/mysql/repository_test.go) — common/go.mod
		// intentionally does NOT depend on the driver package itself, only
		// on the driver name string "mysql" being registered process-wide
		// by the time this runs.
		WaitingFor: wait.ForSQL("3306/tcp", "mysql", func(host string, port network.Port) string {
			return fmt.Sprintf("root:orca@tcp(%s:%s)/%s?parseTime=true", host, port.Port(), dbName)
		}).WithStartupTimeout(2 * time.Minute).WithPollInterval(500 * time.Millisecond),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("testutil: starting mysql container: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("testutil: getting container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "3306")
	if err != nil {
		t.Fatalf("testutil: getting mapped port: %v", err)
	}

	return fmt.Sprintf("mysql://root:orca@tcp(%s:%s)/%s", host, port.Port(), dbName)
}
