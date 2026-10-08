//go:build integration

package postgres

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func (f *pgFixture) rawInsert(table string, defaults, overrides map[string]any) error {
	cols := []string{}
	vals := []any{}
	ph := []string{}
	merged := map[string]any{}
	for k, v := range defaults {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	for k, v := range merged {
		cols = append(cols, k)
		vals = append(vals, v)
		ph = append(ph, fmt.Sprintf("$%d", len(vals)))
	}
	_, err := f.admin.Exec(context.Background(), "INSERT INTO request."+table+" ("+strings.Join(cols, ", ")+") VALUES ("+strings.Join(ph, ", ")+")", vals...)
	return err
}

func (f *pgFixture) executionEnv() contracttest.ExecutionEnv {
	base := New(f.app)
	executing := NewExecutingRequestRepository(base)
	return contracttest.ExecutionEnv{
		LifecycleEnv:  f.lifecycleEnv(),
		Approvals:     NewApprovalRepository(base),
		GateApprovals: NewApprovalGateReader(base),
		PhaseStarts:   NewPhaseStartRepository(base),
		Outcomes:      NewTaskRunOutcomeRepository(base),
		Checks:        NewRequestCheckRepository(base),
		Scanner:       executing,
		Leases:        executing,
		Backlog:       NewBacklogRequestReader(base),
		Processed:     NewProcessedEventRepository(base),
		InsertRawOutcome: func(tenantID string, ov map[string]any) error {
			return f.rawInsert("task_run_outcomes", map[string]any{
				"id": uuid.NewString(), "tenant_id": tenantID, "task_id": uuid.NewString(), "event_id": uuid.NewString(), "occurred_at": "2026-10-01T00:00:00Z",
			}, ov)
		},
		InsertRawCheck: func(tenantID string, ov map[string]any) error {
			return f.rawInsert("request_checks", map[string]any{
				"id": uuid.NewString(), "tenant_id": tenantID, "kind": "ops_result", "status": "passed", "source": "manual",
			}, ov)
		},
	}
}

func TestPostgres_ExecutionRepositoryContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunExecutionRepositoryContract(t, func(*testing.T) contracttest.ExecutionEnv { return f.executionEnv() })
}

// The reconcile scan reads other tenants' rows under the relay setting only; it must never be able to write them.
func TestPostgres_ExecutionRelayPoliciesAreReadOnly(t *testing.T) {
	f := newMigratedPostgres(t)
	ctx := context.Background()
	for _, table := range []string{"requests", "task_run_outcomes", "execution_reconcile_state"} {
		var cmd string
		err := f.admin.QueryRow(ctx, `SELECT cmd FROM pg_policies WHERE schemaname = 'request' AND tablename = $1 AND policyname = 'relay_scan'`, table).Scan(&cmd)
		if err != nil || cmd != "SELECT" {
			t.Errorf("%s: relay_scan must be a SELECT-only policy, got %q (%v)", table, cmd, err)
		}
	}
}

// The app role is NOBYPASSRLS: without the tenant GUC a repository call must see nothing, not everything.
func TestPostgres_ExecutionTablesUnderRLS(t *testing.T) {
	f := newMigratedPostgres(t)
	ctx := context.Background()
	tenantA := uuid.NewString()
	if err := f.rawInsert("phase_starts", map[string]any{"phase_task_id": uuid.NewString(), "request_id": uuid.NewString(), "started_by": uuid.NewString()}, map[string]any{"tenant_id": tenantA}); err != nil {
		t.Fatal(err)
	}
	if err := f.rawInsert("request_checks", map[string]any{"id": uuid.NewString(), "request_id": uuid.NewString(), "kind": "ops_result", "status": "passed", "source": "manual"}, map[string]any{"tenant_id": tenantA}); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"phase_starts", "request_checks", "task_run_outcomes", "execution_reconcile_state"} {
		var n int
		if err := f.app.QueryRow(ctx, "SELECT count(*) FROM request."+table).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s: a connection without the tenant setting must see no rows, saw %d (%v)", table, n, err)
		}
		tx, err := f.app.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, uuid.NewString())
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM request."+table).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s: another tenant's setting must see no rows, saw %d (%v)", table, n, err)
		}
		_ = tx.Rollback(ctx)
	}
	tx, err := f.app.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, _ = tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantA)
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM request.phase_starts`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("the owning tenant sees its row: %d %v", n, err)
	}
	// WITH CHECK: a tenant cannot write a row for another tenant.
	_, err = tx.Exec(ctx, `INSERT INTO request.phase_starts (tenant_id, phase_task_id, request_id, started_by) VALUES ($1, $2, $3, $4)`,
		uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString())
	if err == nil || !strings.Contains(err.Error(), "row-level security") {
		t.Fatalf("cross-tenant insert must be rejected by RLS, got %v", err)
	}
	var _ pgx.Tx = tx
}

func TestPostgres_ExecutionMigration_UpDownUp(t *testing.T) {
	_, admin := startPostgres(t)
	ups := contracttest.MigrationScripts(t, "postgres", "up")
	downs := contracttest.MigrationScripts(t, "postgres", "down")
	applyScripts(t, admin, ups)
	for _, table := range []string{"phase_starts", "task_run_outcomes", "execution_reconcile_state", "request_checks"} {
		if !tableExists(t, admin, table) {
			t.Fatalf("%s missing after up", table)
		}
	}
	assertAllTablesForceRLS(t, admin)
	applyScripts(t, admin, downs)
	applyScripts(t, admin, ups)
	assertAllTablesForceRLS(t, admin)
	// Down of just the two execution migrations (0051 and 0050) leaves the earlier schema usable.
	i51 := contracttest.MigrationScriptIndex(t, "postgres", "down", "0051_")
	i50 := contracttest.MigrationScriptIndex(t, "postgres", "down", "0050_")
	applyScripts(t, admin, downs[i51:i50+1])
	for _, table := range []string{"phase_starts", "task_run_outcomes", "execution_reconcile_state", "request_checks"} {
		if tableExists(t, admin, table) {
			t.Fatalf("%s should be gone after its down", table)
		}
	}
	if !tableExists(t, admin, "requests") || !tableExists(t, admin, "approvals") {
		t.Fatal("earlier tables must survive")
	}
	var policies int
	if err := admin.QueryRow(context.Background(), `SELECT count(*) FROM pg_policies WHERE schemaname='request' AND tablename='requests' AND policyname='relay_scan'`).Scan(&policies); err != nil || policies != 0 {
		t.Fatalf("down must drop the relay policy on requests: %d %v", policies, err)
	}
}

func TestPostgres_ExecutionFlowContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunExecutionFlowContract(t, func(*testing.T) contracttest.ExecutionEnv { return f.executionEnv() })
}
