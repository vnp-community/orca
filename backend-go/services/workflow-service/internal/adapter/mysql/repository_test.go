//go:build integration

// Integration tests run against a real MySQL via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/mysql/...`. Mirrors
// internal/adapter/postgres/repository_test.go's test names/shape 1:1
// where a Postgres-only feature doesn't get in the way, plus
// TestRepository_ListTemplates_DoesNotLeakAcrossTenants and
// TestRepository_GetTemplate_DoesNotLeakAcrossTenants (TASK-BE-DB-003's
// pattern) — workflow.templates/executions both carry an RLS policy in the
// Postgres migration, so these prove application-layer tenant_id scoping
// alone is sufficient on a dialect with NO RLS equivalent at all.
package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// jsonEqual compares two JSON strings structurally rather than
// byte-for-byte — mirrors internal/adapter/postgres's identical helper
// (MySQL's JSON column also reformats on read-back).
func jsonEqual(t *testing.T, a, b string) bool {
	t.Helper()
	var va, vb any
	if err := json.Unmarshal([]byte(a), &va); err != nil {
		t.Fatalf("unmarshal %q: %v", a, err)
	}
	if err := json.Unmarshal([]byte(b), &vb); err != nil {
		t.Fatalf("unmarshal %q: %v", b, err)
	}
	ja, _ := json.Marshal(va)
	jb, _ := json.Marshal(vb)
	return string(ja) == string(jb)
}

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	// testutil.StartMySQL returns "mysql://root:orca@tcp(host:port)/db" —
	// valid as-is for golang-migrate's mysql driver CLI (used below), but
	// go-sql-driver/mysql's database/sql driver expects its OWN DSN format
	// with no "mysql://" scheme. parseTime=true is required for TIMESTAMP
	// columns to scan into time.Time/sql.NullTime instead of []byte.
	rawDSN := testutil.StartMySQL(t, "workflow")
	driverDSN := strings.TrimPrefix(rawDSN, "mysql://") + "?parseTime=true"

	migrationsPath, err := filepath.Abs("../../../migrations/mysql")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	cmd := exec.Command("migrate", "-path", migrationsPath, "-database", rawDSN, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running migrations: %v\n%s", err, out)
	}

	db, err := sql.Open("mysql", driverDSN)
	if err != nil {
		t.Fatalf("connecting to mysql: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("pinging mysql: %v", err)
	}

	return New(db)
}

func TestRepository_CreateAndGetTemplate(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), uuid.NewString(), "deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	got, err := repo.GetTemplate(ctx, tmpl.TenantID, tmpl.ID)
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	if got.Name != "deploy" {
		t.Errorf("expected name=deploy, got %s", got.Name)
	}
}

// TestRepository_GetTemplate_DoesNotLeakAcrossTenants proves the
// application-layer tenant_id filter alone (no RLS equivalent exists in
// MySQL) actually excludes another tenant's row — see this file's package
// doc comment.
func TestRepository_GetTemplate_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), uuid.NewString(), "secret", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	_, err = repo.GetTemplate(ctx, uuid.NewString(), tmpl.ID)
	if !errors.Is(err, domain.ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound looking up another tenant's template, got %v", err)
	}
}

func TestRepository_ExecutionPauseResumeRoundTrip(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	tmpl, _ := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "release", `{"steps":[]}`, domain.ScopeTeam, "", "owner-1")
	_ = repo.CreateTemplate(ctx, tmpl)

	exec, err := domain.NewWorkflowExecution(uuid.NewString(), tenantID, tmpl.ID, "trace-1", "", "")
	if err != nil {
		t.Fatalf("building execution: %v", err)
	}
	if err := repo.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	if err := exec.Pause(time.Now().UTC()); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if err := repo.UpdateExecution(ctx, exec, nil); err != nil {
		t.Fatalf("update (pause): %v", err)
	}

	got, err := repo.GetExecution(ctx, tenantID, exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if got.Status != domain.StatusPaused || got.PausedAt == nil {
		t.Fatalf("expected paused execution with PausedAt set, got %+v", got)
	}
}

