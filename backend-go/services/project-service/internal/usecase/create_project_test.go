package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestCreateProject_AppliesDefaultsAndCreatedBy(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewCreateProject(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	got, err := uc.Execute(ctx, CreateProjectInput{Name: "my-project"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DefaultBranch != domain.DefaultBranch {
		t.Errorf("expected DefaultBranch=%q, got %q", domain.DefaultBranch, got.DefaultBranch)
	}
	if got.Visibility != domain.DefaultVisibility {
		t.Errorf("expected Visibility=%q, got %q", domain.DefaultVisibility, got.Visibility)
	}
	if got.CreatedBy != "user-1" {
		t.Errorf("expected CreatedBy=user-1, got %q", got.CreatedBy)
	}
}

func TestCreateProject_UsesRequestFieldsWhenProvided(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewCreateProject(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	got, err := uc.Execute(ctx, CreateProjectInput{
		Name:          "my-project",
		Description:   "a description",
		DefaultBranch: "develop",
		Visibility:    domain.VisibilityTeam,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Description != "a description" {
		t.Errorf("expected Description to round-trip, got %q", got.Description)
	}
	if got.DefaultBranch != "develop" {
		t.Errorf("expected DefaultBranch=develop, got %q", got.DefaultBranch)
	}
	if got.Visibility != domain.VisibilityTeam {
		t.Errorf("expected Visibility=team, got %q", got.Visibility)
	}
}

func TestCreateProject_RejectsInvalidVisibility(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewCreateProject(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, CreateProjectInput{Name: "my-project", Visibility: "bogus"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_INVALID_VISIBILITY")
}

func TestCreateProject_RequiresTenantContext(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewCreateProject(repo)

	_, err := uc.Execute(context.Background(), CreateProjectInput{Name: "my-project"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestCreateProject_RequiresUserContext(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewCreateProject(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, CreateProjectInput{Name: "my-project"})
	assertAppError(t, err, apperrors.KindUnauthenticated, "PROJECT_NO_USER")
}
