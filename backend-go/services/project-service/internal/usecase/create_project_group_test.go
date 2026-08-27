package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestCreateProjectGroup_RootGroup(t *testing.T) {
	repo := newFakeProjectGroupRepository()
	uc := NewCreateProjectGroup(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, CreateProjectGroupInput{Name: "group-a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ParentGroupID != "" {
		t.Errorf("expected a root group, got parent %q", got.ParentGroupID)
	}
}

func TestCreateProjectGroup_ValidatesParentExists(t *testing.T) {
	repo := newFakeProjectGroupRepository()
	repo.groups["parent-1"] = domain.ProjectGroup{ID: "parent-1", TenantID: "tenant-1", Name: "parent"}
	uc := NewCreateProjectGroup(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, CreateProjectGroupInput{Name: "child", ParentGroupID: "parent-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ParentGroupID != "parent-1" {
		t.Errorf("expected ParentGroupID=parent-1, got %q", got.ParentGroupID)
	}
}

func TestCreateProjectGroup_RejectsUnknownParent(t *testing.T) {
	repo := newFakeProjectGroupRepository()
	uc := NewCreateProjectGroup(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, CreateProjectGroupInput{Name: "child", ParentGroupID: "missing"})
	assertAppError(t, err, apperrors.KindFailedPrecondition, "PROJECT_GROUP_PARENT_NOT_FOUND")
}

func TestCreateProjectGroup_RejectsEmptyName(t *testing.T) {
	repo := newFakeProjectGroupRepository()
	uc := NewCreateProjectGroup(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, CreateProjectGroupInput{Name: ""})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_GROUP_INVALID")
}

func TestCreateProjectGroup_RequiresTenantContext(t *testing.T) {
	repo := newFakeProjectGroupRepository()
	uc := NewCreateProjectGroup(repo)

	_, err := uc.Execute(context.Background(), CreateProjectGroupInput{Name: "group-a"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
