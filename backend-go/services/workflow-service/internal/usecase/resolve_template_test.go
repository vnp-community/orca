package usecase

import (
	"context"
	"testing"
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
