//go:build integration

package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/moby/moby/api/types/network"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const sessionParams = "parseTime=true&loc=UTC&time_zone=%27%2B00%3A00%27"

type myFixture struct {
	db    *sql.DB // as the service uses it (no multiStatements)
	admin *sql.DB // multiStatements, for migrations and assertions
}

func applyScripts(t *testing.T, db *sql.DB, scripts []string) {
	t.Helper()
	for i, s := range scripts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("migration script %d: %v", i, err)
		}
	}
}

var shared struct {
	once      sync.Once
	container testcontainers.Container
	hostPort  string
	err       error
	dbCounter atomic.Int64
}

// TestMain keeps one MySQL container for the whole package; each test gets its own database.
func TestMain(m *testing.M) {
	code := m.Run()
	if shared.container != nil {
		_ = shared.container.Terminate(context.Background())
	}
	os.Exit(code)
}

func sharedMySQLAddr(t *testing.T) string {
	t.Helper()
	shared.once.Do(func() {
		ctx := context.Background()
		c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "mysql:8.0",
				ExposedPorts: []string{"3306/tcp"},
				Env:          map[string]string{"MYSQL_ROOT_PASSWORD": "orca", "MYSQL_DATABASE": "request"},
				WaitingFor: wait.ForSQL("3306/tcp", "mysql", func(host string, port network.Port) string {
					return fmt.Sprintf("root:orca@tcp(%s:%s)/request", host, port.Port())
				}).WithStartupTimeout(3 * time.Minute).WithPollInterval(time.Second),
			},
			Started: true,
		})
		if err != nil {
			shared.err = err
			return
		}
		shared.container = c
		host, err := c.Host(ctx)
		if err != nil {
			shared.err = err
			return
		}
		p, err := c.MappedPort(ctx, "3306")
		if err != nil {
			shared.err = err
			return
		}
		shared.hostPort = fmt.Sprintf("%s:%s", host, p.Port())
	})
	if shared.err != nil {
		t.Fatalf("start mysql: %v", shared.err)
	}
	return shared.hostPort
}

// startMySQL returns two handles on a brand-new empty database: db as the service opens it,
// admin with multiStatements for migrations and assertions.
func startMySQL(t *testing.T) (db, admin *sql.DB) {
	t.Helper()
	addr := sharedMySQLAddr(t)
	root, err := sql.Open("mysql", fmt.Sprintf("root:orca@tcp(%s)/", addr))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	name := fmt.Sprintf("t%d", shared.dbCounter.Add(1))
	if _, err := root.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatal(err)
	}
	open := func(extra string) *sql.DB {
		d, err := sql.Open("mysql", fmt.Sprintf("root:orca@tcp(%s)/%s?%s%s", addr, name, sessionParams, extra))
		if err != nil {
			t.Fatal(err)
		}
		d.SetMaxOpenConns(25)
		t.Cleanup(func() { _ = d.Close() })
		return d
	}
	return open(""), open("&multiStatements=true")
}

func newMigratedMySQL(t *testing.T) *myFixture {
	t.Helper()
	db, admin := startMySQL(t)
	applyScripts(t, admin, contracttest.MigrationScripts(t, "mysql", "up"))
	return &myFixture{db: db, admin: admin}
}

func (f *myFixture) contractEnv() contracttest.Env {
	base := New(f.db)
	return contracttest.Env{
		Tx:          base,
		Requests:    NewRequestRepository(base),
		History:     NewRequestTypeHistoryRepository(base),
		Solutions:   NewSolutionRecordRepository(base),
		Links:       NewRequestLinkRepository(base),
		Idempotency: NewRequestIdempotencyRepository(base),
	}
}
