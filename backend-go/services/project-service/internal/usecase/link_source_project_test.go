package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestLinkSourceProject_AnyMemberOfContainerAndSourceCanLink(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["container"] = domain.Project{ID: "container", TenantID: "tenant-1"}
	repo.projects["source"] = domain.Project{ID: "source", TenantID: "tenant-1"}
	repo.members = append(repo.members,
		domain.ProjectMember{ProjectID: "container", UserID: "u1", Role: domain.ProjectRoleMember},
		domain.ProjectMember{ProjectID: "source", UserID: "u1", Role: domain.ProjectRoleMember},
	)
	sourceProjects := newFakeSourceProjectRepository()
	uc := NewLinkSourceProject(sourceProjects, repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	sp, err := uc.Execute(ctx, LinkSourceProjectInput{ContainerProjectID: "container", SourceProjectID: "source"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sp.ContainerProjectID != "container" || sp.SourceProjectID != "source" || sp.LinkedBy != "u1" {
		t.Errorf("unexpected source project: %+v", sp)
	}
	if len(sourceProjects.links) != 1 {
		t.Errorf("expected 1 link persisted, got %d", len(sourceProjects.links))
	}
}

func TestLinkSourceProject_NonMemberOfContainerDenied(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["container"] = domain.Project{ID: "container", TenantID: "tenant-1"}
	repo.projects["source"] = domain.Project{ID: "source", TenantID: "tenant-1"}
	repo.members = append(repo.members,
		domain.ProjectMember{ProjectID: "source", UserID: "u1", Role: domain.ProjectRoleMember},
	)
	sourceProjects := newFakeSourceProjectRepository()
	uc := NewLinkSourceProject(sourceProjects, repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, LinkSourceProjectInput{ContainerProjectID: "container", SourceProjectID: "source"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if len(sourceProjects.links) != 0 {
		t.Error("expected no link persisted")
	}
}

// TestLinkSourceProject_NonMemberOfSourceDenied is this service's
// equivalent of the legacy TS reference's "ownerUserId must match acting
// user" anti-spoofing check — a member of container can't link a project
// they themselves have no access to.
func TestLinkSourceProject_NonMemberOfSourceDenied(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["container"] = domain.Project{ID: "container", TenantID: "tenant-1"}
	repo.projects["source"] = domain.Project{ID: "source", TenantID: "tenant-1"}
	repo.members = append(repo.members,
		domain.ProjectMember{ProjectID: "container", UserID: "u1", Role: domain.ProjectRoleMember},
		// u1 has no membership row on "source" at all.
	)
	sourceProjects := newFakeSourceProjectRepository()
	uc := NewLinkSourceProject(sourceProjects, repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, LinkSourceProjectInput{ContainerProjectID: "container", SourceProjectID: "source"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if len(sourceProjects.links) != 0 {
		t.Error("expected no link persisted")
	}
}

func TestLinkSourceProject_SelfLinkRejected(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleMember})
	sourceProjects := newFakeSourceProjectRepository()
	uc := NewLinkSourceProject(sourceProjects, repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, LinkSourceProjectInput{ContainerProjectID: "p1", SourceProjectID: "p1"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_SOURCE_PROJECT_INVALID")
}
