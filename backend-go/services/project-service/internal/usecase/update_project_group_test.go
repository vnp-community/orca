package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestUpdateProjectGroup_Renames(t *testing.T) {
	repo := newFakeProjectGroupRepository()
	repo.groups["g1"] = domain.ProjectGroup{ID: "g1", TenantID: "tenant-1", Name: "old-name"}
	uc := NewUpdateProjectGroup(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, UpdateProjectGroupInput{GroupID: "g1", Name: "new-name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "new-name" {
		t.Errorf("expected Name=new-name, got %q", got.Name)
	}
}

func TestUpdateProjectGroup_RejectsEmptyName(t *testing.T) {
	repo := newFakeProjectGroupRepository()
	repo.groups["g1"] = domain.ProjectGroup{ID: "g1", TenantID: "tenant-1", Name: "old-name"}
	uc := NewUpdateProjectGroup(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, UpdateProjectGroupInput{GroupID: "g1", Name: ""})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_GROUP_INVALID")
}

func TestUpdateProjectGroup_NotFound(t *testing.T) {
	repo := newFakeProjectGroupRepository()
	uc := NewUpdateProjectGroup(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, UpdateProjectGroupInput{GroupID: "missing", Name: "new-name"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_GROUP_NOT_FOUND")
}
