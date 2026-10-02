//go:build integration

package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

const (
	tenantA = "aaaaaaaa-0000-4000-8000-000000000001"
	tenantB = "bbbbbbbb-0000-4000-8000-000000000002"
	appRole = "mcp_app"
	appPass = "mcp_app_pw"
)

func readMigration(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "postgres", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// execScript sends a multi-statement script (simple protocol: no args).
func execScript(t *testing.T, ctx context.Context, conn *pgx.Conn, sql string) {
	t.Helper()
	if _, err := conn.Exec(ctx, sql); err != nil {
		t.Fatalf("exec script: %v", err)
	}
}

func adminConn(t *testing.T, dsn string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}

func schemaExists(t *testing.T, ctx context.Context, conn *pgx.Conn) bool {
	t.Helper()
	var n int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM information_schema.schemata WHERE schema_name = 'mcp'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func TestMigration_UpDownUp(t *testing.T) {
	dsn := testutil.StartPostgres(t, "mcp")
	ctx := context.Background()
	conn := adminConn(t, dsn)
	up, down := readMigration(t, "0001_init.up.sql"), readMigration(t, "0001_init.down.sql")

	execScript(t, ctx, conn, up)
	if !schemaExists(t, ctx, conn) {
		t.Fatal("schema missing after up")
	}
	execScript(t, ctx, conn, down)
	if schemaExists(t, ctx, conn) {
		t.Fatal("schema still present after down")
	}
	execScript(t, ctx, conn, up)
	if !schemaExists(t, ctx, conn) {
		t.Fatal("schema missing after second up")
	}
}

// setup migrates as the owner/superuser, then returns repositories over two
// pools: the non-superuser app role (RLS applies) and the superuser (for
// raw inspection only).
func setup(t *testing.T) (repo *Repository, appPool, adminPool *pgxpool.Pool) {
	t.Helper()
	dsn := testutil.StartPostgres(t, "mcp")
	ctx := context.Background()
	conn := adminConn(t, dsn)
	execScript(t, ctx, conn, readMigration(t, "0001_init.up.sql"))
	execScript(t, ctx, conn, fmt.Sprintf(`
		CREATE ROLE %[1]s LOGIN PASSWORD '%[2]s' NOSUPERUSER NOBYPASSRLS;
		GRANT USAGE ON SCHEMA mcp TO %[1]s;
		GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA mcp TO %[1]s;`, appRole, appPass))

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	adminPool, err = pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(adminPool.Close)

	// Parse again: pool configs share their ConnConfig pointer, so mutating
	// the first would silently turn the admin pool into the app role.
	appCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	appCfg.ConnConfig.User, appCfg.ConnConfig.Password = appRole, appPass
	appPool, err = pgxpool.NewWithConfig(ctx, appCfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(appPool.Close)
	return New(appPool), appPool, adminPool
}

func TestAppRoleIsNotExemptFromRLS(t *testing.T) {
	_, appPool, _ := setup(t)
	var super, bypass bool
	if err := appPool.QueryRow(context.Background(),
		`SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(&super, &bypass); err != nil {
		t.Fatal(err)
	}
	if super || bypass {
		t.Fatalf("test role must not bypass RLS (super=%v bypass=%v)", super, bypass)
	}
}

func TestRLS_TenantCannotSeeOrWriteOtherTenantsRows(t *testing.T) {
	repo, appPool, _ := setup(t)
	ctx := context.Background()

	if _, err := repo.GetOrCreateTenantSettings(ctx, domain.DefaultTenantSettings(tenantA, true, 90)); err != nil {
		t.Fatal(err)
	}

	count := func(tenantCtx string) int {
		var n int
		err := (&Repository{pool: appPool}).withTenantTx(ctx, tenantCtx, func(tx pgx.Tx) error {
			// No WHERE tenant_id: only RLS can filter.
			return tx.QueryRow(ctx, `SELECT count(*) FROM mcp.tenant_settings`).Scan(&n)
		})
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if got := count(tenantA); got != 1 {
		t.Fatalf("tenant A sees %d rows, want 1", got)
	}
	if got := count(tenantB); got != 0 {
		t.Fatalf("tenant B sees %d rows of tenant A, want 0", got)
	}

	// WITH CHECK: app.tenant_id = B cannot insert a row for A.
	err := (&Repository{pool: appPool}).withTenantTx(ctx, tenantB, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO mcp.tenant_settings (tenant_id) VALUES ($1)`, tenantA)
		return err
	})
	if err == nil {
		t.Fatal("cross-tenant insert must be rejected by WITH CHECK")
	}

	// B cannot update or delete A's row either (0 rows affected, no error).
	_ = (&Repository{pool: appPool}).withTenantTx(ctx, tenantB, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE mcp.tenant_settings SET enabled = false`)
		if err != nil || tag.RowsAffected() != 0 {
			t.Errorf("cross-tenant update affected %d rows (err=%v)", tag.RowsAffected(), err)
		}
		tag, err = tx.Exec(ctx, `DELETE FROM mcp.tenant_settings`)
		if err != nil || tag.RowsAffected() != 0 {
			t.Errorf("cross-tenant delete affected %d rows (err=%v)", tag.RowsAffected(), err)
		}
		return nil
	})
}

func TestRLS_NoTenantContextSeesNothingAndDoesNotError(t *testing.T) {
	repo, appPool, _ := setup(t)
	ctx := context.Background()
	if _, err := repo.GetOrCreateTenantSettings(ctx, domain.DefaultTenantSettings(tenantA, true, 90)); err != nil {
		t.Fatal(err)
	}
	// Run twice on the same single connection so the second query sees the
	// '' left behind by a finished transaction rather than NULL.
	conn, err := appPool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	for i := 0; i < 2; i++ {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			_, _ = tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantA)
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM mcp.tenant_settings`).Scan(&n); err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		_ = tx.Rollback(ctx)
		if (i == 0 && n != 1) || (i == 1 && n != 0) {
			t.Fatalf("iteration %d: saw %d rows", i, n)
		}
	}
}

