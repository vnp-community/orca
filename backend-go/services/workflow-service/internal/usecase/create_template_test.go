package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func TestCreateTemplate_Succeeds(t *testing.T) {
	repo := newFakeTemplateRepository()
	uc := NewCreateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	tmpl, err := uc.Execute(ctx, CreateTemplateInput{Name: "deploy", DAGJSON: `{"steps":[]}`, Scope: "personal"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpl.Name != "deploy" || tmpl.ParentTemplateID != "" {
		t.Errorf("got %+v, want name=deploy parent=empty", tmpl)
	}
}

func TestCreateTemplate_WithValidParentSucceeds(t *testing.T) {
	repo := newFakeTemplateRepository()
	uc := NewCreateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	parent, err := uc.Execute(ctx, CreateTemplateInput{Name: "company-base", DAGJSON: `{"steps":[]}`, Scope: "company"})
	if err != nil {
		t.Fatalf("creating parent: %v", err)
	}

	child, err := uc.Execute(ctx, CreateTemplateInput{Name: "team-override", DAGJSON: `{"steps":[]}`, Scope: "team", ParentTemplateID: parent.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if child.ParentTemplateID != parent.ID {
		t.Errorf("got parent=%q, want %q", child.ParentTemplateID, parent.ID)
	}
}

func TestCreateTemplate_UnknownParentRejected(t *testing.T) {
	repo := newFakeTemplateRepository()
	uc := NewCreateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, CreateTemplateInput{Name: "orphan", DAGJSON: `{"steps":[]}`, Scope: "personal", ParentTemplateID: "does-not-exist"})
	if err == nil {
		t.Fatal("expected an error creating a template with an unknown parent")
	}
}

func TestCreateTemplate_SelfParentRejected(t *testing.T) {
	// This usecase always assigns a fresh uuid via domain.NewWorkflowTemplate
	// (uuid.NewString()), so a caller can never actually supply "my own id"
	// as ParentTemplateID through CreateTemplateInput — this test exercises
	// the domain invariant directly (domain.NewWorkflowTemplate), confirming
	// it's real and reachable, not that this usecase can trigger it via
	// normal input.
	_, err := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "self", `{"steps":[]}`, domain.ScopePersonal, "tmpl-1", "owner-1")
	if err == nil {
		t.Fatal("expected an error constructing a template that parents itself")
	}
}
