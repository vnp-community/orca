package usecase

import (
	"context"
	"testing"
)

func TestListTemplates_FiltersByTenant(t *testing.T) {
	repo := newFakeTemplateRepository()
	createUC := NewCreateTemplate(repo)
	ctx1 := withTenantContext(context.Background(), "tenant-1")
	ctx2 := withTenantContext(context.Background(), "tenant-2")

	if _, err := createUC.Execute(ctx1, CreateTemplateInput{Name: "a", DAGJSON: `{"steps":[]}`, Scope: "personal"}); err != nil {
		t.Fatalf("creating tenant-1 template: %v", err)
	}
	if _, err := createUC.Execute(ctx2, CreateTemplateInput{Name: "b", DAGJSON: `{"steps":[]}`, Scope: "personal"}); err != nil {
		t.Fatalf("creating tenant-2 template: %v", err)
	}

	uc := NewListTemplates(repo)
	out, err := uc.Execute(ctx1, ListTemplatesInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Templates) != 1 || out.Templates[0].Name != "a" {
		t.Errorf("got %+v, want exactly tenant-1's template", out.Templates)
	}
}

func TestListTemplates_FiltersByScope(t *testing.T) {
	repo := newFakeTemplateRepository()
	createUC := NewCreateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, _ = createUC.Execute(ctx, CreateTemplateInput{Name: "company-a", DAGJSON: `{"steps":[]}`, Scope: "company"})
	_, _ = createUC.Execute(ctx, CreateTemplateInput{Name: "personal-a", DAGJSON: `{"steps":[]}`, Scope: "personal"})

	uc := NewListTemplates(repo)
	out, err := uc.Execute(ctx, ListTemplatesInput{Scope: "company"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Templates) != 1 || out.Templates[0].Name != "company-a" {
		t.Errorf("got %+v, want exactly the company-scoped template", out.Templates)
	}
}

func TestListTemplates_InvalidScopeRejected(t *testing.T) {
	repo := newFakeTemplateRepository()
	uc := NewListTemplates(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ListTemplatesInput{Scope: "bogus"})
	if err == nil {
		t.Fatal("expected an error for an invalid scope filter")
	}
}

func TestListTemplates_PaginationCursorAdvances(t *testing.T) {
	repo := newFakeTemplateRepository()
	createUC := NewCreateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	for i := 0; i < 3; i++ {
		if _, err := createUC.Execute(ctx, CreateTemplateInput{Name: "t", DAGJSON: `{"steps":[]}`, Scope: "personal"}); err != nil {
			t.Fatalf("creating template %d: %v", i, err)
		}
	}

	uc := NewListTemplates(repo)
	firstPage, err := uc.Execute(ctx, ListTemplatesInput{PageSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(firstPage.Templates) != 2 || firstPage.NextPageToken == "" {
		t.Fatalf("expected a full first page with a next token, got %+v", firstPage)
	}

	secondPage, err := uc.Execute(ctx, ListTemplatesInput{PageSize: 2, PageToken: firstPage.NextPageToken})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(secondPage.Templates) != 1 || secondPage.NextPageToken != "" {
		t.Errorf("expected exactly one remaining template and no further page, got %+v", secondPage)
	}
}
