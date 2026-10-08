//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/moby/moby/api/types/network"
	"github.com/stablyai/orca-go/services/request-service/e2e/stubs"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// The stack under test is the real request-service binary (real wiring, interceptors, outbox relay and NATS
// consumers) on a real database. Only what lies behind its outbound gRPC calls is stubbed (see package stubs).
// Docker is required; the dialect comes from E2E_DIALECT (postgres, the default, or mysql).

const (
	serviceToken = "e2e-service-token"
	e2eOwner     = "rf-roll-e2e"
)

type stack struct {
	dialect  string
	grpcAddr string
	httpPort int
	natsURL  string
	stubs    *stubs.Servers
	db       dbReader
	logs     *syncBuffer
	proc     *exec.Cmd
	cleanups []func()

	serviceDSN, stubAddr, binary string
}

// dbReader reads outbox rows for assertions (superuser, so it also sees what RLS hides from the service role).
type dbReader interface {
	outboxSubjects(t *testing.T, tenantID string) []string
	outboxPayloads(t *testing.T, tenantID, subject string) []string
	requestStatus(t *testing.T, requestID string) string
	// renameTenantSettings renames the flag table, which makes the flag unreadable (fault injection for E20).
	renameTenantSettings(t *testing.T, to string)
}

var theStack *stack

func TestMain(m *testing.M) {
	st, err := startStack()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: cannot start the stack (Docker needed): %v\n", err)
		if st != nil {
			st.stop()
		}
		os.Exit(1)
	}
	theStack = st
	code := m.Run()
	st.stop()
	os.Exit(code)
}

func startStack() (*stack, error) {
	st := &stack{dialect: os.Getenv("E2E_DIALECT"), logs: &syncBuffer{}}
	if st.dialect == "" {
		st.dialect = "postgres"
	}
	ctx := context.Background()

	bin, err := buildServer()
	if err != nil {
		return st, err
	}
	st.cleanups = append(st.cleanups, func() { _ = os.RemoveAll(filepath.Dir(bin)) })

	serviceDSN, err := st.startDatabase(ctx)
	if err != nil {
		return st, err
	}
	if st.natsURL, err = st.startNATS(ctx); err != nil {
		return st, err
	}

	st.stubs = stubs.NewServers()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return st, err
	}
	go func() { _ = st.stubs.Serve(lis) }()
	st.cleanups = append(st.cleanups, st.stubs.Stop)
	stubAddr := lis.Addr().String()

	st.serviceDSN, st.stubAddr, st.binary = serviceDSN, stubAddr, bin
	main, err := st.startInstance(map[string]string{"REQUEST_FLOW_ENABLED": "true"})
	if err != nil {
		return st, err
	}
	st.grpcAddr, st.httpPort, st.proc = main.grpcAddr, main.httpPort, main.cmd
	st.logs = main.logs
	return st, nil
}

// instance is one running request-service process.
type instance struct {
	grpcAddr string
	httpPort int
	cmd      *exec.Cmd
	logs     *syncBuffer
}

// startInstance runs another copy of the binary against the same database, NATS and stubs. Overrides replace
// the default environment (for example REQUEST_FLOW_ENABLED=false for the global kill switch).
func (s *stack) startInstance(overrides map[string]string) (*instance, error) {
	grpcPort, httpPort := freePort(), freePort()
	env := map[string]string{
		"GRPC_PORT": fmt.Sprint(grpcPort), "HTTP_PORT": fmt.Sprint(httpPort),
		"DATABASE_DSN": s.serviceDSN, "NATS_URL": s.natsURL,
		"INFRA_FLEET_SERVICE_ADDR": s.stubAddr, "PROJECT_SERVICE_ADDR": s.stubAddr, "AUTH_SERVICE_ADDR": s.stubAddr,
		"SERVICE_INTERNAL_TOKEN": serviceToken, "GATEWAY_INTERNAL_TOKEN": serviceToken,
		"OPA_BUNDLE_PATH":                 opaBundlePath(),
		"REQUEST_APPROVAL_SWEEP_INTERVAL": "1s", "REQUEST_METRICS_SAMPLE_INTERVAL": "1s",
	}
	for k, v := range overrides {
		env[k] = v
	}
	cmd := exec.Command(s.binary)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	logs := &syncBuffer{}
	cmd.Stdout, cmd.Stderr = logs, logs
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	in := &instance{grpcAddr: fmt.Sprintf("127.0.0.1:%d", grpcPort), httpPort: httpPort, cmd: cmd, logs: logs}
	if err := waitReady(httpPort, logs, 60*time.Second); err != nil {
		in.stop()
		return nil, err
	}
	return in, nil
}

