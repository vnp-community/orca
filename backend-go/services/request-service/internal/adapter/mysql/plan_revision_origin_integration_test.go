//go:build integration

package mysql

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func TestMySQL_PlanRevisionFailureContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunPlanRevisionFailureContract(t, contracttest.PlanRevisionFailureEnv{
		SolutionEnv: f.solutionEnv(), Jobs: NewPlanAutoJobRepository(New(f.db)),
	})
}

func TestMySQL_PlanRevisionReadContract(t *testing.T) {
	f := newMigratedMySQL(t)
	base := New(f.db)
	contracttest.RunPlanRevisionReadContract(t, contracttest.PlanRevisionEnv{
		SolutionEnv: f.solutionEnv(), Changes: NewOpenSpecChangeRepository(base), Revisions: NewOpenSpecChangeRevisionRepository(base),
	})
}

// Listing the successful Plan runs of a Request reads the (tenant, request) key, not the table.
func TestMySQL_ListSucceededPlanRunsCanUseAnIndex(t *testing.T) {
	f := newMigratedMySQL(t)
	reqID, tenantID := uuid.NewString(), uuid.NewString()
	if err := insertRawRequest(f.admin, map[string]any{"id": reqID, "tenant_id": tenantID}); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 30; n++ {
		if _, err := f.admin.Exec(`INSERT INTO analysis_runs (id, tenant_id, request_id, kind, mode, status) VALUES (?, ?, ?, 'plan', 'complete', 'succeeded')`, uuid.NewString(), tenantID, reqID); err != nil {
			t.Fatal(err)
		}
	}
	var id, selectType, table, partitions, typ, possible, key, keyLen, ref, rows, filtered, extra *string
	err := f.admin.QueryRow(`EXPLAIN SELECT id FROM analysis_runs WHERE tenant_id = ? AND request_id = ? AND kind = 'plan' AND status = 'succeeded' ORDER BY started_at ASC, id`, tenantID, reqID).
		Scan(&id, &selectType, &table, &partitions, &typ, &possible, &key, &keyLen, &ref, &rows, &filtered, &extra)
	if err != nil {
		t.Fatal(err)
	}
	if possible == nil || !strings.Contains(*possible, "analysis_runs_idem") {
		t.Fatalf("the listing must be able to use the (tenant, request) key: possible_keys=%v", possible)
	}
}

// Migration 0104 lets a job carry the origin 'revision'; going back removes those jobs and keeps the others.
func TestMySQL_Migration_PlanAutoJobsRevisionOrigin(t *testing.T) {
	_, admin := startMySQL(t)
	before, target := contracttest.MigrationUpScriptsSplit(t, "mysql", "0104")
	applyScripts(t, admin, before)
	reqID, tenantID := uuid.NewString(), uuid.NewString()
	if err := insertRawRequest(admin, map[string]any{"id": reqID, "tenant_id": tenantID}); err != nil {
		t.Fatal(err)
	}
	insert := func(origin, anchor, state string) error {
		_, err := admin.Exec(`INSERT INTO plan_auto_jobs (tenant_id, request_id, anchor, origin, project_id, state) VALUES (?, ?, ?, ?, ?, ?)`,
			tenantID, reqID, anchor, origin, uuid.NewString(), state)
		return err
	}
	if err := insert("event", "sol-1", "done"); err != nil {
		t.Fatal(err)
	}
	applyScripts(t, admin, []string{target})
	if err := insert("revision", "rev-1", "done"); err != nil {
		t.Fatalf("after 0104: %v", err)
	}
	if err := insert("manual", "x-1", "done"); err == nil {
		t.Fatal("an unknown origin must still be refused")
	}

	idx := contracttest.MigrationScriptIndex(t, "mysql", "down", "0104_")
	applyScripts(t, admin, contracttest.MigrationScripts(t, "mysql", "down")[idx:idx+1])
	var revisions, others int
	if err := admin.QueryRow(`SELECT COALESCE(SUM(origin = 'revision'), 0), COALESCE(SUM(origin <> 'revision'), 0) FROM plan_auto_jobs`).Scan(&revisions, &others); err != nil {
		t.Fatal(err)
	}
	if revisions != 0 || others != 1 {
		t.Fatalf("down must remove the revision jobs only: %d revision, %d others", revisions, others)
	}
	if err := insert("revision", "rev-2", "done"); err == nil {
		t.Fatal("after down the origin 'revision' must be refused again")
	}
	applyScripts(t, admin, []string{target})
	if err := insert("revision", "rev-3", "done"); err != nil {
		t.Fatalf("re-applied 0104: %v", err)
	}
}
