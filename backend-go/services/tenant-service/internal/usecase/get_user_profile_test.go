package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func TestGetUserProfile_Found(t *testing.T) {
	profiles := newFakeUserProfileRepository()
	profiles.byUserID["user-1"] = mustUserProfile(t, "user-1", "company-1", "dept-1", domain.Settings{"theme": "dark"})

	uc := NewGetUserProfile(profiles)
	ctx := withTenant(context.Background(), "company-1")

	got, err := uc.Execute(ctx, GetUserProfileInput{UserID: "user-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UserID != "user-1" || got.CompanyID != "company-1" || got.DepartmentID != "dept-1" {
		t.Errorf("unexpected profile: %+v", got)
	}
}

func TestGetUserProfile_NotFound(t *testing.T) {
	profiles := newFakeUserProfileRepository()

	uc := NewGetUserProfile(profiles)
	ctx := withTenant(context.Background(), "company-1")

	_, err := uc.Execute(ctx, GetUserProfileInput{UserID: "missing-user"})
	assertAppError(t, err, apperrors.KindNotFound)
}
