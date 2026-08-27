package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func TestUpdateUserProfile_ClearDepartmentClearsField(t *testing.T) {
	profiles := newFakeUserProfileRepository()
	profiles.byUserID["user-1"] = mustUserProfile(t, "user-1", "company-1", "dept-1", nil)

	uc := NewUpdateUserProfile(profiles, newFakeProfileCache(), nil)
	ctx := withTenant(context.Background(), "company-1")

	got, err := uc.Execute(ctx, UpdateUserProfileInput{UserID: "user-1", ClearDepartment: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DepartmentID != "" {
		t.Errorf("expected DepartmentID to be cleared, got %q", got.DepartmentID)
	}
}

func TestUpdateUserProfile_EmptyDepartmentIDWithoutClearIsNoChange(t *testing.T) {
	profiles := newFakeUserProfileRepository()
	profiles.byUserID["user-1"] = mustUserProfile(t, "user-1", "company-1", "dept-1", nil)

	uc := NewUpdateUserProfile(profiles, newFakeProfileCache(), nil)
	ctx := withTenant(context.Background(), "company-1")

	got, err := uc.Execute(ctx, UpdateUserProfileInput{UserID: "user-1", DepartmentID: "", ClearDepartment: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DepartmentID != "dept-1" {
		t.Errorf("expected DepartmentID to be preserved as %q, got %q", "dept-1", got.DepartmentID)
	}
}

func TestUpdateUserProfile_SetSettingsFalseIsNoChange(t *testing.T) {
	profiles := newFakeUserProfileRepository()
	profiles.byUserID["user-1"] = mustUserProfile(t, "user-1", "company-1", "dept-1", domain.Settings{"theme": "dark"})

	uc := NewUpdateUserProfile(profiles, newFakeProfileCache(), nil)
	ctx := withTenant(context.Background(), "company-1")

	got, err := uc.Execute(ctx, UpdateUserProfileInput{
		UserID:      "user-1",
		Settings:    domain.Settings{"theme": "light"},
		SetSettings: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Settings["theme"] != "dark" {
		t.Errorf("expected existing Settings to be preserved, got %+v", got.Settings)
	}
}

func TestUpdateUserProfile_InvalidatesOnlyTargetUser(t *testing.T) {
	profiles := newFakeUserProfileRepository()
	profiles.byUserID["user-1"] = mustUserProfile(t, "user-1", "company-1", "dept-1", nil)
	cache := newFakeProfileCache()

	uc := NewUpdateUserProfile(profiles, cache, nil)
	ctx := withTenant(context.Background(), "company-1")

	if _, err := uc.Execute(ctx, UpdateUserProfileInput{UserID: "user-1", ClearDepartment: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cache.invalidateCalls) != 1 || cache.invalidateCalls[0] != "user-1" {
		t.Errorf("expected exactly one Invalidate call for user-1, got %+v", cache.invalidateCalls)
	}
}
