package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func TestResolveTemplate_LeafWithOwnStepsWinsOverParent(t *testing.T) {
	repo := newFakeTemplateRepository()
	createUC := NewCreateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	parent, err := createUC.Execute(ctx, CreateTemplateInput{
		Name: "company-base", Scope: "company",
		DAGJSON: `{"steps":[{"id":"s1","type":"webhook"}]}`,
	})
	if err != nil {
		t.Fatalf("creating parent: %v", err)
	}
	child, err := createUC.Execute(ctx, CreateTemplateInput{
		Name: "personal-override", Scope: "personal", ParentTemplateID: parent.ID,
		DAGJSON: `{"steps":[{"id":"s1","type":"shell"}]}`,
	})
	if err != nil {
		t.Fatalf("creating child: %v", err)
	}

	uc := NewResolveTemplate(repo)
	out, err := uc.Execute(ctx, ResolveTemplateInput{TemplateID: child.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Template.ID != child.ID {
		t.Errorf("expected the leaf (own steps) to win, got template %q", out.Template.ID)
	}
	if len(out.Chain) != 2 || out.Chain[0].ID != parent.ID || out.Chain[1].ID != child.ID {
		t.Errorf("expected root-first chain [parent, child], got %+v", out.Chain)
	}
}

func TestResolveTemplate_EmptyLeafInheritsFromParent(t *testing.T) {
	repo := newFakeTemplateRepository()
	createUC := NewCreateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	parent, err := createUC.Execute(ctx, CreateTemplateInput{
		Name: "team-base", Scope: "team",
		DAGJSON: `{"steps":[{"id":"s1","type":"webhook"}]}`,
	})
	if err != nil {
		t.Fatalf("creating parent: %v", err)
	}
	// A personal template that exists only to opt into the team template's
	// steps — no steps of its own.
	child, err := createUC.Execute(ctx, CreateTemplateInput{
		Name: "personal-passthrough", Scope: "personal", ParentTemplateID: parent.ID,
		DAGJSON: `{"steps":[]}`,
	})
	if err != nil {
		t.Fatalf("creating child: %v", err)
	}

	uc := NewResolveTemplate(repo)
	out, err := uc.Execute(ctx, ResolveTemplateInput{TemplateID: child.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Template.ID != parent.ID {
		t.Errorf("expected resolution to fall back to the non-empty parent, got template %q", out.Template.ID)
	}
}

func TestResolveTemplate_AllEmptyReturnsLeafItself(t *testing.T) {
	repo := newFakeTemplateRepository()
	createUC := NewCreateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	tmpl, err := createUC.Execute(ctx, CreateTemplateInput{Name: "root", Scope: "personal", DAGJSON: `{"steps":[]}`})
	if err != nil {
		t.Fatalf("creating template: %v", err)
	}

	uc := NewResolveTemplate(repo)
	out, err := uc.Execute(ctx, ResolveTemplateInput{TemplateID: tmpl.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Template.ID != tmpl.ID {
		t.Errorf("expected the template itself (no ancestor has steps either) as a valid, empty answer, got %q", out.Template.ID)
	}
}

func TestResolveTemplate_NotFound(t *testing.T) {
	repo := newFakeTemplateRepository()
	uc := NewResolveTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ResolveTemplateInput{TemplateID: "does-not-exist"})
	if err == nil {
		t.Fatal("expected a not-found error")
	}
}

func TestResolveTemplate_RequiresTemplateID(t *testing.T) {
	repo := newFakeTemplateRepository()
	uc := NewResolveTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ResolveTemplateInput{})
	if err == nil {
		t.Fatal("expected an error for an empty template_id")
	}
}

// --- TASK-WF-004-01: overrides/injectSteps/removeSteps fold ---
//
// These tests construct templates directly via repo.CreateTemplate
// (bypassing usecase.CreateTemplate, whose CreateTemplateInput doesn't
// expose the 3 new fields — that wiring is explicitly out of this task's
// stated scope) so Overrides/InjectSteps/RemoveSteps can be set.

func TestResolveTemplate_Regression_NoNewFieldsResolvesByteIdenticalToBase(t *testing.T) {
	repo := newFakeTemplateRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")

	parentDAG := `{"steps":[{"id":"s1","type":"webhook"}]}`
	parent := domain.WorkflowTemplate{ID: "parent", TenantID: "tenant-1", Name: "parent", Scope: domain.ScopeCompany, DAGJSON: parentDAG, Version: 1}
	child := domain.WorkflowTemplate{ID: "child", TenantID: "tenant-1", Name: "child", Scope: domain.ScopePersonal, ParentTemplateID: "parent", DAGJSON: `{"steps":[]}`, Version: 1}
	if err := repo.CreateTemplate(ctx, parent); err != nil {
		t.Fatalf("creating parent: %v", err)
	}
	if err := repo.CreateTemplate(ctx, child); err != nil {
		t.Fatalf("creating child: %v", err)
	}

	uc := NewResolveTemplate(repo)
	out, err := uc.Execute(ctx, ResolveTemplateInput{TemplateID: "child"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Byte-for-byte, not just semantic equality — this is the regression
	// this task's own must-preserve constraint calls out explicitly: no
	// ParseDAG->Serialize round-trip should happen at all when no
	// descendant used any of the 3 new fields.
	if out.Template.DAGJSON != parentDAG {
		t.Errorf("expected DAGJSON to be byte-identical to the parent's, got %q want %q", out.Template.DAGJSON, parentDAG)
	}
}

func TestResolveTemplate_OverridesChangesOneStepField(t *testing.T) {
	repo := newFakeTemplateRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")

	parent := domain.WorkflowTemplate{
		ID: "parent", TenantID: "tenant-1", Name: "parent", Scope: domain.ScopeCompany, Version: 1,
		DAGJSON: `{"steps":[{"id":"s1","type":"agent","config":{"prompt":"original"}}]}`,
	}
	child := domain.WorkflowTemplate{
		ID: "child", TenantID: "tenant-1", Name: "child", Scope: domain.ScopePersonal, ParentTemplateID: "parent", Version: 1,
		DAGJSON:   `{"steps":[]}`,
		Overrides: map[string]any{"s1.prompt": "overridden"},
	}
	_ = repo.CreateTemplate(ctx, parent)
	_ = repo.CreateTemplate(ctx, child)

	uc := NewResolveTemplate(repo)
	out, err := uc.Execute(ctx, ResolveTemplateInput{TemplateID: "child"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dag, err := domain.ParseDAG(out.Template.DAGJSON)
	if err != nil {
		t.Fatalf("parsing resolved dag: %v", err)
	}
	if len(dag.Steps) != 1 {
		t.Fatalf("expected exactly 1 step, got %d", len(dag.Steps))
	}
	var cfg map[string]any
	if err := json.Unmarshal(dag.Steps[0].Config, &cfg); err != nil {
		t.Fatalf("unmarshal step config: %v", err)
	}
	if cfg["prompt"] != "overridden" {
		t.Errorf("expected prompt to be overridden, got %v", cfg["prompt"])
	}
	if dag.Steps[0].Type != domain.StepTypeAgent {
		t.Errorf("expected every other field unchanged (type), got %v", dag.Steps[0].Type)
	}
}

func TestResolveTemplate_OverridesUnknownStepIDErrors(t *testing.T) {
	repo := newFakeTemplateRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")

	parent := domain.WorkflowTemplate{ID: "parent", TenantID: "tenant-1", Name: "parent", Scope: domain.ScopeCompany, Version: 1, DAGJSON: `{"steps":[{"id":"s1","type":"agent"}]}`}
	child := domain.WorkflowTemplate{
		ID: "child", TenantID: "tenant-1", Name: "child", Scope: domain.ScopePersonal, ParentTemplateID: "parent", Version: 1,
		DAGJSON: `{"steps":[]}`, Overrides: map[string]any{"does-not-exist.prompt": "x"},
	}
	_ = repo.CreateTemplate(ctx, parent)
	_ = repo.CreateTemplate(ctx, child)

	uc := NewResolveTemplate(repo)
	_, err := uc.Execute(ctx, ResolveTemplateInput{TemplateID: "child"})
	if err == nil {
		t.Fatal("expected an error for an override referencing an unknown step id")
	}
}

func TestResolveTemplate_InjectStepsAddsStepAfterAnchor(t *testing.T) {
	repo := newFakeTemplateRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")

	parent := domain.WorkflowTemplate{ID: "parent", TenantID: "tenant-1", Name: "parent", Scope: domain.ScopeCompany, Version: 1, DAGJSON: `{"steps":[{"id":"s1","type":"webhook"},{"id":"s2","type":"webhook"}]}`}
	child := domain.WorkflowTemplate{
		ID: "child", TenantID: "tenant-1", Name: "child", Scope: domain.ScopePersonal, ParentTemplateID: "parent", Version: 1,
		DAGJSON: `{"steps":[]}`,
		InjectSteps: []domain.StepInjection{
			{AnchorStepID: "s1", Position: "after", Step: domain.Step{ID: "injected", Type: domain.StepTypeNotification}},
		},
	}
	_ = repo.CreateTemplate(ctx, parent)
	_ = repo.CreateTemplate(ctx, child)

	uc := NewResolveTemplate(repo)
	out, err := uc.Execute(ctx, ResolveTemplateInput{TemplateID: "child"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dag, err := domain.ParseDAG(out.Template.DAGJSON)
	if err != nil {
		t.Fatalf("parsing resolved dag: %v", err)
	}
	wantOrder := []string{"s1", "injected", "s2"}
	if len(dag.Steps) != len(wantOrder) {
		t.Fatalf("expected %d steps, got %d: %+v", len(wantOrder), len(dag.Steps), dag.Steps)
	}
	for i, want := range wantOrder {
		if dag.Steps[i].ID != want {
			t.Errorf("position %d: expected %s, got %s", i, want, dag.Steps[i].ID)
		}
	}
}

func TestResolveTemplate_InjectStepsUnknownAnchorIsSkippedNotError(t *testing.T) {
	repo := newFakeTemplateRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")

	parent := domain.WorkflowTemplate{ID: "parent", TenantID: "tenant-1", Name: "parent", Scope: domain.ScopeCompany, Version: 1, DAGJSON: `{"steps":[{"id":"s1","type":"webhook"}]}`}
	child := domain.WorkflowTemplate{
		ID: "child", TenantID: "tenant-1", Name: "child", Scope: domain.ScopePersonal, ParentTemplateID: "parent", Version: 1,
		DAGJSON:     `{"steps":[]}`,
		InjectSteps: []domain.StepInjection{{AnchorStepID: "does-not-exist", Position: "after", Step: domain.Step{ID: "injected", Type: domain.StepTypeNotification}}},
	}
	_ = repo.CreateTemplate(ctx, parent)
	_ = repo.CreateTemplate(ctx, child)

	uc := NewResolveTemplate(repo)
	out, err := uc.Execute(ctx, ResolveTemplateInput{TemplateID: "child"})
	if err != nil {
		t.Fatalf("unexpected error (a stale anchor should be skipped, not fail resolution): %v", err)
	}
	dag, _ := domain.ParseDAG(out.Template.DAGJSON)
	if len(dag.Steps) != 1 {
		t.Errorf("expected the injection to be silently skipped, got %d steps", len(dag.Steps))
	}
}

func TestResolveTemplate_RemoveStepsPrunesStepAndDanglingDependsOn(t *testing.T) {
	repo := newFakeTemplateRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")

	parent := domain.WorkflowTemplate{
		ID: "parent", TenantID: "tenant-1", Name: "parent", Scope: domain.ScopeCompany, Version: 1,
		DAGJSON: `{"steps":[{"id":"s1","type":"webhook"},{"id":"s2","type":"webhook","dependsOn":["s1"]}]}`,
	}
	child := domain.WorkflowTemplate{
		ID: "child", TenantID: "tenant-1", Name: "child", Scope: domain.ScopePersonal, ParentTemplateID: "parent", Version: 1,
		DAGJSON: `{"steps":[]}`, RemoveSteps: []string{"s1"},
	}
	_ = repo.CreateTemplate(ctx, parent)
	_ = repo.CreateTemplate(ctx, child)

	uc := NewResolveTemplate(repo)
	out, err := uc.Execute(ctx, ResolveTemplateInput{TemplateID: "child"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dag, _ := domain.ParseDAG(out.Template.DAGJSON)
	if len(dag.Steps) != 1 || dag.Steps[0].ID != "s2" {
		t.Fatalf("expected only s2 to remain, got %+v", dag.Steps)
	}
	if len(dag.Steps[0].DependsOn) != 0 {
		t.Errorf("expected the dangling dependsOn on the removed step to be pruned, got %v", dag.Steps[0].DependsOn)
	}
	// The result must also pass Validate — proves no dangling reference
	// survived the fold.
	if err := dag.Validate(); err != nil {
		t.Errorf("expected the pruned DAG to still validate, got %v", err)
	}
}

func TestResolveTemplate_OverridesInjectionsAndRemovalsCombined(t *testing.T) {
	repo := newFakeTemplateRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")

	parent := domain.WorkflowTemplate{
		ID: "parent", TenantID: "tenant-1", Name: "parent", Scope: domain.ScopeCompany, Version: 1,
		DAGJSON: `{"steps":[{"id":"s1","type":"agent","config":{"prompt":"original"}},{"id":"s2","type":"webhook"}]}`,
	}
	child := domain.WorkflowTemplate{
		ID: "child", TenantID: "tenant-1", Name: "child", Scope: domain.ScopePersonal, ParentTemplateID: "parent", Version: 1,
		DAGJSON:     `{"steps":[]}`,
		Overrides:   map[string]any{"s1.prompt": "overridden"},
		InjectSteps: []domain.StepInjection{{AnchorStepID: "s1", Position: "after", Step: domain.Step{ID: "s3", Type: domain.StepTypeNotification}}},
		RemoveSteps: []string{"s2"},
	}
	_ = repo.CreateTemplate(ctx, parent)
	_ = repo.CreateTemplate(ctx, child)

	uc := NewResolveTemplate(repo)
	out, err := uc.Execute(ctx, ResolveTemplateInput{TemplateID: "child"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dag, _ := domain.ParseDAG(out.Template.DAGJSON)
	if len(dag.Steps) != 2 || dag.Steps[0].ID != "s1" || dag.Steps[1].ID != "s3" {
		t.Fatalf("expected [s1, s3] (s2 removed, s3 injected after s1), got %+v", dag.Steps)
	}
	var cfg map[string]any
	_ = json.Unmarshal(dag.Steps[0].Config, &cfg)
	if cfg["prompt"] != "overridden" {
		t.Errorf("expected s1's prompt override to also apply, got %v", cfg["prompt"])
	}
}

// TestResolveTemplate_OverridesOnEmptyDAGTemplateDoesNotMakeItBase confirms
// base-selection is untouched: a personal template with only Overrides set
// (and an empty dag_json) must still resolve using its parent's steps as
// base — Overrides on an empty-DAG template applies to whatever base the
// existing algorithm picks, it does not itself make that template base.
func TestResolveTemplate_OverridesOnEmptyDAGTemplateDoesNotMakeItBase(t *testing.T) {
	repo := newFakeTemplateRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")

	parent := domain.WorkflowTemplate{ID: "parent", TenantID: "tenant-1", Name: "parent", Scope: domain.ScopeCompany, Version: 1, DAGJSON: `{"steps":[{"id":"s1","type":"agent","config":{"prompt":"original"}}]}`}
	child := domain.WorkflowTemplate{
		ID: "child", TenantID: "tenant-1", Name: "child", Scope: domain.ScopePersonal, ParentTemplateID: "parent", Version: 1,
		DAGJSON:   `{"steps":[]}`, // empty — child must NOT become base despite having Overrides
		Overrides: map[string]any{"s1.prompt": "overridden"},
	}
	_ = repo.CreateTemplate(ctx, parent)
	_ = repo.CreateTemplate(ctx, child)

	uc := NewResolveTemplate(repo)
	out, err := uc.Execute(ctx, ResolveTemplateInput{TemplateID: "child"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// base is still parent (id "parent" survives into the response as the
	// resolved template's identity fields, DAGJSON mutated) — confirmed by
	// checking the resolved template's other identity fields came from
	// parent, not child.
	if out.Template.ID != "parent" || out.Template.Name != "parent" {
		t.Fatalf("expected base-selection to still pick parent (Overrides does not make an empty-dag template become base), got %+v", out.Template)
	}
	dag, _ := domain.ParseDAG(out.Template.DAGJSON)
	var cfg map[string]any
	_ = json.Unmarshal(dag.Steps[0].Config, &cfg)
	if cfg["prompt"] != "overridden" {
		t.Errorf("expected the override to still apply onto whatever base was picked, got %v", cfg["prompt"])
	}
}
