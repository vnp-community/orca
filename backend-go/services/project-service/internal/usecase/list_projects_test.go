package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestListProjects_ReturnsOnlyCallerMemberProjects(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "mine"}
	repo.projects["p2"] = domain.Project{ID: "p2", TenantID: "tenant-1", Name: "not-mine"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})

	profiles := newFakeProfileResolver()
	uc := NewListProjects(repo, profiles)

	ctx := withRoleTenantAndUser(context.Background(), "tenant-1", "u1", "admin")
	out, err := uc.Execute(ctx, ListProjectsInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Projects) != 1 || out.Projects[0].ID != "p1" {
		t.Errorf("expected only p1 (caller's membership), got %+v", out.Projects)
	}
	if repo.listForMemberCalls != 1 {
		t.Errorf("expected ListForMember called once, got %d", repo.listForMemberCalls)
	}
}

func TestListProjects_AdminRole_SkipsProfileResolver(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "ds-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})

	profiles := newFakeProfileResolver()
	uc := NewListProjects(repo, profiles)

	ctx := withRoleTenantAndUser(context.Background(), "tenant-1", "u1", "admin")
	if _, err := uc.Execute(ctx, ListProjectsInput{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profiles.resolveCalls != 0 {
		t.Errorf("expected admin role to short-circuit before calling GetResolvedProfile, got %d calls", profiles.resolveCalls)
	}
}

func TestListProjects_LeadRole_SkipsProfileResolver(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "ds-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})

	profiles := newFakeProfileResolver()
	uc := NewListProjects(repo, profiles)

	ctx := withRoleTenantAndUser(context.Background(), "tenant-1", "u1", "lead")
	if _, err := uc.Execute(ctx, ListProjectsInput{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profiles.resolveCalls != 0 {
		t.Errorf("expected lead role to short-circuit before calling GetResolvedProfile, got %d calls", profiles.resolveCalls)
	}
}

func TestListProjects_DeveloperRole_CallsProfileResolverAndFiltersByTags(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "gpu-project", DevServerID: "ds-gpu"}
	repo.projects["p2"] = domain.Project{ID: "p2", TenantID: "tenant-1", Name: "cpu-project", DevServerID: "ds-cpu"}
	repo.projects["p3"] = domain.Project{ID: "p3", TenantID: "tenant-1", Name: "unbound-project"} // no DevServerID
	repo.members = append(repo.members,
		domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleMember},
		domain.ProjectMember{ProjectID: "p2", UserID: "u1", Role: domain.ProjectRoleMember},
		domain.ProjectMember{ProjectID: "p3", UserID: "u1", Role: domain.ProjectRoleMember},
	)

	profiles := newFakeProfileResolver()
	profiles.hasRestriction = true
	profiles.allowedTags = []string{"gpu"}
	profiles.devServerTags["ds-gpu"] = []string{"gpu", "eu"}
	profiles.devServerTags["ds-cpu"] = []string{"cpu"}

	uc := NewListProjects(repo, profiles)

	ctx := withRoleTenantAndUser(context.Background(), "tenant-1", "u1", "developer")
	out, err := uc.Execute(ctx, ListProjectsInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profiles.resolveCalls != 1 {
		t.Errorf("expected GetResolvedProfile called exactly once, got %d", profiles.resolveCalls)
	}

	gotIDs := map[string]bool{}
	for _, p := range out.Projects {
		gotIDs[p.ID] = true
	}
	if !gotIDs["p1"] {
		t.Error("expected p1 (matching gpu tag) to pass the filter")
	}
	if gotIDs["p2"] {
		t.Error("expected p2 (no matching tag) to be excluded")
	}
	if !gotIDs["p3"] {
		t.Error("expected p3 (no DevServerID) to always pass through regardless of role")
	}
}

// withRoleTenantAndUser layers tenant.WithRole on top of withTenantAndUser —
// local to this test file since no other list_projects test needs a role
// claim in context.
func withRoleTenantAndUser(ctx context.Context, tenantID, userID, role string) context.Context {
	return withRole(withTenantAndUser(ctx, tenantID, userID), role)
}
