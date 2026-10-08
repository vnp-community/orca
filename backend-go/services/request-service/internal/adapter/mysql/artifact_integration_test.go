//go:build integration

package mysql

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func (f *myFixture) artifactEnv() contracttest.ArtifactEnv {
	base := New(f.db)
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

func TestMySQL_ArtifactContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunArtifactContract(t, func(*testing.T) contracttest.ArtifactEnv { return f.artifactEnv() })
}

func TestMySQL_ClarificationContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunClarificationContract(t, func(*testing.T) contracttest.ArtifactEnv { return f.artifactEnv() })
}

func rawRequest(f *myFixture, status string, extra map[string]any) error {
	cols := []string{"id", "tenant_id", "number", "title", "body", "source_provider", "source_url", "status", "urgency", "classification_reason", "return_reason", "reporter_id"}
	vals := map[string]any{
		"id": uuid.NewString(), "tenant_id": uuid.NewString(), "number": 1, "title": "t", "body": "", "source_provider": "manual",
		"source_url": "", "status": status, "urgency": "normal", "classification_reason": "", "return_reason": "", "reporter_id": uuid.NewString(),
	}
	for k, v := range extra {
		if _, ok := vals[k]; !ok {
			cols = append(cols, k)
		}
		vals[k] = v
	}
	args := make([]any, len(cols))
	for i, c := range cols {
		args[i] = vals[c]
	}
	_, err := f.admin.Exec("INSERT INTO requests ("+strings.Join(cols, ", ")+") VALUES ("+strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",")+")", args...)
	return err
}

func TestMySQL_RequestsStatusCheck_Accepts12Values_Rejects13th(t *testing.T) {
	f := newMigratedMySQL(t)
	for _, st := range []string{"new", "classifying", "awaiting_type_confirmation", "analyzing", "awaiting_analysis_approval", "planning",
		"awaiting_plan_approval", "executing", "completed", "request_backlog", "cancelled", "awaiting_information"} {
		extra := map[string]any{}
		if st == "request_backlog" {
			extra["returned_from_stage"], extra["returned_category"] = "plan", "other"
		}
		if err := rawRequest(f, st, extra); err != nil {
			t.Errorf("status %s refused: %v", st, err)
		}
	}
	if err := rawRequest(f, "awaiting_everything", nil); err == nil {
		t.Error("a 13th status must be refused")
	}
	var clause string
	if err := f.admin.QueryRow(`SELECT cc.CHECK_CLAUSE FROM information_schema.CHECK_CONSTRAINTS cc WHERE cc.CONSTRAINT_SCHEMA = DATABASE() AND cc.CONSTRAINT_NAME = 'requests_status_check'`).Scan(&clause); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(clause, "awaiting_information") {
		t.Fatalf("CHECK lacks awaiting_information: %s", clause)
	}
}

func execAdmin(f *myFixture, query string, args ...any) error {
	_, err := f.admin.Exec(query, args...)
	return err
}

