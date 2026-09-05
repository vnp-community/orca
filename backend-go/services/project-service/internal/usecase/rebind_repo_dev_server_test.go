package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestRebindRepoDevServer_AllowedWhenNoActiveExecutions(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1", DevServerID: "dev-1"}

	uc := NewRebindRepoDevServer(repos, ownerMembership("p1", "u1"), allowAllOPA(), &fakeExecutionChecker{active: false}, &fakeExecutionChecker{active: false}, &fakeDevServerLister{exists: true})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	got, err := uc.Execute(ctx, RebindRepoDevServerInput{RepoID: "r1", NewDevServerID: "dev-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DevServerID != "dev-2" {
		t.Errorf("expected DevServerID=dev-2, got %q", got.DevServerID)
	}
}

func TestRebindRepoDevServer_RejectedWhenWorkflowHasActiveExecutions(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1", DevServerID: "dev-1"}

	uc := NewRebindRepoDevServer(repos, ownerMembership("p1", "u1"), allowAllOPA(), &fakeExecutionChecker{active: true}, &fakeExecutionChecker{active: false}, &fakeDevServerLister{exists: true})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, RebindRepoDevServerInput{RepoID: "r1", NewDevServerID: "dev-2"})
	assertFailedPrecondition(t, err)

	if repos.repos["r1"].DevServerID != "dev-1" {
		t.Errorf("expected dev_server_id to remain unchanged, got %q", repos.repos["r1"].DevServerID)
	}
}

func TestRebindRepoDevServer_RejectedWhenTaskHasActiveExecutions(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1", DevServerID: "dev-1"}

	uc := NewRebindRepoDevServer(repos, ownerMembership("p1", "u1"), allowAllOPA(), &fakeExecutionChecker{active: false}, &fakeExecutionChecker{active: true}, &fakeDevServerLister{exists: true})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, RebindRepoDevServerInput{RepoID: "r1", NewDevServerID: "dev-2"})
	assertFailedPrecondition(t, err)

	if repos.repos["r1"].DevServerID != "dev-1" {
		t.Errorf("expected dev_server_id to remain unchanged, got %q", repos.repos["r1"].DevServerID)
	}
}

func TestRebindRepoDevServer_FailsClosedOnCheckerError(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1", DevServerID: "dev-1"}

	uc := NewRebindRepoDevServer(repos, ownerMembership("p1", "u1"), allowAllOPA(), &fakeExecutionChecker{err: errors.New("workflow-service unreachable")}, &fakeExecutionChecker{active: false}, &fakeDevServerLister{exists: true})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, RebindRepoDevServerInput{RepoID: "r1", NewDevServerID: "dev-2"})
	assertFailedPrecondition(t, err)
}

func TestRebindRepoDevServer_RepoNotFound(t *testing.T) {
	repos := newFakeRepoRepository()
	uc := NewRebindRepoDevServer(repos, newFakeProjectRepository(), allowAllOPA(), &fakeExecutionChecker{}, &fakeExecutionChecker{}, &fakeDevServerLister{exists: true})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, RebindRepoDevServerInput{RepoID: "missing", NewDevServerID: "dev-2"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND")
}

func TestRebindRepoDevServer_OwnerAllowedMemberDenied(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1", DevServerID: "dev-1"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})

	uc := NewRebindRepoDevServer(repos, membership, &fakeOPAClient{decide: projectRegoDecide}, &fakeExecutionChecker{}, &fakeExecutionChecker{}, &fakeDevServerLister{exists: true})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	_, err := uc.Execute(ctx, RebindRepoDevServerInput{RepoID: "r1", NewDevServerID: "dev-2"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if repos.repos["r1"].DevServerID != "dev-1" {
		t.Errorf("expected dev_server_id to remain unchanged, got %q", repos.repos["r1"].DevServerID)
	}
}

func TestRebindRepoDevServer_GlobalAdminAllowedRegardlessOfProjectRole(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1", DevServerID: "dev-1"}
	opa := &fakeOPAClient{decide: func(callerProjectRole, callerGlobalRole, action string) bool { return true }}

	uc := NewRebindRepoDevServer(repos, newFakeProjectRepository(), opa, &fakeExecutionChecker{}, &fakeExecutionChecker{}, &fakeDevServerLister{exists: true})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "admin-1")
	if _, err := uc.Execute(ctx, RebindRepoDevServerInput{RepoID: "r1", NewDevServerID: "dev-2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRebindRepoDevServer_RejectsUnknownDevServerID(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1", DevServerID: "dev-1"}

	uc := NewRebindRepoDevServer(repos, ownerMembership("p1", "u1"), allowAllOPA(), &fakeExecutionChecker{}, &fakeExecutionChecker{}, &fakeDevServerLister{exists: false})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, RebindRepoDevServerInput{RepoID: "r1", NewDevServerID: "dev-missing"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_DEV_SERVER_NOT_FOUND")
	if repos.repos["r1"].DevServerID != "dev-1" {
		t.Errorf("expected dev_server_id to remain unchanged, got %q", repos.repos["r1"].DevServerID)
	}
}

func TestRebindRepoDevServer_EmptyNewDevServerIDUnbindsToLocal(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1", DevServerID: "dev-1"}
	// A lister that would reject anything — proves an empty NewDevServerID
	// (unbind to local) never calls Exists at all.
	uc := NewRebindRepoDevServer(repos, ownerMembership("p1", "u1"), allowAllOPA(), &fakeExecutionChecker{}, &fakeExecutionChecker{}, &fakeDevServerLister{exists: false})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	got, err := uc.Execute(ctx, RebindRepoDevServerInput{RepoID: "r1", NewDevServerID: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DevServerID != "" {
		t.Errorf("expected empty DevServerID, got %q", got.DevServerID)
	}
}

func TestRebindRepoDevServer_RequiresTenantContext(t *testing.T) {
	repos := newFakeRepoRepository()
	uc := NewRebindRepoDevServer(repos, newFakeProjectRepository(), allowAllOPA(), &fakeExecutionChecker{}, &fakeExecutionChecker{}, &fakeDevServerLister{})

	_, err := uc.Execute(context.Background(), RebindRepoDevServerInput{RepoID: "r1", NewDevServerID: "dev-2"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestRebindRepoDevServer_FailsClosedOnPolicyEvalError(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1", DevServerID: "dev-1"}

	uc := NewRebindRepoDevServer(repos, ownerMembership("p1", "u1"), &fakeOPAClient{err: errors.New("opa unreachable")}, &fakeExecutionChecker{}, &fakeExecutionChecker{}, &fakeDevServerLister{exists: true})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, RebindRepoDevServerInput{RepoID: "r1", NewDevServerID: "dev-2"})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_POLICY_EVAL_FAILED")
	if repos.repos["r1"].DevServerID != "dev-1" {
		t.Errorf("expected dev_server_id to remain unchanged, got %q", repos.repos["r1"].DevServerID)
	}
}