func TestRepository_CreateExecution_PersistsInputsJSON(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	tmpl, _ := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "inputs-test", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	exec, err := domain.NewWorkflowExecution(uuid.NewString(), tenantID, tmpl.ID, "trace-inputs", "", "")
	if err != nil {
		t.Fatalf("building execution: %v", err)
	}
	exec.InputsJSON = `{"feature_description":"add dark mode"}`
	if err := repo.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	got, err := repo.GetExecution(ctx, tenantID, exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if !jsonEqual(t, got.InputsJSON, exec.InputsJSON) {
		t.Errorf("expected InputsJSON to round-trip, got %q want %q", got.InputsJSON, exec.InputsJSON)
	}
}

func TestRepository_CreateExecution_EmptyInputsJSONRoundTripsEmpty(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	tmpl, _ := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "no-inputs-test", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	exec, err := domain.NewWorkflowExecution(uuid.NewString(), tenantID, tmpl.ID, "trace-no-inputs", "", "")
	if err != nil {
		t.Fatalf("building execution: %v", err)
	}
	if err := repo.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	got, err := repo.GetExecution(ctx, tenantID, exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if got.InputsJSON != "" {
		t.Errorf("expected empty InputsJSON, got %q", got.InputsJSON)
	}
}

func TestRepository_ListTemplates_KeysetPagination(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	for i := 0; i < 3; i++ {
		tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "t", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
		if err != nil {
			t.Fatalf("building template: %v", err)
		}
		if err := repo.CreateTemplate(ctx, tmpl); err != nil {
			t.Fatalf("create template: %v", err)
		}
	}

	firstPage, next, err := repo.ListTemplates(ctx, tenantID, "", "", nil, "", "", 2)
	if err != nil {
		t.Fatalf("list templates (page 1): %v", err)
	}
	if len(firstPage) != 2 || next == "" {
		t.Fatalf("expected a full first page with a next token, got %d rows, next=%q", len(firstPage), next)
	}

	secondPage, next2, err := repo.ListTemplates(ctx, tenantID, "", "", nil, "", next, 2)
	if err != nil {
		t.Fatalf("list templates (page 2): %v", err)
	}
	if len(secondPage) != 1 || next2 != "" {
		t.Fatalf("expected exactly one remaining template and no further page, got %d rows, next=%q", len(secondPage), next2)
	}
}

// TestRepository_ListTemplates_DoesNotLeakAcrossTenants — see this file's
// package doc comment.
func TestRepository_ListTemplates_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.NewString(), uuid.NewString()

	tmplA, _ := domain.NewWorkflowTemplate(uuid.NewString(), tenantA, "a-only", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err := repo.CreateTemplate(ctx, tmplA); err != nil {
		t.Fatalf("create template a: %v", err)
	}
	tmplB, _ := domain.NewWorkflowTemplate(uuid.NewString(), tenantB, "b-only", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err := repo.CreateTemplate(ctx, tmplB); err != nil {
		t.Fatalf("create template b: %v", err)
	}

	got, _, err := repo.ListTemplates(ctx, tenantA, "", "", nil, "", "", 50)
	if err != nil {
		t.Fatalf("list templates: %v", err)
	}
	if len(got) != 1 || got[0].ID != tmplA.ID {
		t.Fatalf("expected only tenant A's template, got %+v", got)
	}
}

func TestRepository_ListExecutions_KeysetPaginationNewestFirst(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	projectID := uuid.NewString()

	tmpl, _ := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "list-exec", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	var ids []string
	for i := 0; i < 3; i++ {
		id := uuid.NewString()
		exec, err := domain.NewWorkflowExecution(id, tenantID, tmpl.ID, "trace-"+id, projectID, "")
		if err != nil {
			t.Fatalf("building execution %s: %v", id, err)
		}
		if err := repo.CreateExecution(ctx, exec); err != nil {
			t.Fatalf("create execution %s: %v", id, err)
		}
		ids = append(ids, id)
		time.Sleep(10 * time.Millisecond) // ensure distinct created_at for deterministic ordering
	}

	firstPage, next, err := repo.ListExecutions(ctx, tenantID, projectID, "", 2)
	if err != nil {
		t.Fatalf("list executions (page 1): %v", err)
	}
	if len(firstPage) != 2 || next == "" {
		t.Fatalf("expected a full first page with a next cursor, got %d rows, next=%q", len(firstPage), next)
	}
	if firstPage[0].ID != ids[2] || firstPage[1].ID != ids[1] {
		t.Fatalf("expected newest-first order [%s, %s], got %+v", ids[2], ids[1], firstPage)
	}

	secondPage, next2, err := repo.ListExecutions(ctx, tenantID, projectID, next, 2)
	if err != nil {
		t.Fatalf("list executions (page 2): %v", err)
	}
	if len(secondPage) != 1 || next2 != "" {
		t.Fatalf("expected exactly one remaining execution and no further page, got %d rows, next=%q", len(secondPage), next2)
	}
	if secondPage[0].ID != ids[0] {
		t.Fatalf("expected the oldest execution %s last, got %+v", ids[0], secondPage)
	}
}

func TestRepository_ResolveChain_RootFirstOrder(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	root, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "company-base", `{"steps":[{"id":"s1","type":"webhook"}]}`, domain.ScopeCompany, "", "owner-1")
	if err != nil {
		t.Fatalf("building root: %v", err)
	}
	if err := repo.CreateTemplate(ctx, root); err != nil {
		t.Fatalf("create root: %v", err)
	}

	child, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "personal-override", `{"steps":[]}`, domain.ScopePersonal, root.ID, "owner-1")
	if err != nil {
		t.Fatalf("building child: %v", err)
	}
	if err := repo.CreateTemplate(ctx, child); err != nil {
		t.Fatalf("create child: %v", err)
	}

	chain, err := repo.ResolveChain(ctx, tenantID, child.ID, 5)
	if err != nil {
		t.Fatalf("resolve chain: %v", err)
	}
	if len(chain) != 2 || chain[0].ID != root.ID || chain[1].ID != child.ID {
		t.Fatalf("expected root-first chain [root, child], got %+v", chain)
	}
}

func TestRepository_ResolveChain_NotFound(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	_, err := repo.ResolveChain(ctx, uuid.NewString(), uuid.NewString(), 5)
	if err == nil {
		t.Fatal("expected an error resolving a chain for a template that doesn't exist")
	}
}

func TestRepository_Update_CorrectVersion_Succeeds(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	tmpl.Name = "deploy-v2"
	tmpl.DAGJSON = `{"steps":[{"id":"s1","type":"webhook"}]}`
	tmpl.Scope = domain.ScopeTeam

	updated, err := repo.Update(ctx, tmpl, 1, true)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("want bumped version 2, got %d", updated.Version)
	}
	if updated.Name != "deploy-v2" || updated.Scope != domain.ScopeTeam {
		t.Fatalf("update did not persist the new fields: %+v", updated)
	}

	got, err := repo.GetTemplate(ctx, tenantID, tmpl.ID)
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	if got.Version != 2 || got.Name != "deploy-v2" || got.Scope != domain.ScopeTeam {
		t.Fatalf("re-read template does not reflect the update: %+v", got)
	}
	if !jsonEqual(t, got.DAGJSON, tmpl.DAGJSON) {
		t.Fatalf("re-read dag_json = %q, want structurally equal to %q", got.DAGJSON, tmpl.DAGJSON)
	}
}

