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

	tmpl, err := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000001", "11111111-1111-1111-1111-111111111111", "deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
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

	tmpl, _ := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000002", tenantID, "release", `{"steps":[]}`, domain.ScopeTeam, "", "owner-1")
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
		tmpl, err := domain.NewWorkflowTemplate(id, tenantID, "t-"+id, `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
		if err != nil {
			t.Fatalf("building template %s: %v", id, err)
		}
		if err := repo.CreateTemplate(ctx, tmpl); err != nil {
			t.Fatalf("create template %s: %v", id, err)
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

func TestRepository_ResolveChain_RootFirstOrder(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "44444444-4444-4444-4444-444444444444"

	root, err := domain.NewWorkflowTemplate("aaaaaaaa-1111-0000-0000-000000000001", tenantID, "company-base", `{"steps":[{"id":"s1","type":"webhook"}]}`, domain.ScopeCompany, "", "owner-1")
	if err != nil {
		t.Fatalf("building root: %v", err)
	}
	if err := repo.CreateTemplate(ctx, root); err != nil {
		t.Fatalf("create root: %v", err)
	}

	child, err := domain.NewWorkflowTemplate("aaaaaaaa-1111-0000-0000-000000000002", tenantID, "personal-override", `{"steps":[]}`, domain.ScopePersonal, root.ID, "owner-1")
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

	tmpl, err := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000003", tenantID, "deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
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
	tenantID := "88888888-8888-8888-8888-888888888888"

	tmpl, err := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000007", tenantID, "deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	tmpl.Name = "deploy-metadata-only"
	tmpl.DAGJSON = `{"steps":[{"id":"s1","type":"webhook"}]}`

	updated, err := repo.Update(ctx, tmpl, 1, false)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Version != 1 {
		t.Fatalf("want version unchanged at 1 when bumpVersion=false, got %d", updated.Version)
	}
	if updated.Name != "deploy-metadata-only" {
		t.Fatalf("update did not persist the new name despite bumpVersion=false: %+v", updated)
	}

	got, err := repo.GetTemplate(ctx, tenantID, tmpl.ID)
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	if got.Version != 1 || got.Name != "deploy-metadata-only" {
		t.Fatalf("re-read template does not reflect the update: %+v", got)
	}
	if !jsonEqual(t, got.DAGJSON, tmpl.DAGJSON) {
		t.Fatalf("re-read dag_json = %q, want structurally equal to %q", got.DAGJSON, tmpl.DAGJSON)
	}

	// expected_version still gates the write even with bumpVersion=false —
	// a second call at the same (unmoved) expected_version must succeed
	// again, proving the WHERE clause isn't accidentally always-true.
	tmpl2 := got
	tmpl2.Name = "deploy-metadata-only-2"
	if _, err := repo.Update(ctx, tmpl2, 1, false); err != nil {
		t.Fatalf("second no-bump update at unmoved expected_version: %v", err)
	}
}

func TestRepository_Update_StaleVersion_ReturnsConflict(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "77777777-7777-7777-7777-777777777777"

	tmpl, err := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000004", tenantID, "deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
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

	tmpl, err := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000003", tenantID, "step-exec-template", `{"steps":[{"id":"a","type":"shell"}]}`, domain.ScopePersonal, "", "owner-1")
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

	tmplA, _ := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000005", tenantA, "t-a", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	_ = repo.CreateTemplate(ctx, tmplA)
	tmplB, _ := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000006", tenantB, "t-b", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
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
	if err := repo.UpdateExecution(ctx, paused, nil); err != nil {
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
	if err := repo.UpdateExecution(ctx, completed, nil); err != nil {
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

	tmpl, _ := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000004", tenantA, "t", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
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

// ── SOL-PW-04 — outbox (TASK-PW-04-05) ────────────────────────────────────

func TestRepository_UpdateExecution_WithEvent_WritesOutboxRowInSameTransaction(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "11111111-1111-1111-1111-111111111111"

	tmpl, _ := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000005", tenantID, "t", `{"steps":[]}`, domain.ScopePersonal, "")
	_ = repo.CreateTemplate(ctx, tmpl)
	exec, _ := domain.NewWorkflowExecution("dddddddd-0000-0000-0000-000000000005", tenantID, tmpl.ID, "trace", "")
	if err := repo.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	exec.Status = domain.StatusCompleted
	event := &domain.OutboxEvent{
		ID:          "eeeeeeee-0000-0000-0000-000000000010",
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
	tenantID := "11111111-1111-1111-1111-111111111112"

	tmpl, _ := domain.NewWorkflowTemplate("cccccccc-0000-0000-0000-000000000006", tenantID, "t", `{"steps":[]}`, domain.ScopePersonal, "")
	_ = repo.CreateTemplate(ctx, tmpl)
	exec, _ := domain.NewWorkflowExecution("dddddddd-0000-0000-0000-000000000006", tenantID, tmpl.ID, "trace", "")
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
