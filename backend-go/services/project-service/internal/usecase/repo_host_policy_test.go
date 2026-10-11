package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestRepoHostPolicy_RequireHost(t *testing.T) {
	if err := (RepoHostPolicy{}).requireHost("ds-1"); err != nil {
		t.Fatalf("a bound repo is always valid: %v", err)
	}
	if err := (RepoHostPolicy{AllowLocalRepos: true}).requireHost(""); err != nil {
		t.Fatalf("local repos allowed: %v", err)
	}
	assertAppError(t, (RepoHostPolicy{}).requireHost(""), apperrors.KindInvalidArgument, CodeRepoHostRequired)
}

// CreateProject used to drop repo_path silently when no dev server was given.
func TestCreateProject_RepoPathWithoutHost_RejectedUnlessLocalAllowed(t *testing.T) {
	repo := newFakeProjectRepository()
	repos := newFakeRepoRepository()
	uc := NewCreateProject(repo, repos, &fakeDevServerLister{exists: true}, &fakeDevServerHealthChecker{reachable: true}, &fakeDevServerRelay{}, RepoHostPolicy{})
	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")

	_, _, err := uc.Execute(ctx, CreateProjectInput{Name: "p", RepoPath: "/Users/x/repo"})
	assertAppError(t, err, apperrors.KindInvalidArgument, CodeRepoHostRequired)
	if len(repo.projects) != 0 {
		t.Errorf("expected no project row, got %d", len(repo.projects))
	}

	// A project without a repo path stays creatable without a host.
	if _, _, err := uc.Execute(ctx, CreateProjectInput{Name: "p"}); err != nil {
		t.Fatalf("project without repo_path must not need a host: %v", err)
	}
}

func TestRebindRepoDevServer_UnbindRejectedUnlessLocalAllowed(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1", DevServerID: "dev-1"}
	uc := NewRebindRepoDevServer(repos, ownerMembership("p1", "u1"), allowAllOPA(), &fakeExecutionChecker{}, &fakeExecutionChecker{}, &fakeDevServerLister{exists: true}, RepoHostPolicy{})
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	_, err := uc.Execute(ctx, RebindRepoDevServerInput{RepoID: "r1", NewDevServerID: ""})
	assertAppError(t, err, apperrors.KindInvalidArgument, CodeRepoHostRequired)
	if repos.repos["r1"].DevServerID != "dev-1" {
		t.Errorf("binding must stay unchanged, got %q", repos.repos["r1"].DevServerID)
	}
}

func TestImportNested_WithoutHost_RejectedUnlessLocalAllowed(t *testing.T) {
	groupRepo := newFakeProjectGroupRepository()
	uc := NewImportNested(groupRepo, newFakeRepoRepository(), newFakeProjectRepository(), RepoHostPolicy{})
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	_, _, err := uc.Execute(ctx, ImportNestedInput{Selected: []domain.NestedRepoCandidate{{Path: "/home/dev/repo-a", IsGitRepo: true}}})
	assertAppError(t, err, apperrors.KindInvalidArgument, CodeRepoHostRequired)
}
