package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
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

func TestListTemplates_InvalidSortRejected(t *testing.T) {
	repo := newFakeTemplateRepository()
	uc := NewListTemplates(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ListTemplatesInput{Sort: "bogus"})
	if err == nil {
		t.Fatal("expected an error for an invalid sort")
	}
}

func TestListTemplates_QueryFiltersByNameOrDescription(t *testing.T) {
	repo := newFakeTemplateRepository()
	mustNewTemplate(t, repo, "t1", "tenant-1", "deploy pipeline", `{"steps":[]}`, "", domain.WithDescription("ships to prod"))
	mustNewTemplate(t, repo, "t2", "tenant-1", "unrelated", `{"steps":[]}`, "", domain.WithDescription("nothing to do with deploys"))
	mustNewTemplate(t, repo, "t3", "tenant-1", "cleanup job", `{"steps":[]}`, "", domain.WithDescription("housekeeping"))

	uc := NewListTemplates(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	out, err := uc.Execute(ctx, ListTemplatesInput{Query: "deploy"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Templates) != 2 {
		t.Errorf("expected 2 templates matching \"deploy\" (name or description), got %+v", out.Templates)
	}
}

func TestListTemplates_TagsFilterRequiresAllListedTags(t *testing.T) {
	repo := newFakeTemplateRepository()
	mustNewTemplate(t, repo, "t1", "tenant-1", "a", `{"steps":[]}`, "", domain.WithTags([]string{"deploy", "prod"}))
	mustNewTemplate(t, repo, "t2", "tenant-1", "b", `{"steps":[]}`, "", domain.WithTags([]string{"deploy"}))
	mustNewTemplate(t, repo, "t3", "tenant-1", "c", `{"steps":[]}`, "", domain.WithTags([]string{"deploy", "prod", "extra"}))

	uc := NewListTemplates(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	out, err := uc.Execute(ctx, ListTemplatesInput{Tags: []string{"deploy", "prod"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Templates) != 2 {
		t.Errorf("expected exactly the 2 templates carrying BOTH deploy and prod, got %+v", out.Templates)
	}
	for _, tmpl := range out.Templates {
		if tmpl.ID == "t2" {
			t.Errorf("expected t2 (missing 'prod') excluded by the AND-filter, got it in results")
		}
	}
}

func TestListTemplates_SortTrendingOrdersByUsageThenRating(t *testing.T) {
	repo := newFakeTemplateRepository()
	mustNewTemplate(t, repo, "low", "tenant-1", "low", `{"steps":[]}`, "", domain.WithUsageCount(1), domain.WithRating(5, 5))
	mustNewTemplate(t, repo, "high-usage", "tenant-1", "high-usage", `{"steps":[]}`, "", domain.WithUsageCount(10), domain.WithRating(1, 5))
	mustNewTemplate(t, repo, "mid-usage-high-rating", "tenant-1", "mid", `{"steps":[]}`, "", domain.WithUsageCount(5), domain.WithRating(20, 5))

	uc := NewListTemplates(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	out, err := uc.Execute(ctx, ListTemplatesInput{Sort: "trending"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Templates) != 3 {
		t.Fatalf("expected all 3 templates, got %d", len(out.Templates))
	}
	gotOrder := []string{out.Templates[0].ID, out.Templates[1].ID, out.Templates[2].ID}
	wantOrder := []string{"high-usage", "mid-usage-high-rating", "low"}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Errorf("expected trending order %v (usage_count DESC), got %v", wantOrder, gotOrder)
			break
		}
	}
}