func (in *instance) stop() {
	if in.cmd == nil || in.cmd.Process == nil {
		return
	}
	_ = in.cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() { _ = in.cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		_ = in.cmd.Process.Kill()
	}
}

func buildServer() (string, error) {
	dir, err := os.MkdirTemp("", "request-e2e-")
	if err != nil {
		return "", err
	}
	bin := filepath.Join(dir, "request-service")
	out, err := exec.Command("go", "build", "-o", bin, "../cmd/server").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go build ../cmd/server: %v\n%s", err, out)
	}
	return bin, nil
}

// opaBundlePath is absolute because the binary runs with the e2e directory as its working directory.
func opaBundlePath() string {
	p, err := filepath.Abs("../../../policy/orca-authz")
	if err != nil {
		panic(err)
	}
	return p
}

func freePort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

func waitReady(httpPort int, logs *syncBuffer, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/readyz", httpPort))
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("request-service never became ready; log:\n%s", tail(logs, 40))
}

func tail(b *syncBuffer, n int) string {
	lines := strings.Split(strings.TrimSpace(b.String()), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func (s *stack) logTail(n int) string { return tail(s.logs, n) }

func (s *stack) stop() {
	if s.proc != nil {
		(&instance{cmd: s.proc}).stop()
	}
	for i := len(s.cleanups) - 1; i >= 0; i-- {
		s.cleanups[i]()
	}
}

func labels() map[string]string { return map[string]string{"rf-owner": e2eOwner} }

func (s *stack) startNATS(ctx context.Context) (string, error) {
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "nats:2.10-alpine", Cmd: []string{"-js"}, ExposedPorts: []string{"4222/tcp"}, Labels: labels(),
			WaitingFor: wait.ForLog("Server is ready"),
		},
		Started: true,
	})
	if err != nil {
		return "", fmt.Errorf("start nats: %w", err)
	}
	s.cleanups = append(s.cleanups, func() { _ = c.Terminate(context.Background()) })
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "4222")
	return fmt.Sprintf("nats://%s:%s", host, port.Port()), nil
}

// startDatabase starts the dialect's container, applies every migration and returns the DSN the service uses.
// Postgres: the service connects as a NOSUPERUSER NOBYPASSRLS role so row level security is really in force.
func (s *stack) startDatabase(ctx context.Context) (string, error) {
	switch s.dialect {
	case "postgres":
		return s.startPostgres(ctx)
	case "mysql":
		return s.startMySQL(ctx)
	}
	return "", fmt.Errorf("E2E_DIALECT must be postgres or mysql, got %q", s.dialect)
}

