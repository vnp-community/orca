package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// tenantContextForUser is withTenantContext's parameterized-user sibling —
// needed here to exercise "two DIFFERENT users rating the same template,"
// which withTenantContext's fixed "user-1" can't express.
func tenantContextForUser(tenantID, userID string) context.Context {
	return tenant.WithUserID(tenant.WithTenantID(context.Background(), tenantID), userID)
}

func TestRateTemplate_RequiresValidStarsRange(t *testing.T) {
	repo := newFakeTemplateRepository()
	tmpl := mustNewTemplate(t, repo, "tmpl-1", "tenant-1", "deploy", `{"steps":[]}`, "")

	uc := NewRateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	for _, stars := range []int32{0, 6, -1} {
		_, err := uc.Execute(ctx, tmpl.ID, stars)
		if err == nil {
			t.Errorf("expected an error for stars=%d", stars)
			continue
		}
		var ae *apperrors.AppError
		if !errors.As(err, &ae) || ae.Code != "WORKFLOW_INVALID_RATING" {
			t.Errorf("stars=%d: expected WORKFLOW_INVALID_RATING, got %v", stars, err)
		}
	}
}

func TestRateTemplate_FirstRating_SetsAggregate(t *testing.T) {
	repo := newFakeTemplateRepository()
	tmpl := mustNewTemplate(t, repo, "tmpl-1", "tenant-1", "deploy", `{"steps":[]}`, "")

	uc := NewRateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	result, err := uc.Execute(ctx, tmpl.ID, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RatingSum != 4 || result.RatingCount != 1 {
		t.Errorf("expected sum=4 count=1 after the first rating, got %+v", result)
	}
}

// TestRateTemplate_SecondRatingFromSameUser_UpdatesNotDuplicates is the
// task's core assertion: a second rating from the SAME user updates their
// prior rating (rating_count stays 1) rather than double-counting it.
func TestRateTemplate_SecondRatingFromSameUser_UpdatesNotDuplicates(t *testing.T) {
	repo := newFakeTemplateRepository()
	tmpl := mustNewTemplate(t, repo, "tmpl-1", "tenant-1", "deploy", `{"steps":[]}`, "")

	uc := NewRateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1") // same acting user (user-1) both calls

	if _, err := uc.Execute(ctx, tmpl.ID, 3); err != nil {
		t.Fatalf("first rating: %v", err)
	}
	result, err := uc.Execute(ctx, tmpl.ID, 5)
	if err != nil {
		t.Fatalf("second rating: %v", err)
	}
	if result.RatingCount != 1 {
		t.Errorf("expected rating_count to stay 1 (update, not duplicate), got %d", result.RatingCount)
	}
	if result.RatingSum != 5 {
		t.Errorf("expected rating_sum to reflect the UPDATED value (5, not 3+5=8, a stale double-count), got %d", result.RatingSum)
	}
}

func TestRateTemplate_MultipleUsers_AggregateSumsAll(t *testing.T) {
	repo := newFakeTemplateRepository()
	tmpl := mustNewTemplate(t, repo, "tmpl-1", "tenant-1", "deploy", `{"steps":[]}`, "")

	uc := NewRateTemplate(repo)
	ctx1 := tenantContextForUser("tenant-1", "user-a")
	ctx2 := tenantContextForUser("tenant-1", "user-b")

	if _, err := uc.Execute(ctx1, tmpl.ID, 5); err != nil {
		t.Fatalf("user-a rating: %v", err)
	}
	result, err := uc.Execute(ctx2, tmpl.ID, 3)
	if err != nil {
		t.Fatalf("user-b rating: %v", err)
	}
	if result.RatingSum != 8 || result.RatingCount != 2 {
		t.Errorf("expected sum=8 count=2 across two distinct users, got %+v", result)
	}
}

func TestRateTemplate_UnknownTemplate_NotFound(t *testing.T) {
	repo := newFakeTemplateRepository()
	uc := NewRateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, "does-not-exist", 3)
	if err == nil {
		t.Fatal("expected a not-found error")
	}
}
