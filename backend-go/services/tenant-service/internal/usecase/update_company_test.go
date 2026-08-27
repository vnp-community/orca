package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func TestUpdateCompany_AppliesPatch(t *testing.T) {
	companies := newFakeCompanyRepository()
	_, _ = companies.Create(context.Background(), mustCompany(t, "company-1", "Old Name", nil))

	uc := NewUpdateCompany(companies, newFakeUserProfileRepository(), newFakeProfileCache(), nil)

	got, err := uc.Execute(context.Background(), UpdateCompanyInput{
		ID:    "company-1",
		Patch: domain.CompanySettingsPatch{Name: "New Name"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "New Name" {
		t.Errorf("expected patched name %q, got %q", "New Name", got.Name)
	}
}

func TestUpdateCompany_InvalidatesEveryCompanyUser(t *testing.T) {
	companies := newFakeCompanyRepository()
	_, _ = companies.Create(context.Background(), mustCompany(t, "company-1", "Acme", nil))
	profiles := newFakeUserProfileRepository()
	profiles.byUserID["user-1"] = mustUserProfile(t, "user-1", "company-1", "", nil)
	profiles.byUserID["user-2"] = mustUserProfile(t, "user-2", "company-1", "", nil)
	profiles.byUserID["user-3"] = mustUserProfile(t, "user-3", "company-1", "", nil)
	cache := newFakeProfileCache()
	cache.byUserID["user-1"] = domain.ResolvedProfile{Settings: domain.Settings{"stale": true}}
	cache.byUserID["user-2"] = domain.ResolvedProfile{Settings: domain.Settings{"stale": true}}
	cache.byUserID["user-3"] = domain.ResolvedProfile{Settings: domain.Settings{"stale": true}}

	uc := NewUpdateCompany(companies, profiles, cache, nil)

	if _, err := uc.Execute(context.Background(), UpdateCompanyInput{
		ID:    "company-1",
		Patch: domain.CompanySettingsPatch{Name: "New Name"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, uid := range []string{"user-1", "user-2", "user-3"} {
		if _, cached := cache.byUserID[uid]; cached {
			t.Errorf("expected %s's cached profile to be invalidated", uid)
		}
	}
}

func TestUpdateCompany_NotFound(t *testing.T) {
	companies := newFakeCompanyRepository()
	profiles := newFakeUserProfileRepository()
	cache := newFakeProfileCache()

	uc := NewUpdateCompany(companies, profiles, cache, nil)

	_, err := uc.Execute(context.Background(), UpdateCompanyInput{
		ID:    "missing-company",
		Patch: domain.CompanySettingsPatch{Name: "New Name"},
	})
	assertAppError(t, err, apperrors.KindNotFound)

	if profiles.listByCompanyCalls != 0 {
		t.Errorf("expected ListUserIDsByCompany not to be called on a failed write, got %d calls", profiles.listByCompanyCalls)
	}
	if len(cache.invalidateCalls) != 0 {
		t.Errorf("expected no cache invalidation on a failed write, got %+v", cache.invalidateCalls)
	}
}
