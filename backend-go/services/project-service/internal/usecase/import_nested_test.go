package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestImportNested_CreatesOneGroupAndProjectPerCandidate(t *testing.T) {
	groupRepo := newFakeProjectGroupRepository()
	groupRepo.importNestedGroups = []domain.ProjectGroup{
		{ID: "g1", TenantID: "tenant-1", Name: "repo-a", ProjectID: "proj-a"},
		{ID: "g2", TenantID: "tenant-1", Name: "repo-b", ProjectID: "proj-b"},
	}
	groupRepo.importNestedProjects = []domain.Project{
		{ID: "proj-a", TenantID: "tenant-1", Name: "repo-a"},
		{ID: "proj-b", TenantID: "tenant-1", Name: "repo-b"},
	}
	uc := NewImportNested(groupRepo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	selected := []domain.NestedRepoCandidate{
		{Path: "/home/dev/repo-a", SuggestedName: "repo-a", IsGitRepo: true},
		{Path: "/home/dev/repo-b", SuggestedName: "repo-b", IsGitRepo: true},
	}
	groups, projects, err := uc.Execute(ctx, ImportNestedInput{DevServerID: "dev-1", Selected: selected})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d: %+v", len(groups), groups)
	}
	if len(projects) != 2 {
		t.Errorf("expected 2 projects, got %d: %+v", len(projects), projects)
	}
}

func TestImportNested_RejectsNonexistentParentGroup(t *testing.T) {
	groupRepo := newFakeProjectGroupRepository()
	uc := NewImportNested(groupRepo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, _, err := uc.Execute(ctx, ImportNestedInput{ParentGroupID: "missing", Selected: []domain.NestedRepoCandidate{{Path: "/home/dev/repo-a"}}})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_GROUP_NOT_FOUND")
}

func TestImportNested_NoTenantIsUnauthenticated(t *testing.T) {
	groupRepo := newFakeProjectGroupRepository()
	uc := NewImportNested(groupRepo)

	_, _, err := uc.Execute(context.Background(), ImportNestedInput{Selected: []domain.NestedRepoCandidate{{Path: "/home/dev/repo-a"}}})
	assertAppError(t, err, apperrors.KindUnauthenticated, "PROJECT_NO_TENANT")
}
