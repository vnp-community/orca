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

	uc := NewUpdateCompany(companies, newFakeUserProfileRepository(), newFakeProfileCache(), nil, newFakeOPAClient(true), nil)

	got, err := uc.Execute(adminCtx("company-1"), UpdateCompanyInput{
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

	uc := NewUpdateCompany(companies, profiles, cache, nil, newFakeOPAClient(true), nil)

	if _, err := uc.Execute(adminCtx("company-1"), UpdateCompanyInput{
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

	uc := NewUpdateCompany(companies, profiles, cache, nil, newFakeOPAClient(true), nil)

	_, err := uc.Execute(adminCtx("company-1"), UpdateCompanyInput{
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

func TestUpdateCompany_DeniesNonAdmin(t *testing.T) {
	for _, role := range []string{"lead", "user", ""} {
		t.Run("role="+role, func(t *testing.T) {
			companies := newFakeCompanyRepository()
			_, _ = companies.Create(context.Background(), mustCompany(t, "company-1", "Acme", nil))
			opa := newFakeOPAClient(false)

			uc := NewUpdateCompany(companies, newFakeUserProfileRepository(), newFakeProfileCache(), nil, opa, nil)

			ctx := withRole(withActor(withTenant(context.Background(), "company-1"), "actor-1"), role)
			_, err := uc.Execute(ctx, UpdateCompanyInput{ID: "company-1", Patch: domain.CompanySettingsPatch{Name: "New Name"}})
			assertAppError(t, err, apperrors.KindPermissionDenied)

			if _, ok := companies.byID["company-1"]; !ok {
				t.Fatal("sanity: company should still exist")
			}
			if companies.byID["company-1"].Name != "Acme" {
				t.Errorf("expected company.Update never called on OPA deny, name changed to %q", companies.byID["company-1"].Name)
			}
		})
	}
}

func TestUpdateCompany_InvalidSettingsJSON_ShortCircuitsBeforeUpdate(t *testing.T) {
	companies := newFakeCompanyRepository()
	_, _ = companies.Create(context.Background(), mustCompany(t, "company-1", "Acme", nil))

	uc := NewUpdateCompany(companies, newFakeUserProfileRepository(), newFakeProfileCache(), nil, newFakeOPAClient(true), nil)

	_, err := uc.Execute(adminCtx("company-1"), UpdateCompanyInput{
		ID:    "company-1",
		Patch: domain.CompanySettingsPatch{SettingsJSON: `{"agent":{"approvedModels":["not-a-real-model"]}}`},
	})
	assertAppError(t, err, apperrors.KindInvalidArgument)
	if companies.byID["company-1"].Name != "Acme" {
		t.Error("expected no write to have happened on invalid settings")
	}
}

func TestUpdateCompany_PublishesAuditEventOnSuccess(t *testing.T) {
	companies := newFakeCompanyRepository()
	_, _ = companies.Create(context.Background(), mustCompany(t, "company-1", "Acme", nil))
	audit := newFakeAuditPublisher()

	uc := NewUpdateCompany(companies, newFakeUserProfileRepository(), newFakeProfileCache(), nil, newFakeOPAClient(true), audit)

	ctx := withRole(withActor(withTenant(context.Background(), "company-1"), "actor-9"), "admin")
	if _, err := uc.Execute(ctx, UpdateCompanyInput{ID: "company-1", Patch: domain.CompanySettingsPatch{Name: "New Name"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(audit.calls) != 1 {
		t.Fatalf("expected exactly 1 audit event, got %d", len(audit.calls))
	}
	got := audit.calls[0]
	if got.actorID != "actor-9" || got.action != "company.profile.updated" || got.target != "company-1" {
		t.Errorf("unexpected audit event: %+v", got)
	}
}