func TestRepository_Update_NoBump_LeavesVersionUnchanged(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	tmpl.Name = "deploy-metadata-only"
	updated, err := repo.Update(ctx, tmpl, 1, false)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Version != 1 {
		t.Fatalf("want version unchanged at 1 when bumpVersion=false, got %d", updated.Version)
	}

	// expected_version still gates the write even with bumpVersion=false —
	// a second call at the same (unmoved) expected_version must succeed
	// again, proving the WHERE clause isn't accidentally always-true —
	// mirrors internal/adapter/postgres's identical second-call assertion.
	tmpl2 := updated
	tmpl2.Name = "deploy-metadata-only-2"
	if _, err := repo.Update(ctx, tmpl2, 1, false); err != nil {
		t.Fatalf("second no-bump update at unmoved expected_version: %v", err)
	}
}

func TestRepository_Update_StaleVersion_ReturnsConflict(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	attempt := tmpl
	attempt.Name = "should-not-apply"

	_, err = repo.Update(ctx, attempt, 99, true)
	if !errors.Is(err, domain.ErrTemplateVersionConflict) {
		t.Fatalf("want domain.ErrTemplateVersionConflict, got %v", err)
	}

	got, err := repo.GetTemplate(ctx, tenantID, tmpl.ID)
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	if got.Version != 1 || got.Name != "deploy" {
		t.Fatalf("row must be unchanged after a conflicting update, got %+v", got)
	}
}

// TestRepository_CreateExecution_AdHocNullTemplateID exercises migration
// 0005_execution_ad_hoc_template: ExecuteAdHocStep's synthetic execution
// has no backing template, which requires executions.template_id to
// actually accept NULL.
func TestRepository_CreateExecution_AdHocNullTemplateID(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	exec, err := domain.NewAdHocWorkflowExecution(uuid.NewString(), tenantID, "trace-adhoc")
	if err != nil {
		t.Fatalf("building ad hoc execution: %v", err)
	}
	if err := repo.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create ad hoc execution: %v", err)
	}

	got, err := repo.GetExecution(ctx, tenantID, exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if got.TemplateID != "" {
		t.Fatalf("expected a NULL/empty template_id for an ad hoc execution, got %q", got.TemplateID)
	}
	if got.Status != domain.StatusRunning {
		t.Fatalf("expected status=running, got %v", got.Status)
	}
}