func TestMySQL_Clarifications_OneOpenPerRequest(t *testing.T) {
	f := newMigratedMySQL(t)
	tenantID, reqID := uuid.NewString(), uuid.NewString()
	ins := func(seq int, status string) error {
		return execAdmin(f, `INSERT INTO clarifications (id, tenant_id, request_id, seq, source, status, resume_status, asked_request_revision, due_at, cancel_reason, created_by)
			VALUES (?,?,?,?,'manual',?,'analyzing',1, NOW(6) + INTERVAL 1 DAY,'','u')`, uuid.NewString(), tenantID, reqID, seq, status)
	}
	if err := ins(1, "open"); err != nil {
		t.Fatal(err)
	}
	if err := ins(2, "open"); err == nil || !strings.Contains(err.Error(), "clarifications_one_open") {
		t.Fatalf("a second open clarification must hit clarifications_one_open: %v", err)
	}
	if err := execAdmin(f, `UPDATE clarifications SET status = 'answered' WHERE request_id = ?`, reqID); err != nil {
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
	if err := execAdmin(f, `INSERT INTO clarification_assignees (clarification_id, tenant_id, principal_kind) SELECT id, tenant_id, 'reporter' FROM clarifications WHERE seq = 3 AND request_id = ?`, reqID); err != nil {
		t.Fatalf("reporter assignee with the empty principal key: %v", err)
	}
	if err := execAdmin(f, `INSERT INTO clarification_assignees (clarification_id, tenant_id, principal_kind) SELECT id, tenant_id, 'reporter' FROM clarifications WHERE seq = 3 AND request_id = ?`, reqID); err == nil {
		t.Fatal("the reporter assignee is unique per clarification")
	}
}

func TestMySQL_Decisions_OneLivePerSubject(t *testing.T) {
	f := newMigratedMySQL(t)
	tenantID, reqID := uuid.NewString(), uuid.NewString()
	ins := func(seq int, status string) error {
		return execAdmin(f, `INSERT INTO decisions (id, tenant_id, request_id, seq, subject_kind, subject_id, question, options, recommendation_reason, rationale, status)
			VALUES (?,?,?,?,'solution_option','sol-1','','[]','','',?)`, uuid.NewString(), tenantID, reqID, seq, status)
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
		if err := execAdmin(f, `UPDATE decisions SET status = 'superseded' WHERE tenant_id = ?`, tenantID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMySQL_Migration_BackfillSolutionSeq_PerRequestOrdered(t *testing.T) {
	_, admin := startMySQL(t)
	ups := contracttest.MigrationScripts(t, "mysql", "up")
	i60 := contracttest.MigrationScriptIndex(t, "mysql", "up", "0060_")
	applyScripts(t, admin, ups[:i60]) // everything before 0060
	tenantID, reqA, reqB := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for i, req := range []string{reqA, reqA, reqA, reqB, reqB} {
		if _, err := admin.Exec(`INSERT INTO solutions (id, tenant_id, request_id, options, created_at, updated_at)
			VALUES (?,?,?,'{}', TIMESTAMPADD(SECOND, ?, NOW(6)), NOW(6))`, uuid.NewString(), tenantID, req, i); err != nil {
			t.Fatal(err)
		}
	}
	applyScripts(t, admin, ups[i60:i60+1]) // 0060
	rows, err := admin.Query(`SELECT request_id, GROUP_CONCAT(seq ORDER BY seq) FROM solutions GROUP BY request_id`)
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
	if _, err := admin.Exec(`INSERT INTO solutions (id, tenant_id, request_id, options, seq) VALUES (?,?,?,'{}',1)`, uuid.NewString(), tenantID, reqA); err == nil {
		t.Fatal("(tenant, request, seq) must be unique")
	}
	applyScripts(t, admin, ups[i60+1:i60+3]) // 0061, 0062
	if _, err := admin.Exec(`INSERT INTO solutions (id, tenant_id, request_id, options) VALUES (?,?,?,'{}')`,
		uuid.NewString(), tenantID, reqA); err == nil {
		t.Fatal("0062 makes seq NOT NULL: a solution written without one must be refused")
	}
}

func TestMySQL_Down0061_MovesAwaitingInformationToBacklog(t *testing.T) {
	f := newMigratedMySQL(t)
	downs := contracttest.MigrationScripts(t, "mysql", "down")
	ups := contracttest.MigrationScripts(t, "mysql", "up")
	id := uuid.NewString()
	if err := rawRequest(f, "awaiting_information", map[string]any{"id": id}); err != nil {
		t.Fatal(err)
	}
	i61 := contracttest.MigrationScriptIndex(t, "mysql", "down", "0061_")
	u61 := contracttest.MigrationScriptIndex(t, "mysql", "up", "0061_")
	applyScripts(t, f.admin, downs[i61:i61+1]) // 0061 down (later migrations stay applied)
	var status, stage, category, reason string
	if err := f.admin.QueryRow(`SELECT status, returned_from_stage, returned_category, return_reason FROM requests WHERE id = ?`, id).Scan(&status, &stage, &category, &reason); err != nil {
		t.Fatal(err)
	}
	if status != "request_backlog" || stage != "task" || category != "missing_info" || reason != "rollback_awaiting_information" {
		t.Fatalf("%s %s %s %s", status, stage, category, reason)
	}
	if hasTable(t, f.admin, "clarifications") || hasTable(t, f.admin, "decisions") {
		t.Fatal("the clarification tables must be gone")
	}
	if err := rawRequest(f, "awaiting_information", nil); err == nil {
		t.Fatal("the 11-value CHECK must be back")
	}
	applyScripts(t, f.admin, ups[u61:u61+1]) // 0061 up again
	if err := rawRequest(f, "awaiting_information", nil); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}
