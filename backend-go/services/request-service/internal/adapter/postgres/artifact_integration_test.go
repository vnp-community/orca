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

func (f *pgFixture) artifactEnv() contracttest.ArtifactEnv {
	base := New(f.app)
	return contracttest.ArtifactEnv{
		IntakeEnv:      f.intakeEnv(),
		Content:        NewRequestContentRepository(base),
		Revisions:      NewRequestRevisionRepository(base),
		Index:          NewArtifactIndexRepository(base),
		Relations:      NewArtifactRelationRepository(base),
		Coverage:       NewRequestCoverageRepository(base),
		Clarifications: NewClarificationRepository(base),
		Decisions:      NewDecisionRepository(base),
	}
}

func TestPostgres_ArtifactContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunArtifactContract(t, func(*testing.T) contracttest.ArtifactEnv { return f.artifactEnv() })
}

func TestPostgres_ClarificationContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunClarificationContract(t, func(*testing.T) contracttest.ArtifactEnv { return f.artifactEnv() })
}

func TestPostgres_RequestsStatusCheck_Accepts12Values_Rejects13th(t *testing.T) {
	f := newMigratedPostgres(t)
	statuses := []string{"new", "classifying", "awaiting_type_confirmation", "analyzing", "awaiting_analysis_approval", "planning",
		"awaiting_plan_approval", "executing", "completed", "request_backlog", "cancelled", "awaiting_information"}
	for _, st := range statuses {
		over := map[string]any{"status": st}
		if st == "request_backlog" {
			over["returned_from_stage"], over["returned_category"] = "plan", "other"
		}
		if err := insertRequestRow(f.admin, over); err != nil {
			t.Errorf("status %s refused: %v", st, err)
		}
	}
	if err := insertRequestRow(f.admin, map[string]any{"status": "awaiting_everything"}); err == nil {
		t.Error("a 13th status must be refused")
	}
	// the Go constants and the CHECK list agree
	var def string
	if err := f.admin.QueryRow(context.Background(),
		`SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid = 'request.requests'::regclass AND conname = 'requests_status_check'`).Scan(&def); err != nil {
		t.Fatal(err)
	}
	for _, st := range statuses {
		if !strings.Contains(def, "'"+st+"'") {
			t.Errorf("CHECK lacks %s: %s", st, def)
		}
	}
}

func mustExecAdmin(t *testing.T, conn *pgx.Conn, sql string, args ...any) error {
	t.Helper()
	_, err := conn.Exec(context.Background(), sql, args...)
	return err
}

func TestPostgres_Clarifications_OneOpenPerRequest(t *testing.T) {
	f := newMigratedPostgres(t)
	tenantID, reqID := uuid.NewString(), uuid.NewString()
	ins := func(seq int, status string) error {
		return mustExecAdmin(t, f.admin, `INSERT INTO request.clarifications (id, tenant_id, request_id, seq, source, status, resume_status, asked_request_revision, due_at, created_by)
			VALUES ($1,$2,$3,$4,'manual',$5,'analyzing',1, now() + interval '1 day','u')`, uuid.NewString(), tenantID, reqID, seq, status)
	}
	if err := ins(1, "open"); err != nil {
		t.Fatal(err)
	}
	if err := ins(2, "open"); err == nil || !strings.Contains(err.Error(), "clarifications_one_open") {
		t.Fatalf("a second open clarification must hit clarifications_one_open: %v", err)
	}
	if err := mustExecAdmin(t, f.admin, `UPDATE request.clarifications SET status = 'answered' WHERE request_id = $1`, reqID); err != nil {
		t.Fatal(err)
	}
	if err := ins(2, "open"); err != nil {
		t.Fatalf("after the first is answered a new open one fits: %v", err)
	}
	if err := ins(3, "expired"); err != nil {
		t.Fatalf("closed clarifications do not count: %v", err)
	}
	if err := ins(4, "bogus"); err == nil {
		t.Fatal("the status CHECK")
	}
	if err := mustExecAdmin(t, f.admin, `INSERT INTO request.clarification_assignees (clarification_id, tenant_id, principal_kind) SELECT id, tenant_id, 'reporter' FROM request.clarifications WHERE seq = 3 AND request_id = $1`, reqID); err != nil {
		t.Fatalf("reporter assignee with the empty principal key: %v", err)
	}
	if err := mustExecAdmin(t, f.admin, `INSERT INTO request.clarification_assignees (clarification_id, tenant_id, principal_kind) SELECT id, tenant_id, 'reporter' FROM request.clarifications WHERE seq = 3 AND request_id = $1`, reqID); err == nil {
		t.Fatal("the reporter assignee is unique per clarification")
	}
}

