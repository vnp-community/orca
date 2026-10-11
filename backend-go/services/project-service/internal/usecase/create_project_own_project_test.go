package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// newOwnProjectCreate wires CreateProject to one world where the dev server
// exists, is online and has the folder.
func newOwnProjectCreate(policy RepoHostPolicy) (*CreateProject, *registrationWorld, *fakeDevServerRelay) {
	w := newRegistrationWorld()
	relay := &fakeDevServerRelay{relayResult: []byte(`{"exists":true,"isDir":true}`)}
	uc := NewCreateProject(w.projects, w.repos, &fakeDevServerLister{exists: true}, &fakeDevServerHealthChecker{reachable: true}, relay, policy)
	return uc, w, relay
}

func TestCreateProject_WritesProjectOwnerAndRepoTogetherAndReturnsThem(t *testing.T) {
	uc, w, _ := newOwnProjectCreate(RepoHostPolicy{})
	ctx := withTenantAndUser(context.Background(), "tenant-1", "creator")

	project, repo, err := uc.Execute(ctx, CreateProjectInput{Name: "Mine", DevServerID: "ds-1", RepoPath: "/srv/mine", Visibility: domain.VisibilityTeam})
	if err != nil {
		t.Fatal(err)
	}
	if w.projects.createWithOwnerCalls != 1 {
		t.Errorf("project, owner and repo must go through one transaction, calls = %d", w.projects.createWithOwnerCalls)
	}
	if repo == nil || repo.ProjectID != project.ID || repo.URL != "/srv/mine" || repo.DevServerID != "ds-1" || repo.ID == "" {
		t.Fatalf("returned repo = %+v for project %q", repo, project.ID)
	}
	if stored := w.repos.repos[repo.ID]; stored.ProjectID != project.ID {
		t.Errorf("repo not stored under the new project: %+v", stored)
	}
	var owner bool
	for _, m := range w.projects.members {
		if m.ProjectID == project.ID && m.UserID == "creator" && m.Role == domain.ProjectRoleOwner {
			owner = true
		}
	}
	if !owner {
		t.Errorf("creator must be the owner, members = %+v", w.projects.members)
	}
	if project.DevServerID != "ds-1" || project.Visibility != domain.VisibilityTeam || project.Name != "Mine" {
		t.Errorf("project = %+v", project)
	}
}

func TestCreateProject_EveryVisibilityIsAccepted(t *testing.T) {
	for _, v := range []string{"", domain.VisibilityPrivate, domain.VisibilityTeam, domain.VisibilityDepartment, domain.VisibilityCompany} {
		uc, _, _ := newOwnProjectCreate(RepoHostPolicy{})
		ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
		project, repo, err := uc.Execute(ctx, CreateProjectInput{Name: "p", DevServerID: "ds-1", RepoPath: "/srv/p", Visibility: v})
		if err != nil || repo == nil {
			t.Fatalf("visibility %q: err=%v repo=%v", v, err, repo)
		}
		want := v
		if want == "" {
			want = domain.DefaultVisibility
		}
		if project.Visibility != want {
			t.Errorf("visibility %q stored as %q, want %q", v, project.Visibility, want)
		}
	}
}