func TestRLS_ForcedOnAllTables(t *testing.T) {
	_, _, adminPool := setup(t)
	rows, err := adminPool.Query(context.Background(), `
		SELECT c.relname, c.relrowsecurity, c.relforcerowsecurity
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'mcp' AND c.relkind = 'r'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var name string
		var enabled, forced bool
		if err := rows.Scan(&name, &enabled, &forced); err != nil {
			t.Fatal(err)
		}
		if !enabled || !forced {
			t.Errorf("table %s: rls enabled=%v forced=%v", name, enabled, forced)
		}
		seen++
	}
	if seen != 3 {
		t.Fatalf("expected 3 tables, saw %d", seen)
	}
}

func TestGetOrCreate_LazyDefaultAndNoOverwrite(t *testing.T) {
	repo, _, _ := setup(t)
	ctx := context.Background()

	got, err := repo.GetOrCreateTenantSettings(ctx, domain.DefaultTenantSettings(tenantA, false, 30))
	if err != nil {
		t.Fatal(err)
	}
	if got.Enabled || got.MaxTokenDays != 30 || got.ApprovalTTLSeconds != 600 || got.KillSwitch.Active || got.UpdatedAt.IsZero() {
		t.Fatalf("unexpected defaults: %+v", got)
	}
	// Different defaults later must not change the stored row.
	again, err := repo.GetOrCreateTenantSettings(ctx, domain.DefaultTenantSettings(tenantA, true, 90))
	if err != nil {
		t.Fatal(err)
	}
	if again.Enabled || again.MaxTokenDays != 30 {
		t.Fatalf("existing row was overwritten: %+v", again)
	}
}

func TestGetOrCreate_ConcurrentFirstCallsConverge(t *testing.T) {
	repo, _, adminPool := setup(t)
	ctx := context.Background()
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			_, err := repo.GetOrCreateTenantSettings(ctx, domain.DefaultTenantSettings(tenantA, true, 90))
			errs <- err
		}()
	}
	for i := 0; i < 8; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := adminPool.QueryRow(ctx, `SELECT count(*) FROM mcp.tenant_settings`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows=%d err=%v", n, err)
	}
}

func TestUpdateTenantSettings_UpsertsAndKeepsKillSwitch(t *testing.T) {
	repo, _, adminPool := setup(t)
	ctx := context.Background()

	if _, err := repo.GetOrCreateTenantSettings(ctx, domain.DefaultTenantSettings(tenantA, true, 90)); err != nil {
		t.Fatal(err)
	}
	if _, err := adminPool.Exec(ctx, `UPDATE mcp.tenant_settings SET kill_switch_active = true, kill_switch_reason = 'x', kill_switch_at = now()`); err != nil {
		t.Fatal(err)
	}

	user := uuid.NewString()
	s := domain.DefaultTenantSettings(tenantA, false, 14)
	s.DCREnabled, s.ApprovalTTLSeconds, s.UpdatedBy = true, 120, user
	got, err := repo.UpdateTenantSettings(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if got.Enabled || !got.DCREnabled || got.MaxTokenDays != 14 || got.ApprovalTTLSeconds != 120 || got.UpdatedBy != user {
		t.Fatalf("not updated: %+v", got)
	}
	if !got.KillSwitch.Active || got.KillSwitch.Reason != "x" || got.KillSwitch.At == nil {
		t.Fatalf("kill switch must be untouched: %+v", got.KillSwitch)
	}

	// Insert path (no prior row) and CHECK violation surfaces as an error.
	if _, err := repo.UpdateTenantSettings(ctx, domain.DefaultTenantSettings(tenantB, true, 90)); err != nil {
		t.Fatal(err)
	}
	bad := domain.DefaultTenantSettings(tenantB, true, 91)
	if _, err := repo.UpdateTenantSettings(ctx, bad); err == nil {
		t.Fatal("CHECK constraint should reject max_token_days=91")
	}
}

func TestOutbox_EnqueueIsTenantScopedAndRelayReadsAcrossTenants(t *testing.T) {
	repo, appPool, _ := setup(t)
	ctx := context.Background()

	idA, idB := uuid.NewString(), uuid.NewString()
	for tenantID, id := range map[string]string{tenantA: idA, tenantB: idB} {
		err := repo.EnqueueOutbox(ctx, tenantID, domain.OutboxRecord{
			ID: id, Subject: "orca.mcp.test.created", OccurredAt: time.Now(), Version: 1, PayloadJSON: []byte(`{"k":"v"}`),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// A normal tenant transaction sees only its own event.
	var own int
	_ = repo.withTenantTx(ctx, tenantA, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM mcp.outbox_events`).Scan(&own)
	})
	if own != 1 {
		t.Fatalf("tenant A sees %d outbox rows, want 1", own)
	}
	// Without either setting the app role sees nothing.
	var bare int
	if err := appPool.QueryRow(ctx, `SELECT count(*) FROM mcp.outbox_events`).Scan(&bare); err != nil || bare != 0 {
		t.Fatalf("unscoped read saw %d rows (err=%v)", bare, err)
	}
	// Tenant cannot enqueue for another tenant.
	err := repo.withTenantTx(ctx, tenantA, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO mcp.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES ($1, $2, 's', now(), 1, '{}')`, uuid.NewString(), tenantB)
		return err
	})
	if err == nil {
		t.Fatal("cross-tenant outbox insert must be rejected")
	}

	recs, err := repo.FetchUnpublished(ctx, 10)
	if err != nil || len(recs) != 2 {
		t.Fatalf("relay fetch: n=%d err=%v", len(recs), err)
	}
	if recs[0].Event.TenantID == "" || recs[0].Subject != "orca.mcp.test.created" || recs[0].Event.ID != recs[0].ID {
		t.Fatalf("bad record mapping: %+v", recs[0])
	}
	if err := repo.MarkPublished(ctx, []string{idA}); err != nil {
		t.Fatal(err)
	}
	recs, err = repo.FetchUnpublished(ctx, 10)
	if err != nil || len(recs) != 1 || recs[0].ID != idB {
		t.Fatalf("after mark: %+v err=%v", recs, err)
	}
}

func TestOutbox_RelayPolicyCannotInsertOrDelete(t *testing.T) {
	repo, _, _ := setup(t)
	ctx := context.Background()
	err := repo.withRelayTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO mcp.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES ($1, $2, 's', now(), 1, '{}')`, uuid.NewString(), tenantA)
		return err
	})
	if err == nil {
		t.Fatal("relay mode must not allow INSERT")
	}
}
