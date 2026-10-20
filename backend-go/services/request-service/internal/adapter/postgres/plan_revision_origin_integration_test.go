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

func TestPostgres_PlanRevisionFailureContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunPlanRevisionFailureContract(t, contracttest.PlanRevisionFailureEnv{
		SolutionEnv: f.solutionEnv(), Jobs: NewPlanAutoJobRepository(New(f.app)),
	})
}

func TestPostgres_PlanRevisionReadContract(t *testing.T) {
	f := newMigratedPostgres(t)
	base := New(f.app)
	contracttest.RunPlanRevisionReadContract(t, contracttest.PlanRevisionEnv{
		SolutionEnv: f.solutionEnv(), Changes: NewOpenSpecChangeRepository(base), Revisions: NewOpenSpecChangeRevisionRepository(base),
	})
}

// Listing the successful Plan runs of a Request (the numbers of its versions) reads an index, not the table.
func TestPostgres_ListSucceededPlanRunsUsesAnIndex(t *testing.T) {
	f := newMigratedPostgres(t)
	ctx := context.Background()
	reqID, tenantID := uuid.NewString(), uuid.NewString()
	if err := insertRequestRow(f.admin, map[string]any{"id": reqID, "tenant_id": tenantID}); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 60; n++ {
		if _, err := f.admin.Exec(ctx, `INSERT INTO request.analysis_runs (id, tenant_id, request_id, kind, mode, status) VALUES (gen_random_uuid(), $1, $2, 'plan', 'complete', 'succeeded')`, tenantID, reqID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.admin.Exec(ctx, `ANALYZE request.analysis_runs`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.admin.Exec(ctx, `SET enable_seqscan = off`); err != nil {
		t.Fatal(err)
	}
	rows, err := f.admin.Query(ctx, `EXPLAIN SELECT id FROM request.analysis_runs WHERE tenant_id = $1 AND request_id = $2 AND kind = 'plan' AND status = 'succeeded' ORDER BY started_at ASC, id`, tenantID, reqID)
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
		t.Fatalf("the version listing must use an index:\n%s", plan.String())
	}
}

// Migration 0104 lets a job carry the origin 'revision'; going back removes those jobs and keeps the others.
func TestPostgres_Migration_PlanAutoJobsRevisionOrigin(t *testing.T) {
	_, admin := startPostgres(t)
	ctx := context.Background()
	before, target := contracttest.MigrationUpScriptsSplit(t, "postgres", "0104")
	applyScripts(t, admin, before)
	reqID, tenantID := uuid.NewString(), uuid.NewString()
	if err := insertRequestRow(admin, map[string]any{"id": reqID, "tenant_id": tenantID}); err != nil {
		t.Fatal(err)
	}
	insert := func(origin, anchor, state string) error {
		_, err := admin.Exec(ctx, `INSERT INTO request.plan_auto_jobs (tenant_id, request_id, anchor, origin, project_id, state) VALUES ($1, $2, $3, $4, $5, $6)`,
			tenantID, reqID, anchor, origin, uuid.NewString(), state)
		return err
	}
	if err := insert("event", "sol-1", "done"); err != nil {
		t.Fatal(err)
	}
	if err := insert("revision", "rev-0", "done"); err == nil {
		t.Fatal("before 0104 the origin 'revision' must be refused")
	}
	applyScripts(t, admin, []string{target})
	if err := insert("revision", "rev-1", "done"); err != nil {
		t.Fatalf("after 0104: %v", err)
	}
	if err := insert("manual", "x-1", "done"); err == nil {
		t.Fatal("an unknown origin must still be refused")
	}

	idx := contracttest.MigrationScriptIndex(t, "postgres", "down", "0104_")
	applyScripts(t, admin, contracttest.MigrationScripts(t, "postgres", "down")[idx:idx+1])
	var revisions, others int
	if err := admin.QueryRow(ctx, `SELECT count(*) FILTER (WHERE origin = 'revision'), count(*) FILTER (WHERE origin <> 'revision') FROM request.plan_auto_jobs`).Scan(&revisions, &others); err != nil {
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
