package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestGetProjectContext_MembershipGated_NonMemberDenied(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	uc := NewGetProjectContext(repo, newFakeRepoRepository(), newFakeDevServerHostnameResolver(), &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, "p1")
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
}

func TestGetProjectContext_MemberAllowed(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", Description: "desc", DevServerID: "ds-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleMember})

	repos := newFakeRepoRepository()
	_, _ = repos.AddRepo(context.Background(), domain.Repo{ID: "r1", ProjectID: "p1", URL: "git@example.com:repo.git"})

	hosts := newFakeDevServerHostnameResolver()
	hosts.hostnames["ds-1"] = "dev-1.example.com"

	uc := NewGetProjectContext(repo, repos, hosts, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	got, err := uc.Execute(ctx, "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ProjectID != "p1" || got.ProjectName != "proj" || got.Description != "desc" {
		t.Errorf("unexpected project fields: %+v", got)
	}
	if got.RepoURL != "git@example.com:repo.git" {
		t.Errorf("expected RepoURL from the project's primary repo, got %q", got.RepoURL)
	}
	if got.DevServerHostname != "dev-1.example.com" {
		t.Errorf("expected resolved hostname, got %q", got.DevServerHostname)
	}
}

func TestGetProjectContext_NoRepos_EmptyRepoURL_NotAnError(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})

	uc := NewGetProjectContext(repo, newFakeRepoRepository(), newFakeDevServerHostnameResolver(), &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	got, err := uc.Execute(ctx, "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.RepoURL != "" {
		t.Errorf("expected empty RepoURL for a project with zero repos, got %q", got.RepoURL)
	}
}

func TestGetProjectContext_HostnameResolverError_BestEffort_DoesNotFailTheRead(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "ds-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})

	hosts := newFakeDevServerHostnameResolver()
	hosts.err = errors.New("infra-fleet-service unreachable")

	uc := NewGetProjectContext(repo, newFakeRepoRepository(), hosts, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	got, err := uc.Execute(ctx, "p1")
	if err != nil {
		t.Fatalf("expected hostname resolver failure to be best-effort, got error: %v", err)
	}
	if got.DevServerHostname != "" {
		t.Errorf("expected empty hostname on resolver error, got %q", got.DevServerHostname)
	}
}
