package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func setupChain(t *testing.T, repo *fakeTaskRepository, tenantID string, ids ...string) {
	t.Helper()
	parent := ""
	for _, id := range ids {
		task, err := domain.NewTask(id, tenantID, id, domain.StatusOpen, parent, "")
		if err != nil {
			t.Fatalf("building task %s: %v", id, err)
		}
		repo.tasks[id] = task
		parent = id
	}
}

func TestResolvePermission_RequiresTenantContext(t *testing.T) {
	uc := NewResolvePermission(newFakeTaskRepository(), &fakeGrantRepository{}, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true})
	_, err := uc.Execute(context.Background(), ResolvePermissionInput{TaskID: "t1", UserID: "u1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestResolvePermission_ResolvesAGrantOnTheTaskItself(t *testing.T) {
	tasks := newFakeTaskRepository()
	setupChain(t, tasks, "tenant-1", "root", "child")
	grants := &fakeGrantRepository{grants: []domain.Grant{
		{TaskID: "child", SubjectID: "user-1", Level: domain.GrantLevelOwner, ApplyTree: false},
	}}
	opa := &fakeOPAClient{allow: true}
	uc := NewResolvePermission(tasks, grants, &fakeTeamScopeResolver{}, opa)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	level, err := uc.Execute(ctx, ResolvePermissionInput{TaskID: "child", UserID: "user-1", Action: "admin"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if level != domain.GrantLevelOwner {
		t.Errorf("expected GrantLevelOwner, got %v", level)
	}
	if !opa.called {
		t.Error("expected OPAClient.Decision to be called once a grant was resolved")
	}
}

func TestResolvePermission_ResolvesAnInheritedAncestorGrant(t *testing.T) {
	tasks := newFakeTaskRepository()
	setupChain(t, tasks, "tenant-1", "root", "parent", "child")
	grants := &fakeGrantRepository{grants: []domain.Grant{
		{TaskID: "root", SubjectID: "user-1", Level: domain.GrantLevelAdmin, ApplyTree: true},
	}}
	uc := NewResolvePermission(tasks, grants, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	level, err := uc.Execute(ctx, ResolvePermissionInput{TaskID: "child", UserID: "user-1", Action: "read"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if level != domain.GrantLevelAdmin {
		t.Errorf("expected GrantLevelAdmin from the inherited root grant, got %v", level)
	}
}

func TestResolvePermission_UsesTeamScopeResolverForTeamGrants(t *testing.T) {
	tasks := newFakeTaskRepository()
	setupChain(t, tasks, "tenant-1", "task-1")
	grants := &fakeGrantRepository{grants: []domain.Grant{
		{TaskID: "task-1", SubjectID: "team-a", Level: domain.GrantLevelTeam, ApplyTree: false},
	}}
	teams := &fakeTeamScopeResolver{teams: []string{"team-a"}}
	uc := NewResolvePermission(tasks, grants, teams, &fakeOPAClient{allow: true})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	level, err := uc.Execute(ctx, ResolvePermissionInput{TaskID: "task-1", UserID: "user-1", Action: "read"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if level != domain.GrantLevelTeam {
		t.Errorf("expected GrantLevelTeam, got %v", level)
	}
}

func TestResolvePermission_DeniesWhenNoGrantMatches(t *testing.T) {
	tasks := newFakeTaskRepository()
	setupChain(t, tasks, "tenant-1", "task-1")
	opa := &fakeOPAClient{allow: true}
	uc := NewResolvePermission(tasks, &fakeGrantRepository{}, &fakeTeamScopeResolver{}, opa)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ResolvePermissionInput{TaskID: "task-1", UserID: "user-1", Action: "read"}); err == nil {
		t.Fatal("expected a permission-denied error when no grant matches")
	}
	if opa.called {
		t.Error("OPAClient.Decision must not be called when the BFS walk finds no grant at all")
	}
}

func TestResolvePermission_AllowsWhenOPADecisionIsTrue(t *testing.T) {
	tasks := newFakeTaskRepository()
	setupChain(t, tasks, "tenant-1", "task-1")
	grants := &fakeGrantRepository{grants: []domain.Grant{
		{TaskID: "task-1", SubjectID: "user-1", Level: domain.GrantLevelUser, ApplyTree: false},
	}}
	opa := &fakeOPAClient{allow: true}
	uc := NewResolvePermission(tasks, grants, &fakeTeamScopeResolver{}, opa)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	level, err := uc.Execute(ctx, ResolvePermissionInput{TaskID: "task-1", UserID: "user-1", Action: "write"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if level != domain.GrantLevelUser {
		t.Errorf("expected GrantLevelUser, got %v", level)
	}
}

func TestResolvePermission_DeniesWhenOPADecisionIsFalse(t *testing.T) {
	tasks := newFakeTaskRepository()
	setupChain(t, tasks, "tenant-1", "task-1")
	grants := &fakeGrantRepository{grants: []domain.Grant{
		// Company-level grants match by tenant/company ID (Grant.Matches),
		// so SubjectID must equal the caller's CompanyID (tenantID) here.
		{TaskID: "task-1", SubjectID: "tenant-1", Level: domain.GrantLevelCompany, ApplyTree: false},
	}}
	opa := &fakeOPAClient{allow: false}
	uc := NewResolvePermission(tasks, grants, &fakeTeamScopeResolver{}, opa)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, ResolvePermissionInput{TaskID: "task-1", UserID: "user-1", Action: "write"})
	if err == nil {
		t.Fatal("expected a permission-denied error when OPA denies the requested action")
	}
	if !opa.called {
		t.Error("expected OPAClient.Decision to be called once a grant was resolved")
	}
}

func TestResolvePermission_FailsClosedOnOPAEvaluationError(t *testing.T) {
	tasks := newFakeTaskRepository()
	setupChain(t, tasks, "tenant-1", "task-1")
	grants := &fakeGrantRepository{grants: []domain.Grant{
		{TaskID: "task-1", SubjectID: "user-1", Level: domain.GrantLevelOwner, ApplyTree: false},
	}}
	opa := &fakeOPAClient{decisionErr: errors.New("bundle unavailable")}
	uc := NewResolvePermission(tasks, grants, &fakeTeamScopeResolver{}, opa)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ResolvePermissionInput{TaskID: "task-1", UserID: "user-1", Action: "admin"}); err == nil {
		t.Fatal("expected a permission-denied error when OPA evaluation itself fails")
	}
}