func TestCreateProject_InvalidInputFailsBeforeAnyNetworkCall(t *testing.T) {
	uc, w, relay := newOwnProjectCreate(RepoHostPolicy{})
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	_, _, err := uc.Execute(ctx, CreateProjectInput{Name: "p", DevServerID: "ds-1", RepoPath: "/srv/p", Visibility: "secret"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_INVALID_VISIBILITY")
	_, _, err = uc.Execute(ctx, CreateProjectInput{Name: "", DevServerID: "ds-1", RepoPath: "/srv/p"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_INVALID")
	if len(relay.createConnectionCalls) != 0 {
		t.Errorf("no host round trip for a request that cannot succeed: %+v", relay.createConnectionCalls)
	}
	if w.projects.createWithOwnerCalls != 0 {
		t.Error("nothing may be written")
	}
}

// Project names are not unique; each call is its own project and repo.
func TestCreateProject_SameNameTwiceMakesTwoProjects(t *testing.T) {
	uc, w, _ := newOwnProjectCreate(RepoHostPolicy{})
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	p1, _, err1 := uc.Execute(ctx, CreateProjectInput{Name: "Same", DevServerID: "ds-1", RepoPath: "/srv/a"})
	p2, _, err2 := uc.Execute(ctx, CreateProjectInput{Name: "Same", DevServerID: "ds-1", RepoPath: "/srv/b"})
	if err1 != nil || err2 != nil || p1.ID == p2.ID {
		t.Fatalf("two projects expected: %v %v ids %q %q", err1, err2, p1.ID, p2.ID)
	}
	if len(w.projects.projects) != 5 { // 3 seeded + 2
		t.Errorf("projects = %d", len(w.projects.projects))
	}
}

func TestCreateProject_TrailingSlashOrNotMakesTheSameLocation(t *testing.T) {
	uc, w, _ := newOwnProjectCreate(RepoHostPolicy{})
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	if _, _, err := uc.Execute(ctx, CreateProjectInput{Name: "a", DevServerID: "ds-1", RepoPath: "/srv/x/"}); err != nil {
		t.Fatal(err)
	}
	before := len(w.projects.projects)
	_, _, err := uc.Execute(ctx, CreateProjectInput{Name: "b", DevServerID: "ds-1", RepoPath: "/srv/x"})
	assertAppError(t, err, apperrors.KindAlreadyExists, CodeRepoAlreadyRegistered)
	if len(w.projects.projects) != before {
		t.Error("a rejected create must not leave a project behind")
	}
}

func TestCreateProject_TakenLocationNeedsNoHostRoundTrip(t *testing.T) {
	uc, w, relay := newOwnProjectCreate(RepoHostPolicy{})
	w.seed("r1", "alpha", "ds-1", "/srv/taken", true)
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	_, _, err := uc.Execute(ctx, CreateProjectInput{Name: "p", DevServerID: "ds-1", RepoPath: "/srv/taken"})
	_, msg := wireMessage(t, err)
	want := `PROJECT_REPO_ALREADY_REGISTERED: repo /srv/taken on dev server ds-1 is already registered in project "Alpha" (alpha)`
	if msg != want {
		t.Errorf("message = %q, want %q", msg, want)
	}
	if len(relay.createConnectionCalls) != 0 || w.projects.createWithOwnerCalls != 0 {
		t.Error("no relay call and no write for a taken location")
	}
}

func TestCreateProject_SamePathOnAnotherHostOrTenantIsFine(t *testing.T) {
	uc, w, _ := newOwnProjectCreate(RepoHostPolicy{})
	w.seed("r1", "alpha", "ds-other", "/srv/app", true)
	w.seed("r2", "gamma", "ds-1", "/srv/app", true) // tenant-2
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	if _, _, err := uc.Execute(ctx, CreateProjectInput{Name: "p", DevServerID: "ds-1", RepoPath: "/srv/app"}); err != nil {
		t.Fatalf("different host / tenant must not collide: %v", err)
	}
}

// Losing the write race must not leave the project: the one transaction rolls back.
func TestCreateProject_LostRaceLeavesNoProject(t *testing.T) {
	uc, w, _ := newOwnProjectCreate(RepoHostPolicy{})
	w.seed("r-winner", "alpha", "ds-1", "/srv/app", true)
	w.repos.skipFinds = 1
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	before := len(w.projects.projects)

	_, _, err := uc.Execute(ctx, CreateProjectInput{Name: "p", DevServerID: "ds-1", RepoPath: "/srv/app"})
	assertAppError(t, err, apperrors.KindAlreadyExists, CodeRepoAlreadyRegistered)
	if len(w.projects.projects) != before {
		t.Error("project must not exist after losing the race")
	}
}

func TestCreateProject_RepoThatCannotBeStoredIsAnErrorNotASilentSkip(t *testing.T) {
	uc, w, _ := newOwnProjectCreate(RepoHostPolicy{})
	w.repos.addErr = context.DeadlineExceeded
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	before := len(w.projects.projects)

	_, _, err := uc.Execute(ctx, CreateProjectInput{Name: "p", DevServerID: "ds-1", RepoPath: "/srv/app"})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_CREATE_FAILED")
	if len(w.projects.projects) != before {
		t.Error("a project without its repo must not be created")
	}
}

func TestCreateProject_LocalRepoPathWithoutHostCreatesALocalRepoWhenAllowed(t *testing.T) {
	uc, w, relay := newOwnProjectCreate(RepoHostPolicy{AllowLocalRepos: true})
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	_, repo, err := uc.Execute(ctx, CreateProjectInput{Name: "local", RepoPath: "/data/repos/x"})
	if err != nil || repo == nil || repo.DevServerID != "" {
		t.Fatalf("repo=%+v err=%v", repo, err)
	}
	if len(relay.createConnectionCalls) != 0 {
		t.Error("no host to ask")
	}
	_, _, err = uc.Execute(ctx, CreateProjectInput{Name: "local2", RepoPath: "/data/repos/x/"})
	assertAppError(t, err, apperrors.KindAlreadyExists, CodeRepoAlreadyRegistered)
	_ = w
}

func TestCreateProject_WithoutRepoPathAndHostStillCreatesAnOwnedProject(t *testing.T) {
	uc, w, _ := newOwnProjectCreate(RepoHostPolicy{})
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	project, repo, err := uc.Execute(ctx, CreateProjectInput{Name: "bare"})
	if err != nil || repo != nil {
		t.Fatalf("repo=%v err=%v", repo, err)
	}
	if _, ok := w.projects.projects[project.ID]; !ok {
		t.Error("project not stored")
	}
}
