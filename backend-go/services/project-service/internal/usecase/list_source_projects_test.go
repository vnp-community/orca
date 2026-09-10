package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestListSourceProjects_AnyMemberCanList(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["container"] = domain.Project{ID: "container", TenantID: "tenant-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "container", UserID: "u1", Role: domain.ProjectRoleMember})
	sourceProjects := newFakeSourceProjectRepository()
	sourceProjects.links[sourceProjectKey("container", "s1")] = domain.SourceProject{ContainerProjectID: "container", SourceProjectID: "s1"}
	sourceProjects.links[sourceProjectKey("container", "s2")] = domain.SourceProject{ContainerProjectID: "container", SourceProjectID: "s2"}
	sourceProjects.links[sourceProjectKey("other", "s3")] = domain.SourceProject{ContainerProjectID: "other", SourceProjectID: "s3"}
	uc := NewListSourceProjects(sourceProjects, repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	list, err := uc.Execute(ctx, ListSourceProjectsInput{ContainerProjectID: "container"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 source projects (scoped to container), got %d: %+v", len(list), list)
	}
}

func TestListSourceProjects_NonMemberDenied(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["container"] = domain.Project{ID: "container", TenantID: "tenant-1"}
	sourceProjects := newFakeSourceProjectRepository()
	uc := NewListSourceProjects(sourceProjects, repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "outsider")
	_, err := uc.Execute(ctx, ListSourceProjectsInput{ContainerProjectID: "container"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
}
