package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
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
	// The effective template keeps the REQUESTED (leaf) template's own
	// identity — ResolveTemplate answers "what does child effectively look
	// like," which is child's id carrying the parent's inherited steps
	// since child itself defines none.
	if out.Template.ID != child.ID {
		t.Errorf("expected effective template to keep the requested template's identity, got %q", out.Template.ID)
	}
	if !jsonEqualSteps(t, out.Template.DAGJSON, `{"steps":[{"id":"s1","type":"webhook"}]}`) {
		t.Errorf("expected resolution to fall back to the non-empty parent's steps, got dag_json=%s", out.Template.DAGJSON)
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

// jsonEqualSteps compares two dag_json strings structurally (field order
// and marshal whitespace shouldn't matter) rather than byte-for-byte.
func jsonEqualSteps(t *testing.T, a, b string) bool {
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

// mustNewTemplate builds and directly persists (bypassing CreateTemplate's
// usecase, which doesn't yet expose Inherit-mode fields on its input — see
// TASK-WF-01-07) a WorkflowTemplate with the given Inherit-mode options,
// for tests that need overrides/inject_steps/remove_steps on a specific
// chain level.
func mustNewTemplate(t *testing.T, repo *fakeTemplateRepository, id, tenantID, name, dagJSON, parentID string, opts ...domain.TemplateOption) domain.WorkflowTemplate {
	t.Helper()
	tmpl, err := domain.NewWorkflowTemplate(id, tenantID, name, dagJSON, domain.ScopePersonal, parentID, "owner-1", opts...)
	if err != nil {
		t.Fatalf("building template %s: %v", id, err)
	}
	if err := repo.CreateTemplate(context.Background(), tmpl); err != nil {
		t.Fatalf("persisting template %s: %v", id, err)
	}
	return tmpl
}

func TestResolveTemplate_OverrideMergesOntoInheritedStep(t *testing.T) {
	repo := newFakeTemplateRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")

	// 3-level chain: root -> middle (overrides root's step) -> leaf (no
	// steps/overrides of its own, pure passthrough).
	root := mustNewTemplate(t, repo, "root", "tenant-1", "root",
		`{"steps":[{"id":"s1","type":"shell","config":{"cmd":"echo root","timeout":30}}]}`, "")
	middle := mustNewTemplate(t, repo, "middle", "tenant-1", "middle", `{"steps":[]}`, root.ID,
		domain.WithOverrides(`{"s1":{"cmd":"echo middle"}}`))
	leaf := mustNewTemplate(t, repo, "leaf", "tenant-1", "leaf", `{"steps":[]}`, middle.ID)

	uc := NewResolveTemplate(repo)
	out, err := uc.Execute(ctx, ResolveTemplateInput{TemplateID: leaf.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !jsonEqualSteps(t, out.Template.DAGJSON, `{"steps":[{"id":"s1","type":"shell","config":{"cmd":"echo middle","timeout":30}}]}`) {
		t.Errorf("expected override to merge onto inherited step's Config (other fields surviving), got %s", out.Template.DAGJSON)
	}
}

func TestResolveTemplate_RemoveStepsDropsStepAndDanglingDependency(t *testing.T) {
	repo := newFakeTemplateRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")

	root := mustNewTemplate(t, repo, "root", "tenant-1", "root",
		`{"steps":[{"id":"s1","type":"shell"},{"id":"s2","type":"shell","dependsOn":["s1"]}]}`, "")
	leaf := mustNewTemplate(t, repo, "leaf", "tenant-1", "leaf", `{"steps":[]}`, root.ID,
		domain.WithRemoveSteps(`["s1"]`))

	uc := NewResolveTemplate(repo)
	out, err := uc.Execute(ctx, ResolveTemplateInput{TemplateID: leaf.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !jsonEqualSteps(t, out.Template.DAGJSON, `{"steps":[{"id":"s2","type":"shell"}]}`) {
		t.Errorf("expected s1 removed and stripped from s2's dependsOn, got %s", out.Template.DAGJSON)
	}
}

func TestResolveTemplate_InjectStepsAppendsReferencingInheritedStep(t *testing.T) {
	repo := newFakeTemplateRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")

	root := mustNewTemplate(t, repo, "root", "tenant-1", "root", `{"steps":[{"id":"s1","type":"shell"}]}`, "")
	leaf := mustNewTemplate(t, repo, "leaf", "tenant-1", "leaf", `{"steps":[]}`, root.ID,
		domain.WithInjectSteps(`[{"id":"s2","type":"notification","dependsOn":["s1"]}]`))

	uc := NewResolveTemplate(repo)
	out, err := uc.Execute(ctx, ResolveTemplateInput{TemplateID: leaf.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !jsonEqualSteps(t, out.Template.DAGJSON,
		`{"steps":[{"id":"s1","type":"shell"},{"id":"s2","type":"notification","dependsOn":["s1"]}]}`) {
		t.Errorf("expected injected step appended referencing inherited s1, got %s", out.Template.DAGJSON)
	}
}

func TestResolveTemplate_NoInheritFieldsAnywhereMatchesOldBehavior(t *testing.T) {
	// Regression: the exact fixture from
	// TestResolveTemplate_LeafWithOwnStepsWinsOverParent, reused verbatim —
	// a chain with none of overrides/inject_steps/remove_steps set anywhere
	// must resolve identically to the pre-deepMerge policy (own non-empty
	// steps win, scoped per-level).
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
	if !jsonEqualSteps(t, out.Template.DAGJSON, `{"steps":[{"id":"s1","type":"shell"}]}`) {
		t.Errorf("expected leaf's own steps to win unchanged, got %s", out.Template.DAGJSON)
	}
}

func TestResolveTemplate_MergeProducingCyclicDAGIsInvalid(t *testing.T) {
	repo := newFakeTemplateRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")

	root := mustNewTemplate(t, repo, "root", "tenant-1", "root", `{"steps":[{"id":"s1","type":"shell"}]}`, "")
	// inject_steps introduces a step that depends on itself — a direct
	// cycle Validate() rejects post-merge.
	leaf := mustNewTemplate(t, repo, "leaf", "tenant-1", "leaf", `{"steps":[]}`, root.ID,
		domain.WithInjectSteps(`[{"id":"s2","type":"shell","dependsOn":["s2"]}]`))

	uc := NewResolveTemplate(repo)
	_, err := uc.Execute(ctx, ResolveTemplateInput{TemplateID: leaf.ID})
	if err == nil {
		t.Fatal("expected a merge producing a cyclic DAG to fail validation")
	}
	var apiErr *apperrors.AppError
	if !errors.As(err, &apiErr) || apiErr.Code != "WORKFLOW_INVALID_TEMPLATE" {
		t.Fatalf("expected WORKFLOW_INVALID_TEMPLATE, got %v", err)
	}
}