func (s *stack) startPostgres(ctx context.Context) (string, error) {
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "postgres:16-alpine", ExposedPorts: []string{"5432/tcp"}, Labels: labels(),
			Env:        map[string]string{"POSTGRES_USER": "orca", "POSTGRES_PASSWORD": "orca", "POSTGRES_DB": "request"},
			WaitingFor: wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		},
		Started: true,
	})
	if err != nil {
		return "", fmt.Errorf("start postgres: %w", err)
	}
	s.cleanups = append(s.cleanups, func() { _ = c.Terminate(context.Background()) })
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "5432")
	adminDSN := fmt.Sprintf("postgres://orca:orca@%s:%s/request?sslmode=disable", host, port.Port())
	admin, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		return "", err
	}
	s.cleanups = append(s.cleanups, func() { _ = admin.Close(context.Background()) })
	scripts, err := migrationScripts("postgres")
	if err != nil {
		return "", err
	}
	for i, sc := range scripts {
		if _, err := admin.Exec(ctx, sc); err != nil {
			return "", fmt.Errorf("postgres migration %d: %w", i, err)
		}
	}
	for _, stmt := range []string{
		`CREATE ROLE request_app LOGIN PASSWORD 'app_pw' NOSUPERUSER NOBYPASSRLS`,
		`GRANT USAGE ON SCHEMA request TO request_app`,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA request TO request_app`,
	} {
		if _, err := admin.Exec(ctx, stmt); err != nil {
			return "", fmt.Errorf("%s: %w", stmt, err)
		}
	}
	s.db = &pgReader{conn: admin}
	u, _ := url.Parse(adminDSN)
	u.User = url.UserPassword("request_app", "app_pw")
	return u.String(), nil
}

func (s *stack) startMySQL(ctx context.Context) (string, error) {
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "mysql:8.0", ExposedPorts: []string{"3306/tcp"}, Labels: labels(),
			Env: map[string]string{"MYSQL_ROOT_PASSWORD": "orca", "MYSQL_DATABASE": "request"},
			WaitingFor: wait.ForSQL("3306/tcp", "mysql", func(host string, port network.Port) string {
				return fmt.Sprintf("root:orca@tcp(%s:%s)/request", host, port.Port())
			}).WithStartupTimeout(3 * time.Minute).WithPollInterval(time.Second),
		},
		Started: true,
	})
	if err != nil {
		return "", fmt.Errorf("start mysql: %w", err)
	}
	s.cleanups = append(s.cleanups, func() { _ = c.Terminate(context.Background()) })
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "3306")
	addr := fmt.Sprintf("%s:%s", host, port.Port())
	db, err := sql.Open("mysql", fmt.Sprintf("root:orca@tcp(%s)/request?parseTime=true&multiStatements=true&loc=UTC", addr))
	if err != nil {
		return "", err
	}
	s.cleanups = append(s.cleanups, func() { _ = db.Close() })
	scripts, err := migrationScripts("mysql")
	if err != nil {
		return "", err
	}
	for i, sc := range scripts {
		if _, err := db.ExecContext(ctx, sc); err != nil {
			return "", fmt.Errorf("mysql migration %d: %w", i, err)
		}
	}
	s.db = &myReader{db: db}
	return fmt.Sprintf("mysql://root:orca@%s/request", addr), nil
}

// migrationScripts reads the service's up migrations in version order.
func migrationScripts(dialect string) ([]string, error) {
	dir := filepath.Join("..", "migrations", dialect)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	out := make([]string, 0, len(names))
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return nil, err
		}
		out = append(out, string(b))
	}
	return out, nil
}

type pgReader struct {
	mu   sync.Mutex
	conn *pgx.Conn
}

func (r *pgReader) outboxSubjects(t *testing.T, tenantID string) []string {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	rows, err := r.conn.Query(context.Background(), `SELECT subject FROM request.outbox_events WHERE tenant_id = $1 ORDER BY seq`, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func (r *pgReader) outboxPayloads(t *testing.T, tenantID, subject string) []string {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	rows, err := r.conn.Query(context.Background(), `SELECT payload::text FROM request.outbox_events WHERE tenant_id = $1 AND subject = $2 ORDER BY seq`, tenantID, subject)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func (r *pgReader) requestStatus(t *testing.T, id string) string {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	var s string
	if err := r.conn.QueryRow(context.Background(), `SELECT status FROM request.requests WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

type myReader struct{ db *sql.DB }

func (r *myReader) outboxSubjects(t *testing.T, tenantID string) []string {
	t.Helper()
	rows, err := r.db.Query(`SELECT subject FROM outbox_events WHERE tenant_id = ? ORDER BY seq`, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func (r *myReader) outboxPayloads(t *testing.T, tenantID, subject string) []string {
	t.Helper()
	rows, err := r.db.Query(`SELECT CAST(payload AS CHAR) FROM outbox_events WHERE tenant_id = ? AND subject = ? ORDER BY seq`, tenantID, subject)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func (r *myReader) requestStatus(t *testing.T, id string) string {
	t.Helper()
	var s string
	if err := r.db.QueryRow(`SELECT status FROM requests WHERE id = ?`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func (r *pgReader) renameTenantSettings(t *testing.T, to string) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	from := "tenant_settings"
	if to == "tenant_settings" {
		from = "tenant_settings_unreadable"
	}
	if _, err := r.conn.Exec(context.Background(), fmt.Sprintf(`ALTER TABLE request.%s RENAME TO %s`, from, to)); err != nil {
		t.Fatal(err)
	}
}

func (r *myReader) renameTenantSettings(t *testing.T, to string) {
	t.Helper()
	from := "tenant_settings"
	if to == "tenant_settings" {
		from = "tenant_settings_unreadable"
	}
	if _, err := r.db.Exec(fmt.Sprintf(`RENAME TABLE %s TO %s`, from, to)); err != nil {
		t.Fatal(err)
	}
}

// syncBuffer collects a child's output; exec writes it from its own goroutine while tests read it.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}
