package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// projectWithHost seeds project p1 (tenant-1) owned by u1 and bound to
// devServerID ("" = unbound project).
func projectWithHost(devServerID string) *fakeProjectRepository {
	m := ownerMembership("p1", "u1")
	m.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", DevServerID: devServerID}
	return m
}

func TestAddRepo_HostResolution(t *testing.T) {
	strict := RepoHostPolicy{}
	local := RepoHostPolicy{AllowLocalRepos: true}
	cases := []struct {
		name          string
		projectHost   string
		requestHost   string
		policy        RepoHostPolicy
		hostsExist    bool
		wantHost      string
		wantErrKind   apperrors.Kind
		wantErrCode   string
		wantRepoAdded bool
	}{
		{name: "request host wins over project host", projectHost: "ds-project", requestHost: "ds-request", policy: strict, hostsExist: true, wantHost: "ds-request", wantRepoAdded: true},
		{name: "project host used when request has none", projectHost: "ds-project", policy: strict, hostsExist: true, wantHost: "ds-project", wantRepoAdded: true},
		{name: "no host anywhere is rejected when local repos are not allowed", policy: strict, hostsExist: true, wantErrKind: apperrors.KindInvalidArgument, wantErrCode: CodeRepoHostRequired},
		{name: "no host anywhere is allowed when local repos are allowed", policy: local, hostsExist: true, wantHost: "", wantRepoAdded: true},
		{name: "request host that does not exist is rejected", requestHost: "ds-gone", policy: strict, hostsExist: false, wantErrKind: apperrors.KindInvalidArgument, wantErrCode: "PROJECT_DEV_SERVER_NOT_FOUND"},
		{name: "project host that no longer exists is rejected, not stored", projectHost: "ds-gone", policy: strict, hostsExist: false, wantErrKind: apperrors.KindInvalidArgument, wantErrCode: "PROJECT_DEV_SERVER_NOT_FOUND"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repos := newFakeRepoRepository()
			uc := NewAddRepo(repos, projectWithHost(tc.projectHost), allowAllOPA(), &fakeDevServerLister{exists: tc.hostsExist}, tc.policy)
			ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

			created, err := uc.Execute(ctx, AddRepoInput{ProjectID: "p1", URL: "/Users/x/repo", DevServerID: tc.requestHost})
			if tc.wantErrCode != "" {
				assertAppError(t, err, tc.wantErrKind, tc.wantErrCode)
				if len(repos.repos) != 0 {
					t.Errorf("expected no orphan repo to be stored, got %+v", repos.repos)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if created.DevServerID != tc.wantHost {
				t.Errorf("DevServerID = %q, want %q", created.DevServerID, tc.wantHost)
			}
			if (len(repos.repos) == 1) != tc.wantRepoAdded {
				t.Errorf("repos stored = %d", len(repos.repos))
			}
		})
	}
}

// A project in another tenant is never consulted for its host: tenant
// isolation comes from ProjectRepository.Get being tenant-scoped.
func TestAddRepo_ProjectHostFallbackIsTenantScoped(t *testing.T) {
	projects := ownerMembership("p1", "u1")
	projects.projects["p1"] = domain.Project{ID: "p1", TenantID: "other-tenant", DevServerID: "ds-other"}
	repos := newFakeRepoRepository()
	uc := NewAddRepo(repos, projects, allowAllOPA(), &fakeDevServerLister{exists: true}, RepoHostPolicy{})
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	_, err := uc.Execute(ctx, AddRepoInput{ProjectID: "p1", URL: "/Users/x/repo"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_NOT_FOUND")
	if len(repos.repos) != 0 {
		t.Errorf("expected no repo stored, got %+v", repos.repos)
	}
}

func TestAddRepo_DevServerLookupErrorIsInternalAndStoresNothing(t *testing.T) {
	repos := newFakeRepoRepository()
	uc := NewAddRepo(repos, projectWithHost("ds-project"), allowAllOPA(), &fakeDevServerLister{err: context.DeadlineExceeded}, RepoHostPolicy{})
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	_, err := uc.Execute(ctx, AddRepoInput{ProjectID: "p1", URL: "/Users/x/repo"})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_DEV_SERVER_LOOKUP_FAILED")
	if len(repos.repos) != 0 {
		t.Errorf("expected no repo stored, got %+v", repos.repos)
	}
}
