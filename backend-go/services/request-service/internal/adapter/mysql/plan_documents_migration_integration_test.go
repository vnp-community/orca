//go:build integration

package mysql

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

// A change stored before 0102 reads back with no masked secrets, and 0102 can be reversed and re-applied without
// touching the row.
func TestMySQL_Migration_OpenSpecChangeRedaction(t *testing.T) {
	_, admin := startMySQL(t)
	before, target := contracttest.MigrationUpScriptsSplit(t, "mysql", "0102")
	applyScripts(t, admin, before)
	reqID, tenantID, changeID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if err := insertRawRequest(admin, map[string]any{"id": reqID, "tenant_id": tenantID}); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(`INSERT INTO openspec_changes (id, tenant_id, request_id, change_id, status, tasks_sync_state)
		VALUES (?, ?, ?, 'req-1-x', 'ready', 'in_sync')`, changeID, tenantID, reqID); err != nil {
		t.Fatal(err)
	}
	applyScripts(t, admin, []string{target})

	var n int
	if err := admin.QueryRow(`SELECT redacted_secrets FROM openspec_changes WHERE id = ?`, changeID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("an old change must read 0: %d %v", n, err)
	}
	idx := contracttest.MigrationScriptIndex(t, "mysql", "down", "0102_")
	applyScripts(t, admin, contracttest.MigrationScripts(t, "mysql", "down")[idx:idx+1])
	var cols int
	if err := admin.QueryRow(`SELECT count(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name='openspec_changes' AND column_name='redacted_secrets'`).Scan(&cols); err != nil || cols != 0 {
		t.Fatalf("down must drop the column: %d %v", cols, err)
	}
	applyScripts(t, admin, []string{target})
	if err := admin.QueryRow(`SELECT redacted_secrets FROM openspec_changes WHERE id = ?`, changeID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("re-applied 0102 must default the existing row to 0: %d %v", n, err)
	}
}

func TestMySQL_PlanRevisionContract(t *testing.T) {
	f := newMigratedMySQL(t)
	base := New(f.db)
	contracttest.RunPlanRevisionContract(t, contracttest.PlanRevisionEnv{
		SolutionEnv: f.solutionEnv(), Changes: NewOpenSpecChangeRepository(base), Revisions: NewOpenSpecChangeRevisionRepository(base),
	})
}

// Migration 0103 can be reversed and re-applied.
func TestMySQL_Migration_OpenSpecChangeRevisions(t *testing.T) {
	_, admin := startMySQL(t)
	before, target := contracttest.MigrationUpScriptsSplit(t, "mysql", "0103")
	applyScripts(t, admin, before)
	applyScripts(t, admin, []string{target})
	idx := contracttest.MigrationScriptIndex(t, "mysql", "down", "0103_")
	applyScripts(t, admin, contracttest.MigrationScripts(t, "mysql", "down")[idx:idx+1])
	var n int
	if err := admin.QueryRow(`SELECT count(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name='openspec_change_revisions'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("down must drop the table: %d %v", n, err)
	}
	applyScripts(t, admin, []string{target})
}

// Listing the versions of a Request with many of them reads an index, not the table.
func TestMySQL_PlanRevisionListUsesAnIndex(t *testing.T) {
	f := newMigratedMySQL(t)
	reqID, tenantID := uuid.NewString(), uuid.NewString()
	if err := insertRawRequest(f.admin, map[string]any{"id": reqID, "tenant_id": tenantID}); err != nil {
		t.Fatal(err)
	}
	for n := 1; n <= 60; n++ {
		if _, err := f.admin.Exec(`INSERT INTO openspec_change_revisions (id, tenant_id, request_id, revision, run_id, bundle_digest, files_json)
			VALUES (?, ?, ?, ?, ?, 'd', '[]')`, uuid.NewString(), tenantID, reqID, n, uuid.NewString()); err != nil {
			t.Fatal(err)
		}
	}
	var id, selectType, table, partitions, typ, possible, key, keyLen, ref, rows, filtered, extra *string
	err := f.admin.QueryRow(`EXPLAIN SELECT id FROM openspec_change_revisions WHERE tenant_id = ? AND request_id = ? AND revision < 40 ORDER BY revision DESC LIMIT 10`,
		tenantID, reqID).Scan(&id, &selectType, &table, &partitions, &typ, &possible, &key, &keyLen, &ref, &rows, &filtered, &extra)
	if err != nil {
		t.Fatal(err)
	}
	if typ == nil || *typ == "ALL" || key == nil || !strings.HasPrefix(*key, "openspec_change_revisions_") {
		t.Fatalf("the listing must use an index: type=%v key=%v", typ, key)
	}
}

// The plan branch of the activity feed filters analysis_runs by tenant and Request, so an index serves it.
func TestMySQL_PlanRevisionActivityBranchUsesAnIndex(t *testing.T) {
	f := newMigratedMySQL(t)
	reqID, tenantID := uuid.NewString(), uuid.NewString()
	if err := insertRawRequest(f.admin, map[string]any{"id": reqID, "tenant_id": tenantID}); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 30; n++ {
		if _, err := f.admin.Exec(`INSERT INTO analysis_runs (id, tenant_id, request_id, kind, mode, status, feedback)
			VALUES (?, ?, ?, 'plan', 'complete', 'succeeded', 'x')`, uuid.NewString(), tenantID, reqID); err != nil {
			t.Fatal(err)
		}
	}
	var id, selectType, table, partitions, typ, possible, key, keyLen, ref, rows, filtered, extra *string
	err := f.admin.QueryRow(`EXPLAIN SELECT id FROM analysis_runs WHERE tenant_id = ? AND request_id = ? AND kind = 'plan'
		AND feedback IS NOT NULL AND feedback <> '' AND feedback NOT LIKE 'clarification:%'`, tenantID, reqID).Scan(&id, &selectType, &table, &partitions, &typ, &possible, &key, &keyLen, &ref, &rows, &filtered, &extra)
	if err != nil {
		t.Fatal(err)
	}
	// A table this small is cheaper to scan, so the check is that the (tenant, request) index is a candidate, as in production.
	if possible == nil || !strings.Contains(*possible, "analysis_runs_idem") {
		t.Fatalf("the plan branch must be able to use the (tenant, request) index: possible_keys=%v", possible)
	}
}
