//go:build integration

// Integration tests run against a real Postgres via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/postgres/...`.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// jsonEqual compares two JSON strings structurally rather than
// byte-for-byte — Postgres reformats JSONB on read-back (e.g. adds a space
// after ':'), so a round-tripped output column never matches the literal
// string that was written even when the content is identical.
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
	dsn := testutil.StartPostgres(t, "workflow")

	migrationsPath, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	cmd := exec.Command("migrate", "-path", migrationsPath, "-database", dsn, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running migrations: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting to postgres: %v", err)
	}
	t.Cleanup(pool.Close)

	return New(pool)
}

func TestRepository_CreateAndGetTemplate(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tmpl, err := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000001", "11111111-1111-1111-1111-111111111111", "deploy", `{"steps":[]}`, domain.ScopePersonal, "")
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

func TestRepository_ExecutionPauseResumeRoundTrip(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "22222222-2222-2222-2222-222222222222"

	tmpl, _ := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000002", tenantID, "release", `{"steps":[]}`, domain.ScopeTeam, "")
	_ = repo.CreateTemplate(ctx, tmpl)

	exec, err := domain.NewWorkflowExecution("dddddddd-0000-0000-0000-000000000001", tenantID, tmpl.ID, "trace-1", "")
	if err != nil {
		t.Fatalf("building execution: %v", err)
	}
	if err := repo.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	if err := exec.Pause(time.Now().UTC()); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if err := repo.UpdateExecution(ctx, exec); err != nil {
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

// TestRepository_CreateExecution_PersistsInputsJSON exercises
// TASK-WF-003-01's InputsJSON round-trip — frozen at Execute time so a
// RecoverExecutions resume after a restart still has the original inputs
// available for interpolation.
func TestRepository_CreateExecution_PersistsInputsJSON(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "77777777-7777-7777-7777-777777777777"

	tmpl, _ := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000004", tenantID, "inputs-test", `{"steps":[]}`, domain.ScopePersonal, "")
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	exec, err := domain.NewWorkflowExecution("dddddddd-0000-0000-0000-000000000020", tenantID, tmpl.ID, "trace-inputs", "")
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

// TestRepository_CreateExecution_EmptyInputsJSONRoundTripsEmpty confirms an
// execution with no inputs at all persists and reads back as an empty
// string, not a spurious "null" or "{}".
func TestRepository_CreateExecution_EmptyInputsJSONRoundTripsEmpty(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "88888888-8888-8888-8888-888888888888"

	tmpl, _ := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000005", tenantID, "no-inputs-test", `{"steps":[]}`, domain.ScopePersonal, "")
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	exec, err := domain.NewWorkflowExecution("dddddddd-0000-0000-0000-000000000021", tenantID, tmpl.ID, "trace-no-inputs", "")
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
	tenantID := "33333333-3333-3333-3333-333333333333"

	ids := []string{
		"aaaaaaaa-0000-0000-0000-000000000001",
		"aaaaaaaa-0000-0000-0000-000000000002",
		"aaaaaaaa-0000-0000-0000-000000000003",
	}
	for _, id := range ids {
		tmpl, err := domain.NewWorkflowTemplate(id, tenantID, "t-"+id, `{"steps":[]}`, domain.ScopePersonal, "")
		if err != nil {
			t.Fatalf("building template %s: %v", id, err)
		}
		if err := repo.CreateTemplate(ctx, tmpl); err != nil {
			t.Fatalf("create template %s: %v", id, err)
		}
	}

	firstPage, next, err := repo.ListTemplates(ctx, tenantID, "", "", 2)
	if err != nil {
		t.Fatalf("list templates (page 1): %v", err)
	}
	if len(firstPage) != 2 || next == "" {
		t.Fatalf("expected a full first page with a next token, got %d rows, next=%q", len(firstPage), next)
	}

	secondPage, next2, err := repo.ListTemplates(ctx, tenantID, "", next, 2)
	if err != nil {
		t.Fatalf("list templates (page 2): %v", err)
	}
	if len(secondPage) != 1 || next2 != "" {
		t.Fatalf("expected exactly one remaining template and no further page, got %d rows, next=%q", len(secondPage), next2)
	}
}

// TestRepository_ListExecutions_KeysetPaginationNewestFirst exercises
// TASK-WF-005-01's ListExecutions — ordering by created_at DESC (not by id,
// since execution ids are random UUIDs with no chronological meaning,
// unlike ListTemplates' id-based cursor above). A short sleep between
// inserts guarantees distinct created_at values so ordering is
// unambiguous.
func TestRepository_ListExecutions_KeysetPaginationNewestFirst(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "55555555-5555-5555-5555-555555555555"
	projectID := "66666666-6666-6666-6666-666666666666"

	tmpl, _ := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000003", tenantID, "list-exec", `{"steps":[]}`, domain.ScopePersonal, "")
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	ids := []string{
		"dddddddd-0000-0000-0000-000000000010",
		"dddddddd-0000-0000-0000-000000000011",
		"dddddddd-0000-0000-0000-000000000012",
	}
	for _, id := range ids {
		exec, err := domain.NewWorkflowExecution(id, tenantID, tmpl.ID, "trace-"+id, projectID)
		if err != nil {
			t.Fatalf("building execution %s: %v", id, err)
		}
		if err := repo.CreateExecution(ctx, exec); err != nil {
			t.Fatalf("create execution %s: %v", id, err)
		}
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
	tenantID := "44444444-4444-4444-4444-444444444444"

	root, err := domain.NewWorkflowTemplate("aaaaaaaa-1111-0000-0000-000000000001", tenantID, "company-base", `{"steps":[{"id":"s1","type":"webhook"}]}`, domain.ScopeCompany, "")
	if err != nil {
		t.Fatalf("building root: %v", err)
	}
	if err := repo.CreateTemplate(ctx, root); err != nil {
		t.Fatalf("create root: %v", err)
	}

	child, err := domain.NewWorkflowTemplate("aaaaaaaa-1111-0000-0000-000000000002", tenantID, "personal-override", `{"steps":[]}`, domain.ScopePersonal, root.ID)
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

	_, err := repo.ResolveChain(ctx, "55555555-5555-5555-5555-555555555555", "aaaaaaaa-0000-0000-0000-000000009999", 5)
	if err == nil {
		t.Fatal("expected an error resolving a chain for a template that doesn't exist")
	}
}

func TestRepository_Update_CorrectVersion_Succeeds(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "66666666-6666-6666-6666-666666666666"

	tmpl, err := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000003", tenantID, "deploy", `{"steps":[]}`, domain.ScopePersonal, "")
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

func TestRepository_Update_StaleVersion_ReturnsConflict(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "77777777-7777-7777-7777-777777777777"

	tmpl, err := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000004", tenantID, "deploy", `{"steps":[]}`, domain.ScopePersonal, "")
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

// TestRepository_Update_BumpFalse_DoesNotIncrementVersion covers
// TASK-WF-004-03: bump=false must leave templates.version unchanged while
// still applying every other field — and the WHERE clause's version-match
// check (ErrTemplateVersionConflict's trigger) must stay unaffected by
// bump's value, proven by chaining a second bump=false update using the
// SAME (unbumped) expectedVersion.
func TestRepository_Update_BumpFalse_DoesNotIncrementVersion(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "99999999-9999-9999-9999-999999999999"

	tmpl, err := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000006", tenantID, "deploy", `{"steps":[]}`, domain.ScopePersonal, "")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	tmpl.Name = "deploy-renamed"
	updated, err := repo.Update(ctx, tmpl, 1, false)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Version != 1 {
		t.Fatalf("want version unchanged at 1 (bump=false), got %d", updated.Version)
	}
	if updated.Name != "deploy-renamed" {
		t.Fatalf("expected the name field to still apply even with bump=false, got %q", updated.Name)
	}

	// The version-match WHERE clause must be unaffected by bump=false —
	// a second update against the SAME expectedVersion=1 (still correct,
	// since the first update didn't bump it) must also succeed.
	tmpl.Name = "deploy-renamed-again"
	updated2, err := repo.Update(ctx, tmpl, 1, false)
	if err != nil {
		t.Fatalf("second update (same expectedVersion, proving the WHERE clause is unaffected by bump): %v", err)
	}
	if updated2.Version != 1 {
		t.Fatalf("want version still unchanged at 1, got %d", updated2.Version)
	}
}

// TestRepository_HasActiveExecutionsUsingTemplate backs TASK-WF-004-03's
// version-bump decision — same non-terminal status set
// HasActiveExecutions already uses, scoped by template_id instead of
// project_id.
func TestRepository_HasActiveExecutionsUsingTemplate(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "aaaaaaaa-1111-1111-1111-111111111111"

	tmpl, err := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000007", tenantID, "deploy", `{"steps":[]}`, domain.ScopePersonal, "")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	hasActive, err := repo.HasActiveExecutionsUsingTemplate(ctx, tenantID, tmpl.ID)
	if err != nil {
		t.Fatalf("has active executions using template (before any execution): %v", err)
	}
	if hasActive {
		t.Fatal("expected no active executions before any execution exists")
	}

	exec, err := domain.NewWorkflowExecution("dddddddd-0000-0000-0000-000000000030", tenantID, tmpl.ID, "trace-1", "")
	if err != nil {
		t.Fatalf("building execution: %v", err)
	}
	if err := repo.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	hasActive, err = repo.HasActiveExecutionsUsingTemplate(ctx, tenantID, tmpl.ID)
	if err != nil {
		t.Fatalf("has active executions using template (running execution exists): %v", err)
	}
	if !hasActive {
		t.Fatal("expected an active (running) execution referencing this template to be found")
	}

	exec.Status = domain.StatusCompleted
	if err := repo.UpdateExecution(ctx, exec); err != nil {
		t.Fatalf("marking execution completed: %v", err)
	}
	hasActive, err = repo.HasActiveExecutionsUsingTemplate(ctx, tenantID, tmpl.ID)
	if err != nil {
		t.Fatalf("has active executions using template (completed execution): %v", err)
	}
	if hasActive {
		t.Fatal("expected a completed (terminal) execution to not count as active")
	}
}

// TestRepository_CreateExecution_AdHocNullTemplateID exercises migration
// 0005_execution_ad_hoc_template: ExecuteAdHocStep's synthetic execution
// (domain.NewAdHocWorkflowExecution) has no backing template, which
// requires workflow.executions.template_id to actually accept NULL.
func TestRepository_CreateExecution_AdHocNullTemplateID(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "66666666-6666-6666-6666-666666666666"

	exec, err := domain.NewAdHocWorkflowExecution("dddddddd-0000-0000-0000-000000000002", tenantID, "trace-adhoc")
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

// TestRepository_StepExecution_CreateAndUpdateRoundTrip exercises the new
// step_executions table/migration end-to-end: create a real (templated)
// execution, persist a pending step execution under it, transition it to
// completed with output, and read it back via ListStepExecutions.
func TestRepository_StepExecution_CreateAndUpdateRoundTrip(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "77777777-7777-7777-7777-777777777777"

	tmpl, err := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000003", tenantID, "step-exec-template", `{"steps":[{"id":"a","type":"shell"}]}`, domain.ScopePersonal, "")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	exec, err := domain.NewWorkflowExecution("dddddddd-0000-0000-0000-000000000003", tenantID, tmpl.ID, "trace-step", "")
	if err != nil {
		t.Fatalf("building execution: %v", err)
	}
	if err := repo.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	se, err := domain.NewStepExecution("eeeeeeee-0000-0000-0000-000000000001", exec.ID, "a", "ffffffff-0000-0000-0000-000000000001", 0)
	if err != nil {
		t.Fatalf("building step execution: %v", err)
	}
	if err := repo.CreateStepExecution(ctx, se); err != nil {
		t.Fatalf("create step execution: %v", err)
	}

	se.MarkRunning()
	if err := repo.UpdateStepExecution(ctx, se); err != nil {
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
	if err := repo.UpdateStepExecution(ctx, se); err != nil {
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
	if got.Wave != 0 {
		t.Errorf("expected wave=0, got %d", got.Wave)
	}
	if got.DispatchToken == "" {
		t.Error("expected dispatch_token to round-trip")
	}
}

// TestRepository_ListRunning_ReturnsOnlyRunningAcrossTenants exercises
// usecase.RecoverExecutions' boot-time scan query end-to-end: it must
// return status=running executions regardless of which tenant owns them
// (the one deliberately unscoped query in this repository — see
// ExecutionRepository.ListRunning's doc comment), and it must exclude
// paused/completed rows.
func TestRepository_ListRunning_ReturnsOnlyRunningAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantA := "aaaaaaaa-2222-0000-0000-000000000001"
	tenantB := "aaaaaaaa-2222-0000-0000-000000000002"

	tmplA, _ := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000005", tenantA, "t-a", `{"steps":[]}`, domain.ScopePersonal, "")
	_ = repo.CreateTemplate(ctx, tmplA)
	tmplB, _ := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000006", tenantB, "t-b", `{"steps":[]}`, domain.ScopePersonal, "")
	_ = repo.CreateTemplate(ctx, tmplB)

	running, err := domain.NewWorkflowExecution("dddddddd-0000-0000-0000-000000000005", tenantA, tmplA.ID, "trace-running", "")
	if err != nil {
		t.Fatalf("building running execution: %v", err)
	}
	if err := repo.CreateExecution(ctx, running); err != nil {
		t.Fatalf("create running execution: %v", err)
	}

	runningOtherTenant, err := domain.NewWorkflowExecution("dddddddd-0000-0000-0000-000000000006", tenantB, tmplB.ID, "trace-running-b", "")
	if err != nil {
		t.Fatalf("building second running execution: %v", err)
	}
	if err := repo.CreateExecution(ctx, runningOtherTenant); err != nil {
		t.Fatalf("create second running execution: %v", err)
	}

	paused, err := domain.NewWorkflowExecution("dddddddd-0000-0000-0000-000000000007", tenantA, tmplA.ID, "trace-paused", "")
	if err != nil {
		t.Fatalf("building paused execution: %v", err)
	}
	if err := repo.CreateExecution(ctx, paused); err != nil {
		t.Fatalf("create paused execution: %v", err)
	}
	if err := paused.Pause(time.Now().UTC()); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if err := repo.UpdateExecution(ctx, paused); err != nil {
		t.Fatalf("update (pause): %v", err)
	}

	completed, err := domain.NewWorkflowExecution("dddddddd-0000-0000-0000-000000000008", tenantA, tmplA.ID, "trace-completed", "")
	if err != nil {
		t.Fatalf("building completed execution: %v", err)
	}
	if err := repo.CreateExecution(ctx, completed); err != nil {
		t.Fatalf("create completed execution: %v", err)
	}
	completed.Status = domain.StatusCompleted
	if err := repo.UpdateExecution(ctx, completed); err != nil {
		t.Fatalf("update (completed): %v", err)
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
	if gotIDs[completed.ID] {
		t.Errorf("expected completed execution %s to be excluded", completed.ID)
	}
}

// TestRepository_ListStepExecutions_ScopedByTenant proves ListStepExecutions'
// tenant join actually excludes another tenant's execution_id, not just
// filters by a caller-supplied tenantID string.
func TestRepository_ListStepExecutions_ScopedByTenant(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantA := "88888888-8888-8888-8888-888888888888"
	tenantB := "99999999-9999-9999-9999-999999999999"

	tmpl, _ := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000004", tenantA, "t", `{"steps":[]}`, domain.ScopePersonal, "")
	_ = repo.CreateTemplate(ctx, tmpl)
	exec, _ := domain.NewWorkflowExecution("dddddddd-0000-0000-0000-000000000004", tenantA, tmpl.ID, "trace", "")
	_ = repo.CreateExecution(ctx, exec)
	se, _ := domain.NewStepExecution("eeeeeeee-0000-0000-0000-000000000002", exec.ID, "a", "ffffffff-0000-0000-0000-000000000002", 0)
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
