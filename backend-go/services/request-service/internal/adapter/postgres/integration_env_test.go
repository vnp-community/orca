//go:build integration

package postgres

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const appRole = "request_app"

// pgFixture is one migrated Postgres. admin is the superuser (migrations, assertions);
// app connects as a NOSUPERUSER NOBYPASSRLS role, which is what RLS tests must use.
type pgFixture struct {
	adminDSN string
	admin    *pgx.Conn
	app      *pgxpool.Pool
	appDSN   string
}

func applyScripts(t *testing.T, conn *pgx.Conn, scripts []string) {
	t.Helper()
	for i, s := range scripts {
		if _, err := conn.Exec(context.Background(), s); err != nil {
			t.Fatalf("migration script %d: %v", i, err)
		}
	}
}

var shared struct {
	once      sync.Once
	container testcontainers.Container
	host      string
	port      string
	err       error
	dbCounter atomic.Int64
}

// TestMain keeps one Postgres container for the whole package; each test gets its own
// database inside it, which is far cheaper than a container per test.
func TestMain(m *testing.M) {
	code := m.Run()
	if shared.container != nil {
		_ = shared.container.Terminate(context.Background())
	}
	os.Exit(code)
}

func sharedPostgresAddr(t *testing.T) (host, port string) {
	t.Helper()
	shared.once.Do(func() {
		ctx := context.Background()
		c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "postgres:16-alpine",
				ExposedPorts: []string{"5432/tcp"},
				Env:          map[string]string{"POSTGRES_USER": "orca", "POSTGRES_PASSWORD": "orca", "POSTGRES_DB": "postgres"},
				// The first "ready" line comes from initdb's throwaway server; wait for the real one.
				WaitingFor: wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
			},
			Started: true,
		})
		if err != nil {
			shared.err = err
			return
		}
		shared.container = c
		if shared.host, shared.err = c.Host(ctx); shared.err != nil {
			return
		}
		p, err := c.MappedPort(ctx, "5432")
		if err != nil {
			shared.err = err
			return
		}
		shared.port = p.Port()
	})
	if shared.err != nil {
		t.Fatalf("start postgres: %v", shared.err)
	}
	return shared.host, shared.port
}

// startPostgres returns the superuser DSN and connection of a brand-new empty database.
func startPostgres(t *testing.T) (adminDSN string, admin *pgx.Conn) {
	t.Helper()
	host, port := sharedPostgresAddr(t)
	ctx := context.Background()
	root, err := pgx.Connect(ctx, fmt.Sprintf("postgres://orca:orca@%s:%s/postgres?sslmode=disable", host, port))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close(ctx) }()
	name := fmt.Sprintf("t%d", shared.dbCounter.Add(1))
	if _, err := root.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	adminDSN = fmt.Sprintf("postgres://orca:orca@%s:%s/%s?sslmode=disable", host, port, name)
	admin, err = pgx.Connect(ctx, adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(ctx) })
	return adminDSN, admin
}

func newMigratedPostgres(t *testing.T) *pgFixture {
	t.Helper()
	adminDSN, admin := startPostgres(t)
	applyScripts(t, admin, contracttest.MigrationScripts(t, "postgres", "up"))

	ctx := context.Background()
	// Roles are cluster-wide, so create it once and grant per database.
	if _, err := admin.Exec(ctx, fmt.Sprintf(`DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '%s') THEN
		CREATE ROLE %s LOGIN PASSWORD 'app_pw' NOSUPERUSER NOBYPASSRLS; END IF; END $$`, appRole, appRole)); err != nil {
		t.Fatalf("create role: %v", err)
	}
	for _, stmt := range []string{
		fmt.Sprintf(`GRANT USAGE ON SCHEMA request TO %s`, appRole),
		fmt.Sprintf(`GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA request TO %s`, appRole),
	} {
		if _, err := admin.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	u, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(appRole, "app_pw")
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return &pgFixture{adminDSN: adminDSN, admin: admin, app: pool, appDSN: u.String()}
}

func (f *pgFixture) contractEnv() contracttest.Env {
	base := New(f.app)
	return contracttest.Env{
		Tx:          base,
		Requests:    NewRequestRepository(base),
		History:     NewRequestTypeHistoryRepository(base),
		Solutions:   NewSolutionRecordRepository(base),
		Links:       NewRequestLinkRepository(base),
		Idempotency: NewRequestIdempotencyRepository(base),
	}
}
