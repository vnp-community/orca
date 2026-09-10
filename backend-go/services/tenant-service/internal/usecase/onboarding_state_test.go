package usecase

import (
	"context"
	"errors"
	"testing"
)

func TestGetOnboardingState_RequiresTenantContext(t *testing.T) {
	uc := NewGetOnboardingState(newFakeUserProfileRepository())

	_, err := uc.Execute(context.Background(), "user-1")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestGetOnboardingState_NotFoundMeansWizardNotStarted(t *testing.T) {
	uc := NewGetOnboardingState(newFakeUserProfileRepository())
	ctx := withTenant(context.Background(), "company-1")

	got, err := uc.Execute(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Found {
		t.Error("want Found=false for a user who never saved onboarding state")
	}
}

func TestSetOnboardingState_RequiresTenantContext(t *testing.T) {
	uc := NewSetOnboardingState(newFakeUserProfileRepository())

	err := uc.Execute(context.Background(), SetOnboardingStateInput{UserID: "user-1", StateJSON: "{}"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestSetOnboardingState_ThenGet_RoundTrips(t *testing.T) {
	repo := newFakeUserProfileRepository()
	setUC := NewSetOnboardingState(repo)
	getUC := NewGetOnboardingState(repo)
	ctx := withTenant(context.Background(), "company-1")

	err := setUC.Execute(ctx, SetOnboardingStateInput{
		UserID: "user-1", StateJSON: `{"lastCompletedStep":2}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := getUC.Execute(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Found {
		t.Fatal("want Found=true after saving")
	}
	if got.StateJSON != `{"lastCompletedStep":2}` {
		t.Errorf("want the saved state round-tripped, got %q", got.StateJSON)
	}
}

func TestGetOnboardingState_RepositoryErrorPropagates(t *testing.T) {
	repo := newFakeUserProfileRepository()
	repo.getOnboardingStateErr = errors.New("boom")
	uc := NewGetOnboardingState(repo)
	ctx := withTenant(context.Background(), "company-1")

	_, err := uc.Execute(ctx, "user-1")
	if err == nil {
		t.Fatal("expected an error when the repository fails")
	}
}

func TestSetOnboardingState_RepositoryErrorPropagates(t *testing.T) {
	repo := newFakeUserProfileRepository()
	repo.setOnboardingStateErr = errors.New("boom")
	uc := NewSetOnboardingState(repo)
	ctx := withTenant(context.Background(), "company-1")

	err := uc.Execute(ctx, SetOnboardingStateInput{UserID: "user-1", StateJSON: "{}"})
	if err == nil {
		t.Fatal("expected an error when the repository fails")
	}
}