// TestRepository_StepExecution_CreateAndUpdateRoundTrip exercises the
// step_executions table end-to-end: create a real (templated) execution,
// persist a pending step execution under it, transition it to completed
// with output, and read it back via ListStepExecutions.
func TestRepository_StepExecution_CreateAndUpdateRoundTrip(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "step-exec-template", `{"steps":[{"id":"a","type":"shell"}]}`, domain.ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	exec, err := domain.NewWorkflowExecution(uuid.NewString(), tenantID, tmpl.ID, "trace-step", "", "")
	if err != nil {
		t.Fatalf("building execution: %v", err)
	}
	if err := repo.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	se, err := domain.NewStepExecution(uuid.NewString(), exec.ID, "a", uuid.NewString(), 0)
	if err != nil {
		t.Fatalf("building step execution: %v", err)
	}
	if err := repo.CreateStepExecution(ctx, se); err != nil {
		t.Fatalf("create step execution: %v", err)
	}

	se.MarkRunning()
	if err := repo.UpdateStepExecution(ctx, se, domain.OutboxEvent{}); err != nil {
		t.Fatalf("update (running) step execution: %v", err)
	}

	rows, err := repo.ListStepExecutions(ctx, tenantID, exec.ID)
	if err != nil {
		t.Fatalf("list step executions: %v", err)
	}
	if len(rows) != 1 || rows[0].Status != domain.StepExecutionStatusRunning {
		t.Fatalf("expected 1 row with status=running, got %+v", rows)
	}

	se.FromResult(domain.StepResult{Status: domain.ResultStatusCompleted, OutputJSON: `{"ok":true}`})
	if err := repo.UpdateStepExecution(ctx, se, domain.OutboxEvent{}); err != nil {
		t.Fatalf("update (completed) step execution: %v", err)
	}

	rows, err = repo.ListStepExecutions(ctx, tenantID, exec.ID)
	if err != nil {
		t.Fatalf("list step executions after completion: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 step execution row, got %d", len(rows))
	}
	got := rows[0]
	if got.Status != domain.StepExecutionStatusCompleted {
		t.Errorf("expected status=completed, got %v", got.Status)
	}
	if !jsonEqual(t, got.OutputJSON, `{"ok":true}`) {
		t.Errorf("expected output to round-trip (structurally), got %q", got.OutputJSON)
	}
	if got.DispatchToken == "" {
		t.Error("expected dispatch_token to round-trip")
	}
}

