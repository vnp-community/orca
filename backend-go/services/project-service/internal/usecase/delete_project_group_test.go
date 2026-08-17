package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestDeleteProjectGroup_Deletes(t *testing.T) {
	repo := newFakeProjectGroupRepository()
	repo.groups["g1"] = domain.ProjectGroup{ID: "g1", TenantID: "tenant-1", Name: "group-a"}
	uc := NewDeleteProjectGroup(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, DeleteProjectGroupInput{GroupID: "g1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.groups["g1"]; ok {
		t.Error("expected group to be deleted")
	}
}

func TestDeleteProjectGroup_NotFound(t *testing.T) {
	repo := newFakeProjectGroupRepository()
	uc := NewDeleteProjectGroup(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	err := uc.Execute(ctx, DeleteProjectGroupInput{GroupID: "missing"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_GROUP_NOT_FOUND")
}
