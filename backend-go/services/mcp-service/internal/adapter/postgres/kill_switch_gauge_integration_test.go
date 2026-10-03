//go:build integration

package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

func setupKillGauge(t *testing.T) (*Repository, *pgxpool.Pool) {
	t.Helper()
	dsn := testutil.StartPostgres(t, "mcp")
	ctx := context.Background()
	conn := adminConn(t, dsn)
	for _, m := range append(append([]string{}, allMigrations...), "0008_kill_switch_gauge") {
		execScript(t, ctx, conn, readMigration(t, m+".up.sql"))
	}
	execScript(t, ctx, conn, fmt.Sprintf(`
		CREATE ROLE %[1]s LOGIN PASSWORD '%[2]s' NOSUPERUSER NOBYPASSRLS;
		GRANT USAGE ON SCHEMA mcp TO %[1]s;
		GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA mcp TO %[1]s;`, appRole, appPass))
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.User, cfg.ConnConfig.Password = appRole, appPass
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return New(p), p
}

func TestKillSwitchGauge_CountsAcrossTenantsWithoutLeakingRows(t *testing.T) {
	r, pool := setupKillGauge(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for _, tn := range []string{tenantA, tenantB} {
		_, _ = r.LoadPolicySnapshot(ctx, domain.DefaultTenantSettings(tn, true, 90))
	}
	set := func(tn, scope, target string, active bool) {
		t.Helper()
		e := domain.KillSwitchEntry{ID: uuid.NewString(), TenantID: tn, Scope: scope, TargetID: target, Reason: "incident", Active: active, SetBy: userA1, SetAt: now}
		if _, err := r.UpsertKillSwitch(ctx, e, nil); err != nil {
			t.Fatal(err)
		}
	}
	set(tenantA, "tenant", "", true)
	set(tenantB, "tenant", "", true)
	set(tenantA, "session", "s-1", true)
	set(tenantA, "session", "s-2", false) // inactive switches are not counted

	got, err := r.CountActiveKillSwitches(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got["tenant"] != 2 || got["session"] != 1 || len(got) != 2 {
		t.Fatalf("counts = %v", got)
	}
	// The policy is relay-only and read-only: an ordinary connection sees nothing.
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM mcp.kill_switches`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("no tenant context must see no rows: n=%d err=%v", n, err)
	}
}

// Turning a tenant switch off still owes the PAT restore, so it stays in the
// cleanup queue; other scopes are done once they are off.
func TestKillSwitchCleanup_TenantOffStaysPendingForPATRestore(t *testing.T) {
	r, _ := setupKillGauge(t)
	ctx := context.Background()
	now := time.Now().UTC()
	_, _ = r.LoadPolicySnapshot(ctx, domain.DefaultTenantSettings(tenantA, true, 90))
	tenantSw := domain.KillSwitchEntry{ID: uuid.NewString(), TenantID: tenantA, Scope: "tenant", Reason: "incident", Active: true, SetBy: userA1, SetAt: now}
	if _, err := r.UpsertKillSwitch(ctx, tenantSw, nil); err != nil {
		t.Fatal(err)
	}
	p, _ := r.PendingKillCleanups(ctx, 10)
	if len(p) != 1 || !p[0].Active {
		t.Fatalf("active switch pending: %+v", p)
	}
	_ = r.ClearKillCleanup(ctx, tenantA, p[0].ID)

	tenantSw.Active, tenantSw.Reason = false, "all clear"
	if _, err := r.UpsertKillSwitch(ctx, tenantSw, nil); err != nil {
		t.Fatal(err)
	}
	p, _ = r.PendingKillCleanups(ctx, 10)
	if len(p) != 1 || p[0].Active || p[0].Scope != "tenant" {
		t.Fatalf("tenant switch off must be pending for the PAT restore: %+v", p)
	}
	_ = r.ClearKillCleanup(ctx, tenantA, p[0].ID)

	sess := domain.KillSwitchEntry{ID: uuid.NewString(), TenantID: tenantA, Scope: "session", TargetID: "s-1", Reason: "one", Active: true, SetBy: userA1, SetAt: now}
	_, _ = r.UpsertKillSwitch(ctx, sess, nil)
	p, _ = r.PendingKillCleanups(ctx, 10)
	_ = r.ClearKillCleanup(ctx, tenantA, p[0].ID)
	sess.Active = false
	_, _ = r.UpsertKillSwitch(ctx, sess, nil)
	if p, _ := r.PendingKillCleanups(ctx, 10); len(p) != 0 {
		t.Fatalf("non-tenant switch off owes nothing: %+v", p)
	}
}
