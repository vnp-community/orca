package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestGetSharedProjectData_MemberOfContainerWithLinkedSourceSucceeds(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["container"] = domain.Project{ID: "container", TenantID: "tenant-1"}
	repo.projects["source"] = domain.Project{ID: "source", TenantID: "tenant-1", Name: "Source Project"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "container", UserID: "u1", Role: domain.ProjectRoleMember})

	repoRepo := newFakeRepoRepository()
	repoRepo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "source", URL: "https://example.com/repo.git"}

	worktreeRepo := newFakeWorktreeRepository()
	worktreeRepo.worktrees["w1"] = domain.Worktree{ID: "w1", ProjectID: "source", RepoID: "r1", Path: "/tmp/wt", Branch: "main"}

	sourceProjects := newFakeSourceProjectRepository()
	sourceProjects.links[sourceProjectKey("container", "source")] = domain.SourceProject{ContainerProjectID: "container", SourceProjectID: "source"}

	uc := NewGetSharedProjectData(repo, repoRepo, worktreeRepo, sourceProjects, repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	result, err := uc.Execute(ctx, GetSharedProjectDataInput{ContainerProjectID: "container", SourceProjectID: "source"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Project.ID != "source" || result.Project.Name != "Source Project" {
		t.Errorf("unexpected project: %+v", result.Project)
	}
	if len(result.Repos) != 1 || result.Repos[0].ID != "r1" {
		t.Errorf("unexpected repos: %+v", result.Repos)
	}
	if len(result.Worktrees) != 1 || result.Worktrees[0].ID != "w1" {
		t.Errorf("unexpected worktrees: %+v", result.Worktrees)
	}
}

// TestGetSharedProjectData_EnumerationResistance is this usecase's core
// security invariant (mirrors the legacy TS reference's ACCESS_DENIED_MESSAGE
// doc comment): a non-member of containerProjectID, and a member who passes
// a sourceProjectID that simply isn't linked, must get the IDENTICAL error
// — never a distinguishing message that would let a member probe which
// project ids exist elsewhere in the tenant.
func TestGetSharedProjectData_EnumerationResistance(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["container"] = domain.Project{ID: "container", TenantID: "tenant-1"}
	repo.projects["source"] = domain.Project{ID: "source", TenantID: "tenant-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "container", UserID: "u1", Role: domain.ProjectRoleMember})

	repoRepo := newFakeRepoRepository()
	worktreeRepo := newFakeWorktreeRepository()
	sourceProjects := newFakeSourceProjectRepository()
	// Deliberately no link row for container+source.

	uc := NewGetSharedProjectData(repo, repoRepo, worktreeRepo, sourceProjects, repo, &fakeOPAClient{decide: projectRegoDecide})

	// Case 1: member of container, but source isn't linked.
	ctx1 := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err1 := uc.Execute(ctx1, GetSharedProjectDataInput{ContainerProjectID: "container", SourceProjectID: "source"})
	assertAppError(t, err1, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")

	// Case 2: not a member of container at all.
	ctx2 := withTenantAndUser(context.Background(), "tenant-1", "outsider")
	_, err2 := uc.Execute(ctx2, GetSharedProjectDataInput{ContainerProjectID: "container", SourceProjectID: "source"})
	assertAppError(t, err2, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")

	if err1.Error() != err2.Error() {
		t.Errorf("expected identical errors for enumeration resistance, got %q vs %q", err1.Error(), err2.Error())
	}
}
