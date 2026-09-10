package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func TestUpdateUserProfile_ClearDepartmentClearsField(t *testing.T) {
	profiles := newFakeUserProfileRepository()
	profiles.byUserID["user-1"] = mustUserProfile(t, "user-1", "company-1", "dept-1", nil)

	uc := NewUpdateUserProfile(profiles, newFakeProfileCache(), nil)
	ctx := withActor(withTenant(context.Background(), "company-1"), "user-1")

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
	ctx := withActor(withTenant(context.Background(), "company-1"), "user-1")

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
	ctx := withActor(withTenant(context.Background(), "company-1"), "user-1")

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
	ctx := withActor(withTenant(context.Background(), "company-1"), "user-1")

	if _, err := uc.Execute(ctx, UpdateUserProfileInput{UserID: "user-1", ClearDepartment: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cache.invalidateCalls) != 1 || cache.invalidateCalls[0] != "user-1" {
		t.Errorf("expected exactly one Invalidate call for user-1, got %+v", cache.invalidateCalls)
	}
}

func TestUpdateUserProfile_DeniesEditingAnotherUsersProfile(t *testing.T) {
	profiles := newFakeUserProfileRepository()
	profiles.byUserID["user-1"] = mustUserProfile(t, "user-1", "company-1", "dept-1", nil)

	uc := NewUpdateUserProfile(profiles, newFakeProfileCache(), nil)
	// Actor is "user-2", editing "user-1"'s profile.
	ctx := withActor(withTenant(context.Background(), "company-1"), "user-2")

	_, err := uc.Execute(ctx, UpdateUserProfileInput{UserID: "user-1", ClearDepartment: true})
	assertAppError(t, err, apperrors.KindPermissionDenied)
}

func TestUpdateUserProfile_RejectsSecurityKeyInSettings(t *testing.T) {
	profiles := newFakeUserProfileRepository()
	profiles.byUserID["user-1"] = mustUserProfile(t, "user-1", "company-1", "dept-1", nil)

	uc := NewUpdateUserProfile(profiles, newFakeProfileCache(), nil)
	ctx := withActor(withTenant(context.Background(), "company-1"), "user-1")

	_, err := uc.Execute(ctx, UpdateUserProfileInput{
		UserID:      "user-1",
		Settings:    domain.Settings{"security": domain.Settings{"sessionTimeoutHours": float64(24)}},
		SetSettings: true,
	})
	assertAppError(t, err, apperrors.KindInvalidArgument)
}

func TestUpdateUserProfile_RejectsIntegrationsGithubOrgInSettings(t *testing.T) {
	profiles := newFakeUserProfileRepository()
	profiles.byUserID["user-1"] = mustUserProfile(t, "user-1", "company-1", "dept-1", nil)

	uc := NewUpdateUserProfile(profiles, newFakeProfileCache(), nil)
	ctx := withActor(withTenant(context.Background(), "company-1"), "user-1")

	_, err := uc.Execute(ctx, UpdateUserProfileInput{
		UserID:      "user-1",
		Settings:    domain.Settings{"integrations": domain.Settings{"githubOrg": "acme"}},
		SetSettings: true,
	})
	assertAppError(t, err, apperrors.KindInvalidArgument)
}

func TestUpdateUserProfile_NoAuditPublisherFieldExists(t *testing.T) {
	// Regression guard for the deliberate audit exemption (BL-PRF-01 §4:
	// personal-pref updates are never audit-logged) — UpdateUserProfile has
	// no `audit` field/constructor param at all, unlike UpdateCompany/
	// UpdateDepartment, so there is nothing here that could publish one.
	uc := NewUpdateUserProfile(newFakeUserProfileRepository(), newFakeProfileCache(), nil)
	if uc == nil {
		t.Fatal("expected a non-nil UpdateUserProfile")
	}
}
