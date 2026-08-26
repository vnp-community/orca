package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func TestUpdateDepartment_AppliesPatch(t *testing.T) {
	departments := newFakeDepartmentRepository()
	_, _ = departments.Create(context.Background(), mustDepartment(t, "dept-1", "company-1", "Old Name", nil))

	uc := NewUpdateDepartment(departments, newFakeUserProfileRepository(), newFakeProfileCache(), nil)
	ctx := withTenant(context.Background(), "company-1")

	got, err := uc.Execute(ctx, UpdateDepartmentInput{
		ID:    "dept-1",
		Patch: domain.DepartmentSettingsPatch{Name: "New Name"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "New Name" {
		t.Errorf("expected patched name %q, got %q", "New Name", got.Name)
	}
}

func TestUpdateDepartment_InvalidatesEveryDepartmentUser(t *testing.T) {
	departments := newFakeDepartmentRepository()
	_, _ = departments.Create(context.Background(), mustDepartment(t, "dept-1", "company-1", "Engineering", nil))
	profiles := newFakeUserProfileRepository()
	profiles.byUserID["user-1"] = mustUserProfile(t, "user-1", "company-1", "dept-1", nil)
	profiles.byUserID["user-2"] = mustUserProfile(t, "user-2", "company-1", "dept-1", nil)
	profiles.byUserID["user-3"] = mustUserProfile(t, "user-3", "company-1", "dept-1", nil)
	cache := newFakeProfileCache()
	cache.byUserID["user-1"] = domain.ResolvedProfile{Settings: domain.Settings{"stale": true}}
	cache.byUserID["user-2"] = domain.ResolvedProfile{Settings: domain.Settings{"stale": true}}
	cache.byUserID["user-3"] = domain.ResolvedProfile{Settings: domain.Settings{"stale": true}}

	uc := NewUpdateDepartment(departments, profiles, cache, nil)
	ctx := withTenant(context.Background(), "company-1")

	if _, err := uc.Execute(ctx, UpdateDepartmentInput{
		ID:    "dept-1",
		Patch: domain.DepartmentSettingsPatch{Name: "New Name"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, uid := range []string{"user-1", "user-2", "user-3"} {
		if _, cached := cache.byUserID[uid]; cached {
			t.Errorf("expected %s's cached profile to be invalidated", uid)
		}
	}
}

func TestUpdateDepartment_NotFound(t *testing.T) {
	departments := newFakeDepartmentRepository()
	profiles := newFakeUserProfileRepository()
	cache := newFakeProfileCache()

	uc := NewUpdateDepartment(departments, profiles, cache, nil)
	ctx := withTenant(context.Background(), "company-1")

	_, err := uc.Execute(ctx, UpdateDepartmentInput{
		ID:    "missing-dept",
		Patch: domain.DepartmentSettingsPatch{Name: "New Name"},
	})
	assertAppError(t, err, apperrors.KindNotFound)

	if profiles.listByDeptCalls != 0 {
		t.Errorf("expected ListUserIDsByDepartment not to be called on a failed write, got %d calls", profiles.listByDeptCalls)
	}
	if len(cache.invalidateCalls) != 0 {
		t.Errorf("expected no cache invalidation on a failed write, got %+v", cache.invalidateCalls)
	}
}

func TestUpdateDepartment_CrossCompanyIsNotFound(t *testing.T) {
	departments := newFakeDepartmentRepository()
	_, _ = departments.Create(context.Background(), mustDepartment(t, "dept-1", "other-company", "Engineering", nil))

	uc := NewUpdateDepartment(departments, newFakeUserProfileRepository(), newFakeProfileCache(), nil)
	ctx := withTenant(context.Background(), "company-1")

	_, err := uc.Execute(ctx, UpdateDepartmentInput{
		ID:    "dept-1",
		Patch: domain.DepartmentSettingsPatch{Name: "New Name"},
	})
	assertAppError(t, err, apperrors.KindNotFound)
}