// TestRepository_ListRunning_ReturnsOnlyRunningAcrossTenants exercises
// usecase.RecoverExecutions' boot-time scan query end-to-end — see
// ExecutionRepository.ListRunning's doc comment for why this is the one
// deliberately unscoped query.
func TestRepository_ListRunning_ReturnsOnlyRunningAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.NewString(), uuid.NewString()

	tmplA, _ := domain.NewWorkflowTemplate(uuid.NewString(), tenantA, "t-a", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	_ = repo.CreateTemplate(ctx, tmplA)
	tmplB, _ := domain.NewWorkflowTemplate(uuid.NewString(), tenantB, "t-b", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	_ = repo.CreateTemplate(ctx, tmplB)

	running, err := domain.NewWorkflowExecution(uuid.NewString(), tenantA, tmplA.ID, "trace-running", "", "")
	if err != nil {
		t.Fatalf("building running execution: %v", err)
	}
	if err := repo.CreateExecution(ctx, running); err != nil {
		t.Fatalf("create running execution: %v", err)
	}

	runningOtherTenant, err := domain.NewWorkflowExecution(uuid.NewString(), tenantB, tmplB.ID, "trace-running-b", "", "")
	if err != nil {
		t.Fatalf("building second running execution: %v", err)
	}
	if err := repo.CreateExecution(ctx, runningOtherTenant); err != nil {
		t.Fatalf("create second running execution: %v", err)
	}

	paused, err := domain.NewWorkflowExecution(uuid.NewString(), tenantA, tmplA.ID, "trace-paused", "", "")
	if err != nil {
		t.Fatalf("building paused execution: %v", err)
	}
	if err := repo.CreateExecution(ctx, paused); err != nil {
		t.Fatalf("create paused execution: %v", err)
	}
	if err := paused.Pause(time.Now().UTC()); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if err := repo.UpdateExecution(ctx, paused, nil); err != nil {
		t.Fatalf("update (pause): %v", err)
	}

	got, err := repo.ListRunning(ctx)
	if err != nil {
		t.Fatalf("list running: %v", err)
	}

	gotIDs := make(map[string]bool, len(got))
	for _, e := range got {
		if e.Status != domain.StatusRunning {
			t.Errorf("expected every returned row to be status=running, got %v for %s", e.Status, e.ID)
		}
		gotIDs[e.ID] = true
	}
	if !gotIDs[running.ID] {
		t.Errorf("expected running execution %s to be returned", running.ID)
	}
	if !gotIDs[runningOtherTenant.ID] {
		t.Errorf("expected running execution %s from a different tenant to also be returned — ListRunning is deliberately unscoped", runningOtherTenant.ID)
	}
	if gotIDs[paused.ID] {
		t.Errorf("expected paused execution %s to be excluded", paused.ID)
	}
}

// TestRepository_ListStepExecutions_ScopedByTenant proves ListStepExecutions'
// tenant join actually excludes another tenant's execution_id, not just
// filters by a caller-supplied tenantID string — see this file's package
// doc comment.
func TestRepository_ListStepExecutions_ScopedByTenant(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.NewString(), uuid.NewString()

	tmpl, _ := domain.NewWorkflowTemplate(uuid.NewString(), tenantA, "t", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	_ = repo.CreateTemplate(ctx, tmpl)
	exec, _ := domain.NewWorkflowExecution(uuid.NewString(), tenantA, tmpl.ID, "trace", "", "")
	_ = repo.CreateExecution(ctx, exec)
	se, _ := domain.NewStepExecution(uuid.NewString(), exec.ID, "a", uuid.NewString(), 0)
	if err := repo.CreateStepExecution(ctx, se); err != nil {
		t.Fatalf("create step execution: %v", err)
	}

	rowsForOwner, err := repo.ListStepExecutions(ctx, tenantA, exec.ID)
	if err != nil {
		t.Fatalf("list as owning tenant: %v", err)
	}
	if len(rowsForOwner) != 1 {
		t.Fatalf("expected the owning tenant to see 1 row, got %d", len(rowsForOwner))
	}

	rowsForOther, err := repo.ListStepExecutions(ctx, tenantB, exec.ID)
	if err != nil {
		t.Fatalf("list as other tenant: %v", err)
	}
	if len(rowsForOther) != 0 {
		t.Fatalf("expected a different tenant to see 0 rows for another tenant's execution, got %d", len(rowsForOther))
	}
}

// ── outbox (SOL-PW-04/TASK-PW-04-05) ───────────────────────────────────────

func TestRepository_UpdateExecution_WithEvent_WritesOutboxRowInSameTransaction(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	tmpl, _ := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "t", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	_ = repo.CreateTemplate(ctx, tmpl)
	exec, _ := domain.NewWorkflowExecution(uuid.NewString(), tenantID, tmpl.ID, "trace", "", "")
	if err := repo.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	exec.Status = domain.StatusCompleted
	event := &domain.OutboxEvent{
		ID:          uuid.NewString(),
		Subject:     "orca.workflow.execution.completed",
		OccurredAt:  time.Now().UTC(),
		PayloadJSON: []byte(`{}`),
	}
	if err := repo.UpdateExecution(ctx, exec, event); err != nil {
		t.Fatalf("update with event: %v", err)
	}

	unpublished, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished: %v", err)
	}
	if len(unpublished) != 1 || unpublished[0].Subject != "orca.workflow.execution.completed" {
		t.Fatalf("expected exactly 1 unpublished outbox row, got %+v", unpublished)
	}

	if err := repo.MarkPublished(ctx, []string{unpublished[0].ID}); err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}
	afterMark, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished after mark: %v", err)
	}
	if len(afterMark) != 0 {
		t.Errorf("expected no unpublished rows after MarkPublished, got %+v", afterMark)
	}
}

func TestRepository_UpdateExecution_NilEvent_WritesNoOutboxRow(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	tmpl, _ := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "t", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	_ = repo.CreateTemplate(ctx, tmpl)
	exec, _ := domain.NewWorkflowExecution(uuid.NewString(), tenantID, tmpl.ID, "trace", "", "")
	if err := repo.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	if err := exec.Pause(time.Now().UTC()); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if err := repo.UpdateExecution(ctx, exec, nil); err != nil {
		t.Fatalf("update (nil event): %v", err)
	}

	unpublished, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished: %v", err)
	}
	if len(unpublished) != 0 {
		t.Errorf("expected no outbox rows for a nil-event update, got %+v", unpublished)
	}
}

func TestRepository_MarkPublished_EmptyIDsIsNoop(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	if err := repo.MarkPublished(ctx, nil); err != nil {
		t.Fatalf("expected a no-op, got %v", err)
	}
}