func TestPostgres_Decisions_OneLivePerSubject(t *testing.T) {
	f := newMigratedPostgres(t)
	tenantID, reqID := uuid.NewString(), uuid.NewString()
	ins := func(seq int, status string) error {
		return mustExecAdmin(t, f.admin, `INSERT INTO request.decisions (id, tenant_id, request_id, seq, subject_kind, subject_id, options, status)
			VALUES ($1,$2,$3,$4,'solution_option','sol-1','[]'::jsonb,$5)`, uuid.NewString(), tenantID, reqID, seq, status)
	}
	for i, st := range []string{"open", "chosen", "effective"} {
		if err := ins(i+1, "superseded"); err != nil {
			t.Fatal(err)
		}
		if err := ins(10+i, st); err != nil {
			t.Fatalf("%s: %v", st, err)
		}
		if err := ins(20+i, st); err == nil || !strings.Contains(err.Error(), "decisions_one_live") {
			t.Fatalf("a second live decision (%s) must hit decisions_one_live: %v", st, err)
		}
		if err := mustExecAdmin(t, f.admin, `UPDATE request.decisions SET status = 'superseded' WHERE tenant_id = $1`, tenantID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPostgres_Migration_BackfillSolutionSeq_PerRequestOrdered(t *testing.T) {
	_, admin := startPostgres(t)
	ups := contracttest.MigrationScripts(t, "postgres", "up")
	i60 := contracttest.MigrationScriptIndex(t, "postgres", "up", "0060_")
	applyScripts(t, admin, ups[:i60]) // everything before 0060
	tenantID, reqA, reqB := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for i, req := range []string{reqA, reqA, reqA, reqB, reqB} {
		if _, err := admin.Exec(context.Background(), `INSERT INTO request.solutions (id, tenant_id, request_id, options, created_at, updated_at)
			VALUES ($1,$2,$3,'{}'::jsonb, now() + make_interval(secs => $4), now())`, uuid.NewString(), tenantID, req, i); err != nil {
			t.Fatal(err)
		}
	}
	applyScripts(t, admin, ups[i60:i60+1]) // 0060
	rows, err := admin.Query(context.Background(), `SELECT request_id, string_agg(seq::text, ',' ORDER BY seq) FROM request.solutions GROUP BY request_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var id, seqs string
		if err := rows.Scan(&id, &seqs); err != nil {
			t.Fatal(err)
		}
		got[id] = seqs
	}
	if got[reqA] != "1,2,3" || got[reqB] != "1,2" {
		t.Fatalf("seq backfill: %v", got)
	}
	if _, err := admin.Exec(context.Background(), `INSERT INTO request.solutions (id, tenant_id, request_id, options, seq) VALUES ($1,$2,$3,'{}'::jsonb,1)`, uuid.NewString(), tenantID, reqA); err == nil {
		t.Fatal("(tenant, request, seq) must be unique")
	}
	applyScripts(t, admin, ups[i60+1:i60+3]) // 0061, 0062
	if _, err := admin.Exec(context.Background(), `INSERT INTO request.solutions (id, tenant_id, request_id, options) VALUES ($1,$2,$3,'{}'::jsonb)`,
		uuid.NewString(), tenantID, reqA); err == nil {
		t.Fatal("0062 makes seq NOT NULL: a solution written without one must be refused")
	}
}

func TestPostgres_Down0061_MovesAwaitingInformationToBacklog(t *testing.T) {
	_, admin := startPostgres(t)
	ups := contracttest.MigrationScripts(t, "postgres", "up")
	downs := contracttest.MigrationScripts(t, "postgres", "down")
	applyScripts(t, admin, ups)
	id := uuid.NewString()
	if err := insertRequestRow(admin, map[string]any{"id": id, "status": "awaiting_information"}); err != nil {
		t.Fatal(err)
	}
	i61 := contracttest.MigrationScriptIndex(t, "postgres", "down", "0061_")
	u61 := contracttest.MigrationScriptIndex(t, "postgres", "up", "0061_")
	applyScripts(t, admin, downs[i61:i61+1]) // 0061 down (later migrations stay applied)
	var status, stage, category, reason string
	if err := admin.QueryRow(context.Background(), `SELECT status, returned_from_stage, returned_category, return_reason FROM request.requests WHERE id = $1`, id).Scan(&status, &stage, &category, &reason); err != nil {
		t.Fatal(err)
	}
	if status != "request_backlog" || stage != "task" || category != "missing_info" || reason != "rollback_awaiting_information" {
		t.Fatalf("%s %s %s %s", status, stage, category, reason)
	}
	if tableExists(t, admin, "clarifications") || tableExists(t, admin, "decisions") {
		t.Fatal("the clarification tables must be gone")
	}
	if err := insertRequestRow(admin, map[string]any{"status": "awaiting_information"}); err == nil {
		t.Fatal("the 11-value CHECK must be back")
	}
	applyScripts(t, admin, ups[u61:u61+1]) // 0061 up again
	if err := insertRequestRow(admin, map[string]any{"status": "awaiting_information"}); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	assertAllTablesForceRLS(t, admin)
}

// The new tables hold tenant data: a role without BYPASSRLS sees only its own tenant's rows and nothing without a tenant.
func TestPostgres_RLS_ArtifactAndClarificationTables(t *testing.T) {
	f := newMigratedPostgres(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.NewString(), uuid.NewString()
	reqA, reqB := uuid.NewString(), uuid.NewString()
	seed := map[string]func(tenantID, reqID string) (string, []any){
		"request_revisions": func(tn, rq string) (string, []any) {
			return `INSERT INTO request.request_revisions (id, tenant_id, request_id, revision, cause, snapshot, digest, actor_kind) VALUES ($1,$2,$3,1,'created','{}'::jsonb,'d','system')`, []any{uuid.NewString(), tn, rq}
		},
		"artifact_index": func(tn, rq string) (string, []any) {
			return `INSERT INTO request.artifact_index (tenant_id, display_id, kind, request_id, artifact_id) VALUES ($1,'SOL-1.1','solution',$2,$3)`, []any{tn, rq, uuid.NewString()}
		},
		"artifact_relations": func(tn, rq string) (string, []any) {
			return `INSERT INTO request.artifact_relations (id, tenant_id, request_id, rel, from_kind, from_id, to_kind, to_id) VALUES ($1,$2,$3,'derived_from','solution','a','request','b')`, []any{uuid.NewString(), tn, rq}
		},
		"request_coverage": func(tn, rq string) (string, []any) {
			return `INSERT INTO request.request_coverage (id, tenant_id, request_id, plan_task_id, ac_id, task_id) VALUES ($1,$2,$3,$4,'AC-1',$5)`, []any{uuid.NewString(), tn, rq, uuid.NewString(), uuid.NewString()}
		},
		"clarifications": func(tn, rq string) (string, []any) {
			return `INSERT INTO request.clarifications (id, tenant_id, request_id, seq, source, status, resume_status, asked_request_revision, due_at, created_by) VALUES ($1,$2,$3,1,'manual','open','analyzing',1,now(),'u')`, []any{uuid.NewString(), tn, rq}
		},
		"decisions": func(tn, rq string) (string, []any) {
			return `INSERT INTO request.decisions (id, tenant_id, request_id, seq, subject_kind, subject_id, options, status) VALUES ($1,$2,$3,1,'other','s','[]'::jsonb,'open')`, []any{uuid.NewString(), tn, rq}
		},
	}
	for table, build := range seed {
		for tenantID, reqID := range map[string]string{tenantA: reqA, tenantB: reqB} {
			sqlText, args := build(tenantID, reqID)
			withTenantCommit(t, f.app, tenantID, func(tx pgx.Tx) {
				if _, err := tx.Exec(ctx, sqlText, args...); err != nil {
					t.Fatalf("%s: %v", table, err)
				}
			})
		}
		withTenant(t, f.app, tenantA, func(tx pgx.Tx) {
			if n := countRows(t, tx, table); n != 1 {
				t.Errorf("%s: tenant A sees %d rows, want 1", table, n)
			}
		})
		withTenant(t, f.app, "", func(tx pgx.Tx) {
			if n := countRows(t, tx, table); n != 0 {
				t.Errorf("%s: a session without a tenant sees %d rows", table, n)
			}
		})
		// the write side: a row for another tenant is refused by WITH CHECK
		sqlText, args := build(tenantB, reqB)
		withTenant(t, f.app, tenantA, func(tx pgx.Tx) {
			if _, err := tx.Exec(ctx, sqlText, args...); err == nil {
				t.Errorf("%s: tenant A wrote a row for tenant B", table)
			}
		})
	}
	// the relay policy exposes clarifications read-only across tenants and nothing else
	var n int
	relay := func(sqlText string) error {
		tx, err := f.app.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if _, err := tx.Exec(ctx, `SELECT set_config('app.relay', 'on', true)`); err != nil {
			return err
		}
		return tx.QueryRow(ctx, sqlText).Scan(&n)
	}
	if err := relay(`SELECT count(*) FROM request.clarifications`); err != nil || n != 2 {
		t.Fatalf("relay scan sees %d clarifications: %v", n, err)
	}
	for _, table := range []string{"decisions", "request_revisions", "clarification_questions"} {
		if err := relay(fmt.Sprintf(`SELECT count(*) FROM request.%s`, table)); err != nil || n != 0 {
			t.Errorf("the relay must not see %s: %d %v", table, n, err)
		}
	}
}
