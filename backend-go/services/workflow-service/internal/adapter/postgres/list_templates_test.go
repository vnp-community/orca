//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

func mustCreateTemplate(t *testing.T, repo *Repository, tenantID, name string, opts ...domain.TemplateOption) domain.WorkflowTemplate {
	t.Helper()
	tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, name, `{"steps":[]}`, domain.ScopePersonal, "", "owner-1", opts...)
	if err != nil {
		t.Fatalf("building template %s: %v", name, err)
	}
	if err := repo.CreateTemplate(context.Background(), tmpl); err != nil {
		t.Fatalf("creating template %s: %v", name, err)
	}
	// CreateTemplate deliberately doesn't persist usage_count/rating_sum/
	// rating_count (those only ever move via IncrementUsageCount/
	// UpsertRating in the real system — see CreateTemplate's doc comment);
	// tests that need specific seed values for the trending sort write them
	// directly.
	if tmpl.UsageCount != 0 || tmpl.RatingSum != 0 || tmpl.RatingCount != 0 {
		_, err := repo.pool.Exec(context.Background(),
			`UPDATE workflow.templates SET usage_count = $1, rating_sum = $2, rating_count = $3 WHERE id = $4`,
			tmpl.UsageCount, tmpl.RatingSum, tmpl.RatingCount, tmpl.ID)
		if err != nil {
			t.Fatalf("seeding usage/rating for template %s: %v", name, err)
		}
	}
	return tmpl
}

func TestListTemplates_QueryFullTextMatchesNameOrDescription(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	mustCreateTemplate(t, repo, tenantID, "deploy pipeline", domain.WithDescription("ships to prod"))
	mustCreateTemplate(t, repo, tenantID, "unrelated job", domain.WithDescription("nothing to do with releases"))
	mustCreateTemplate(t, repo, tenantID, "cleanup", domain.WithDescription("routine deploy housekeeping"))

	got, _, err := repo.ListTemplates(ctx, tenantID, "", "deploy", nil, "", "", 50)
	if err != nil {
		t.Fatalf("list templates: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 templates matching \"deploy\" (name or description), got %d: %+v", len(got), got)
	}
}

func TestListTemplates_TagsFilterRequiresAllListedTags(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	both := mustCreateTemplate(t, repo, tenantID, "both", domain.WithTags([]string{"deploy", "prod"}))
	mustCreateTemplate(t, repo, tenantID, "partial", domain.WithTags([]string{"deploy"}))
	extra := mustCreateTemplate(t, repo, tenantID, "extra", domain.WithTags([]string{"deploy", "prod", "extra"}))

	got, _, err := repo.ListTemplates(ctx, tenantID, "", "", []string{"deploy", "prod"}, "", "", 50)
	if err != nil {
		t.Fatalf("list templates: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected exactly 2 templates carrying BOTH tags, got %d: %+v", len(got), got)
	}
	ids := map[string]bool{got[0].ID: true, got[1].ID: true}
	if !ids[both.ID] || !ids[extra.ID] {
		t.Errorf("expected [both, extra], got %+v", got)
	}
}

func TestListTemplates_SortTrendingOrdersByUsageThenRating(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	low := mustCreateTemplate(t, repo, tenantID, "low", domain.WithUsageCount(1), domain.WithRating(5, 5))
	high := mustCreateTemplate(t, repo, tenantID, "high", domain.WithUsageCount(10), domain.WithRating(1, 5))
	mid := mustCreateTemplate(t, repo, tenantID, "mid", domain.WithUsageCount(5), domain.WithRating(20, 5))

	got, _, err := repo.ListTemplates(ctx, tenantID, "", "", nil, "trending", "", 50)
	if err != nil {
		t.Fatalf("list templates: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 templates, got %d", len(got))
	}
	wantOrder := []string{high.ID, mid.ID, low.ID}
	for i, want := range wantOrder {
		if got[i].ID != want {
			t.Fatalf("expected trending order [high, mid, low], got %+v", got)
		}
	}
}

