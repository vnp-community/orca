//go:build integration

package mysql

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func (f *myFixture) rawInsert(table string, defaults, overrides map[string]any) error {
	merged := map[string]any{}
	for k, v := range defaults {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	var cols, ph []string
	var vals []any
	for k, v := range merged {
		cols, ph, vals = append(cols, k), append(ph, "?"), append(vals, v)
	}
	_, err := f.admin.Exec(fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ", "), strings.Join(ph, ", ")), vals...)
	return err
}

func (f *myFixture) executionEnv() contracttest.ExecutionEnv {
	base := New(f.db)
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
				"id": uuid.NewString(), "tenant_id": tenantID, "task_id": uuid.NewString(), "error_message": "", "event_id": uuid.NewString(), "occurred_at": "2026-10-01 00:00:00",
			}, ov)
		},
		InsertRawCheck: func(tenantID string, ov map[string]any) error {
			return f.rawInsert("request_checks", map[string]any{
				"id": uuid.NewString(), "tenant_id": tenantID, "kind": "ops_result", "status": "passed", "source": "manual", "metrics": "{}", "summary": "",
			}, ov)
		},
	}
}

func TestMySQL_ExecutionRepositoryContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunExecutionRepositoryContract(t, func(*testing.T) contracttest.ExecutionEnv { return f.executionEnv() })
}

func TestMySQL_ExecutionMigration_UpDownUp(t *testing.T) {
	_, admin := startMySQL(t)
	ups := contracttest.MigrationScripts(t, "mysql", "up")
	downs := contracttest.MigrationScripts(t, "mysql", "down")
	has := func(table string) bool {
		var n int
		if err := admin.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`, table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 1
	}
	applyScripts(t, admin, ups)
	for _, table := range []string{"phase_starts", "task_run_outcomes", "execution_reconcile_state", "request_checks"} {
		if !has(table) {
			t.Fatalf("%s missing after up", table)
		}
	}
	applyScripts(t, admin, downs)
	applyScripts(t, admin, ups)
	i51 := contracttest.MigrationScriptIndex(t, "mysql", "down", "0051_")
	i50 := contracttest.MigrationScriptIndex(t, "mysql", "down", "0050_")
	applyScripts(t, admin, downs[i51:i50+1])
	for _, table := range []string{"phase_starts", "task_run_outcomes", "execution_reconcile_state", "request_checks"} {
		if has(table) {
			t.Fatalf("%s should be gone after its down", table)
		}
	}
	if !has("requests") || !has("approvals") {
		t.Fatal("earlier tables must survive")
	}
	var idx int
	if err := admin.QueryRow(`SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'requests' AND index_name = 'requests_executing_scan'`).Scan(&idx); err != nil || idx != 0 {
		t.Fatalf("down must drop the scan index: %d %v", idx, err)
	}
	_ = context.Background
}

func TestMySQL_ExecutionFlowContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunExecutionFlowContract(t, func(*testing.T) contracttest.ExecutionEnv { return f.executionEnv() })
}
