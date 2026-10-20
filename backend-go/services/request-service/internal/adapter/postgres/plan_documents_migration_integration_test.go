//go:build integration

package postgres

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

// A change stored before 0102 reads back with no masked secrets, and 0102 can be reversed and re-applied without
// touching the row.
func TestPostgres_Migration_OpenSpecChangeRedaction(t *testing.T) {
	_, admin := startPostgres(t)
	ctx := context.Background()
	before, target := contracttest.MigrationUpScriptsSplit(t, "postgres", "0102")
	applyScripts(t, admin, before)
	reqID, tenantID, changeID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if err := insertRequestRow(admin, map[string]any{"id": reqID, "tenant_id": tenantID}); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO request.openspec_changes (id, tenant_id, request_id, change_id, status, tasks_sync_state)
		VALUES ($1, $2, $3, 'req-1-x', 'ready', 'in_sync')`, changeID, tenantID, reqID); err != nil {
		t.Fatal(err)
	}
	applyScripts(t, admin, []string{target})
	assertAllTablesForceRLS(t, admin)

	var n int
	if err := admin.QueryRow(ctx, `SELECT redacted_secrets FROM request.openspec_changes WHERE id = $1`, changeID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("an old change must read 0: %d %v", n, err)
	}
	idx := contracttest.MigrationScriptIndex(t, "postgres", "down", "0102_")
	applyScripts(t, admin, contracttest.MigrationScripts(t, "postgres", "down")[idx:idx+1])
	var cols int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema='request' AND table_name='openspec_changes' AND column_name='redacted_secrets'`).Scan(&cols); err != nil || cols != 0 {
		t.Fatalf("down must drop the column: %d %v", cols, err)
	}
	applyScripts(t, admin, []string{target})
	if err := admin.QueryRow(ctx, `SELECT redacted_secrets FROM request.openspec_changes WHERE id = $1`, changeID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("re-applied 0102 must default the existing row to 0: %d %v", n, err)
	}
}

func TestPostgres_PlanRevisionContract(t *testing.T) {
	f := newMigratedPostgres(t)
	base := New(f.app)
	contracttest.RunPlanRevisionContract(t, contracttest.PlanRevisionEnv{
		SolutionEnv: f.solutionEnv(), Changes: NewOpenSpecChangeRepository(base), Revisions: NewOpenSpecChangeRevisionRepository(base),
	})
}

// Migration 0103 can be reversed and re-applied, and the version table is fenced by row-level security.
func TestPostgres_Migration_OpenSpecChangeRevisions(t *testing.T) {
	_, admin := startPostgres(t)
	ctx := context.Background()
	before, target := contracttest.MigrationUpScriptsSplit(t, "postgres", "0103")
	applyScripts(t, admin, before)
	applyScripts(t, admin, []string{target})
	assertAllTablesForceRLS(t, admin)
	idx := contracttest.MigrationScriptIndex(t, "postgres", "down", "0103_")
	applyScripts(t, admin, contracttest.MigrationScripts(t, "postgres", "down")[idx:idx+1])
	var n int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema='request' AND table_name='openspec_change_revisions'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("down must drop the table: %d %v", n, err)
	}
	applyScripts(t, admin, []string{target})
}

// Listing the versions of a Request with many of them reads an index, not the table.
func TestPostgres_PlanRevisionListUsesAnIndex(t *testing.T) {
	f := newMigratedPostgres(t)
	ctx := context.Background()
	reqID, tenantID := uuid.NewString(), uuid.NewString()
	if err := insertRequestRow(f.admin, map[string]any{"id": reqID, "tenant_id": tenantID}); err != nil {
		t.Fatal(err)
	}
	for n := 1; n <= 60; n++ {
		if _, err := f.admin.Exec(ctx, `INSERT INTO request.openspec_change_revisions (id, tenant_id, request_id, revision, run_id, bundle_digest, files_json)
			VALUES (gen_random_uuid(), $1, $2, $3, gen_random_uuid(), 'd', '[]')`, tenantID, reqID, n); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.admin.Exec(ctx, `ANALYZE request.openspec_change_revisions`); err != nil {
		t.Fatal(err)
	}
	// The table is small here, so the planner is told not to scan it; the test proves an index can serve the listing.
	if _, err := f.admin.Exec(ctx, `SET enable_seqscan = off`); err != nil {
		t.Fatal(err)
	}
	rows, err := f.admin.Query(ctx, `EXPLAIN SELECT id FROM request.openspec_change_revisions WHERE tenant_id = $1 AND request_id = $2 AND revision < 40 ORDER BY revision DESC LIMIT 10`, tenantID, reqID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintln(&plan, line)
	}
	if !strings.Contains(plan.String(), "Index") || strings.Contains(plan.String(), "Seq Scan") {
		t.Fatalf("the listing must use an index:\n%s", plan.String())
	}
}

// The plan branch of the activity feed filters analysis_runs by tenant and Request, so it reads analysis_runs_by_request.
func TestPostgres_PlanRevisionActivityBranchUsesAnIndex(t *testing.T) {
	f := newMigratedPostgres(t)
	ctx := context.Background()
	reqID, tenantID := uuid.NewString(), uuid.NewString()
	if err := insertRequestRow(f.admin, map[string]any{"id": reqID, "tenant_id": tenantID}); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 30; n++ {
		if _, err := f.admin.Exec(ctx, `INSERT INTO request.analysis_runs (id, tenant_id, request_id, kind, mode, status, feedback)
			VALUES (gen_random_uuid(), $1, $2, 'plan', 'complete', 'succeeded', 'x')`, tenantID, reqID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.admin.Exec(ctx, `ANALYZE request.analysis_runs`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.admin.Exec(ctx, `SET enable_seqscan = off`); err != nil {
		t.Fatal(err)
	}
	rows, err := f.admin.Query(ctx, `EXPLAIN SELECT id FROM request.analysis_runs WHERE tenant_id = $1 AND request_id = $2::uuid AND kind = 'plan'
		AND feedback IS NOT NULL AND feedback <> '' AND feedback NOT LIKE 'clarification:%'`, tenantID, reqID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintln(&plan, line)
	}
	if !strings.Contains(plan.String(), "Index") || strings.Contains(plan.String(), "Seq Scan") {
		t.Fatalf("the plan branch must use an index:\n%s", plan.String())
	}
}