func TestListTemplates_SortTrendingKeysetPagination_StableNonOverlappingSecondPage(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	var created []domain.WorkflowTemplate
	for i := 0; i < 5; i++ {
		created = append(created, mustCreateTemplate(t, repo, tenantID, "t", domain.WithUsageCount(int32(5-i)), domain.WithRating(0, 0)))
	}

	firstPage, next, err := repo.ListTemplates(ctx, tenantID, "", "", nil, "trending", "", 3)
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(firstPage) != 3 || next == "" {
		t.Fatalf("expected a full first page of 3 with a next token, got %d rows, next=%q", len(firstPage), next)
	}

	secondPage, next2, err := repo.ListTemplates(ctx, tenantID, "", "", nil, "trending", next, 3)
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(secondPage) != 2 || next2 != "" {
		t.Fatalf("expected exactly the remaining 2 rows and no further page, got %d rows, next=%q", len(secondPage), next2)
	}

	seen := map[string]bool{}
	for _, tmpl := range firstPage {
		seen[tmpl.ID] = true
	}
	for _, tmpl := range secondPage {
		if seen[tmpl.ID] {
			t.Errorf("template %s appeared in both pages — pagination overlapped", tmpl.ID)
		}
	}
	if len(firstPage)+len(secondPage) != len(created) {
		t.Errorf("expected every created template accounted for across both pages, got %d total", len(firstPage)+len(secondPage))
	}
}

func TestListTemplates_SortRecentOrdersByUpdatedAt(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	first := mustCreateTemplate(t, repo, tenantID, "first")
	time.Sleep(10 * time.Millisecond) // ensure a distinct updated_at ordering
	second := mustCreateTemplate(t, repo, tenantID, "second")
	time.Sleep(10 * time.Millisecond)
	// Touch `first` again so it becomes the most-recently-updated row.
	err := repo.WithTx(ctx, func(tx usecase.TemplateRepositoryTx) error {
		return tx.SetVisibility(ctx, first.ID, domain.VisibilityTeam)
	})
	if err != nil {
		t.Fatalf("touching first: %v", err)
	}

	got, _, err := repo.ListTemplates(ctx, tenantID, "", "", nil, "recent", "", 50)
	if err != nil {
		t.Fatalf("list templates: %v", err)
	}
	if len(got) != 2 || got[0].ID != first.ID || got[1].ID != second.ID {
		t.Fatalf("expected [first, second] (most-recently-touched first), got %+v", got)
	}
}

func TestUpsertRating_SecondRatingFromSameUser_UpdatesAggregateNotDoubleCounts(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	tmpl := mustCreateTemplate(t, repo, tenantID, "rated")
	userID := uuid.NewString() // workflow.ratings.user_id is UUID-typed

	var first, second usecase.RateTemplateResult
	err := repo.WithTx(ctx, func(tx usecase.TemplateRepositoryTx) error {
		var terr error
		first, terr = tx.UpsertRating(ctx, tmpl.ID, userID, 3)
		return terr
	})
	if err != nil {
		t.Fatalf("first rating: %v", err)
	}
	if first.RatingSum != 3 || first.RatingCount != 1 {
		t.Fatalf("expected sum=3 count=1 after the first rating, got %+v", first)
	}

	err = repo.WithTx(ctx, func(tx usecase.TemplateRepositoryTx) error {
		var terr error
		second, terr = tx.UpsertRating(ctx, tmpl.ID, userID, 5)
		return terr
	})
	if err != nil {
		t.Fatalf("second rating: %v", err)
	}
	if second.RatingCount != 1 {
		t.Errorf("expected rating_count to stay 1 (update, not duplicate — ratings.(template_id,user_id) UNIQUE), got %d", second.RatingCount)
	}
	if second.RatingSum != 5 {
		t.Errorf("expected rating_sum to reflect the UPDATED value (5), not a stale double-count (3+5=8), got %d", second.RatingSum)
	}
}
