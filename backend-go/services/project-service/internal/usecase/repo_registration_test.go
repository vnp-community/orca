package usecase

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// registrationWorld is two projects of tenant-1 (alpha: team-visible, beta:
// private) plus one project of another tenant, over one shared repo fake.
type registrationWorld struct {
	projects *fakeProjectRepository
	repos    *fakeRepoRepository
}

func newRegistrationWorld() *registrationWorld {
	projects := newFakeProjectRepository()
	repos := newFakeRepoRepository()
	projects.repoSink, repos.projectDirectory = repos, projects
	projects.projects["alpha"] = domain.Project{ID: "alpha", TenantID: "tenant-1", Name: "Alpha", Visibility: domain.VisibilityTeam}
	projects.projects["beta"] = domain.Project{ID: "beta", TenantID: "tenant-1", Name: "Beta", Visibility: domain.VisibilityPrivate}
	projects.projects["gamma"] = domain.Project{ID: "gamma", TenantID: "tenant-2", Name: "Gamma", Visibility: domain.VisibilityTeam}
	for _, m := range []domain.ProjectMember{
		{ProjectID: "alpha", UserID: "u1", Role: domain.ProjectRoleOwner},
		{ProjectID: "beta", UserID: "u2", Role: domain.ProjectRoleOwner},
		{ProjectID: "gamma", UserID: "u3", Role: domain.ProjectRoleOwner},
	} {
		projects.members = append(projects.members, m)
	}
	return &registrationWorld{projects: projects, repos: repos}
}

// seed stores a repo the way AddRepo would (keyed), or as a legacy row (unkeyed).
func (w *registrationWorld) seed(id, projectID, devServerID, url string, keyed bool) {
	r := domain.Repo{ID: id, ProjectID: projectID, URL: url, DevServerID: devServerID}
	if keyed {
		tenantID := w.projects.projects[projectID].TenantID
		r.RegistrationKey = domain.RepoRegistrationKey(tenantID, devServerID, url)
	}
	w.repos.repos[id] = r
}

func (w *registrationWorld) addRepoUC() *AddRepo {
	return NewAddRepo(w.repos, w.projects, allowAllOPA(), &fakeDevServerLister{exists: true}, RepoHostPolicy{})
}

// wireMessage is what the frontend receives after apperrors.ToGRPCStatus.
func wireMessage(t *testing.T, err error) (codes.Code, string) {
	t.Helper()
	st, ok := status.FromError(apperrors.ToGRPCStatus(err))
	if !ok {
		t.Fatalf("not a status error: %v", err)
	}
	return st.Code(), st.Message()
}

func TestAddRepo_AlreadyRegistered_FixedMessageAndCode(t *testing.T) {
	w := newRegistrationWorld()
	w.seed("r-alpha", "alpha", "ds-1", "/srv/app", true)
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u2")

	// u2 owns beta and adds the same folder there (project-level access is allowed
	// by allowAllOPA); alpha is team-visible so its name and id may be shown.
	_, err := w.addRepoUC().Execute(ctx, AddRepoInput{ProjectID: "beta", URL: "/srv/app", DevServerID: "ds-1"})
	code, msg := wireMessage(t, err)
	if code != codes.AlreadyExists {
		t.Errorf("grpc code = %v, want AlreadyExists", code)
	}
	want := `PROJECT_REPO_ALREADY_REGISTERED: repo /srv/app on dev server ds-1 is already registered in project "Alpha" (alpha)`
	if msg != want {
		t.Errorf("message = %q\nwant      %q", msg, want)
	}
	if len(w.repos.repos) != 1 {
		t.Errorf("a rejected add must store nothing, got %d repos", len(w.repos.repos))
	}
}

func TestAddRepo_AlreadyRegistered_SameProjectToo(t *testing.T) {
	w := newRegistrationWorld()
	w.seed("r1", "alpha", "ds-1", "/srv/app", true)
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	_, err := w.addRepoUC().Execute(ctx, AddRepoInput{ProjectID: "alpha", URL: "/srv/app", DevServerID: "ds-1"})
	assertAppError(t, err, apperrors.KindAlreadyExists, CodeRepoAlreadyRegistered)
}

func TestAddRepo_SpellingVariantsOfTheSamePathAreDuplicates(t *testing.T) {
	for _, tc := range []struct{ existing, requested string }{
		{"/srv/app", "/srv/app/"},
		{"/srv/app/", "/srv//app"},
		{`C:\code\app`, `C:/code/app/`},
		{`C:\code\app\`, `C:\code\app`},
		{"https://git.example/org/app", "https://git.example/org/app/"},
	} {
		w := newRegistrationWorld()
		w.seed("r1", "alpha", "ds-1", tc.existing, false) // legacy row: no key
		ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
		_, err := w.addRepoUC().Execute(ctx, AddRepoInput{ProjectID: "alpha", URL: tc.requested, DevServerID: "ds-1"})
		if err == nil {
			t.Errorf("%q then %q must be a duplicate", tc.existing, tc.requested)
			continue
		}
		assertAppError(t, err, apperrors.KindAlreadyExists, CodeRepoAlreadyRegistered)
	}
}

func TestAddRepo_DifferentLocationsAreNotDuplicates(t *testing.T) {
	cases := []struct {
		name       string
		existing   domain.Repo
		existingIn string
		url, host  string
	}{
		{name: "other dev server, same path", existing: domain.Repo{URL: "/srv/app", DevServerID: "ds-1"}, existingIn: "alpha", url: "/srv/app", host: "ds-2"},
		{name: "same dev server, other path", existing: domain.Repo{URL: "/srv/app", DevServerID: "ds-1"}, existingIn: "alpha", url: "/srv/app2", host: "ds-1"},
		{name: "case differs (paths are case-sensitive)", existing: domain.Repo{URL: "/srv/App", DevServerID: "ds-1"}, existingIn: "alpha", url: "/srv/app", host: "ds-1"},
		{name: "prefix is not a duplicate", existing: domain.Repo{URL: "/srv/app", DevServerID: "ds-1"}, existingIn: "alpha", url: "/srv/app/sub", host: "ds-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newRegistrationWorld()
			tc.existing.ID, tc.existing.ProjectID = "r1", tc.existingIn
			tc.existing.RegistrationKey = domain.RepoRegistrationKey("tenant-1", tc.existing.DevServerID, tc.existing.URL)
			w.repos.repos["r1"] = tc.existing
			ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
			if _, err := w.addRepoUC().Execute(ctx, AddRepoInput{ProjectID: "alpha", URL: tc.url, DevServerID: tc.host}); err != nil {
				t.Fatalf("not a duplicate, got %v", err)
			}
		})
	}
}

// The same dev server id and path in ANOTHER tenant must never block or leak.
func TestAddRepo_OtherTenantSameHostAndPathIsNotADuplicate(t *testing.T) {
	w := newRegistrationWorld()
	w.seed("r-gamma", "gamma", "ds-1", "/srv/app", true)
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	if _, err := w.addRepoUC().Execute(ctx, AddRepoInput{ProjectID: "alpha", URL: "/srv/app", DevServerID: "ds-1"}); err != nil {
		t.Fatalf("another tenant's repo must not count: %v", err)
	}
}

func TestAddRepo_LegacyDuplicateStillBlocksAThirdRegistration(t *testing.T) {
	w := newRegistrationWorld()
	w.seed("r-a", "alpha", "ds-1", "/srv/codex", false)
	w.seed("r-b", "beta", "ds-1", "/srv/codex/", false) // pre-existing duplicate
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	_, err := w.addRepoUC().Execute(ctx, AddRepoInput{ProjectID: "alpha", URL: "/srv/codex", DevServerID: "ds-1"})
	assertAppError(t, err, apperrors.KindAlreadyExists, CodeRepoAlreadyRegistered)
	if len(w.repos.repos) != 2 {
		t.Errorf("legacy duplicates must be left alone, got %d repos", len(w.repos.repos))
	}
}

func TestAddRepo_PrivateProjectOfSomeoneElseIsNotNamed(t *testing.T) {
	w := newRegistrationWorld()
	w.seed("r-beta", "beta", "ds-1", "/srv/app", true) // beta is private, owned by u2
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	_, err := w.addRepoUC().Execute(ctx, AddRepoInput{ProjectID: "alpha", URL: "/srv/app", DevServerID: "ds-1"})
	_, msg := wireMessage(t, err)
	want := "PROJECT_REPO_ALREADY_REGISTERED: repo /srv/app on dev server ds-1 is already registered in another project"
	if msg != want {
		t.Errorf("message = %q, want %q", msg, want)
	}

	// The owner of the private project and a global admin do get the full form.
	for name, c := range map[string]context.Context{
		"member": withTenantAndUser(context.Background(), "tenant-1", "u2"),
		"admin":  tenant.WithRole(withTenantAndUser(context.Background(), "tenant-1", "u1"), "admin"),
	} {
		projectID := "alpha"
		if name == "member" {
			projectID = "beta"
		}
		_, err := w.addRepoUC().Execute(c, AddRepoInput{ProjectID: projectID, URL: "/srv/app", DevServerID: "ds-1"})
		_, msg := wireMessage(t, err)
		wantFull := `PROJECT_REPO_ALREADY_REGISTERED: repo /srv/app on dev server ds-1 is already registered in project "Beta" (beta)`
		if msg != wantFull {
			t.Errorf("%s: message = %q, want %q", name, msg, wantFull)
		}
	}
}

// Two requests pass the pre-check together; the unique key lets one win and the
// loser gets the same, named error.
func TestAddRepo_LostRaceReportsTheWinner(t *testing.T) {
	w := newRegistrationWorld()
	w.seed("r-winner", "alpha", "ds-1", "/srv/app", true)
	w.repos.skipFinds = 1 // the pre-check ran before the winner registered
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	_, err := w.addRepoUC().Execute(ctx, AddRepoInput{ProjectID: "alpha", URL: "/srv/app", DevServerID: "ds-1"})
	_, msg := wireMessage(t, err)
	want := `PROJECT_REPO_ALREADY_REGISTERED: repo /srv/app on dev server ds-1 is already registered in project "Alpha" (alpha)`
	if msg != want {
		t.Errorf("message = %q, want %q", msg, want)
	}
	if len(w.repos.repos) != 1 {
		t.Errorf("loser must store nothing, got %d repos", len(w.repos.repos))
	}
}

func TestAddRepo_LostRaceWhoseWinnerVanishedIsRetryable(t *testing.T) {
	w := newRegistrationWorld()
	w.repos.addErr = domain.ErrRepoAlreadyRegistered
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	_, err := w.addRepoUC().Execute(ctx, AddRepoInput{ProjectID: "alpha", URL: "/srv/app", DevServerID: "ds-1"})
	assertAppError(t, err, apperrors.KindAborted, CodeRepoRegistrationConflict)
}

func TestAddRepo_StoresRegistrationKeyOfNormalisedLocation(t *testing.T) {
	w := newRegistrationWorld()
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	created, err := w.addRepoUC().Execute(ctx, AddRepoInput{ProjectID: "alpha", URL: "/srv/app/", DevServerID: "ds-1"})
	if err != nil {
		t.Fatal(err)
	}
	if created.URL != "/srv/app/" {
		t.Errorf("the caller's spelling must be kept, got %q", created.URL)
	}
	if want := domain.RepoRegistrationKey("tenant-1", "ds-1", "/srv/app"); created.RegistrationKey != want {
		t.Errorf("key = %q, want %q", created.RegistrationKey, want)
	}
}

func TestRebindRepoDevServer_ToAHostThatAlreadyHasThePathIsRejected(t *testing.T) {
	w := newRegistrationWorld()
	w.seed("r-ds1", "alpha", "ds-1", "/srv/app", true)
	w.seed("r-ds2", "beta", "ds-2", "/srv/app/", true)
	uc := NewRebindRepoDevServer(w.repos, w.projects, allowAllOPA(), &fakeExecutionChecker{}, &fakeExecutionChecker{}, &fakeDevServerLister{exists: true}, RepoHostPolicy{})
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	_, err := uc.Execute(ctx, RebindRepoDevServerInput{RepoID: "r-ds1", NewDevServerID: "ds-2"})
	assertAppError(t, err, apperrors.KindAlreadyExists, CodeRepoAlreadyRegistered)
	if w.repos.repos["r-ds1"].DevServerID != "ds-1" {
		t.Errorf("the repo must stay on its host, got %q", w.repos.repos["r-ds1"].DevServerID)
	}
}

func TestRebindRepoDevServer_MovesAndRekeys(t *testing.T) {
	w := newRegistrationWorld()
	w.seed("r1", "alpha", "ds-1", "/srv/app", true)
	uc := NewRebindRepoDevServer(w.repos, w.projects, allowAllOPA(), &fakeExecutionChecker{}, &fakeExecutionChecker{}, &fakeDevServerLister{exists: true}, RepoHostPolicy{})
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	got, err := uc.Execute(ctx, RebindRepoDevServerInput{RepoID: "r1", NewDevServerID: "ds-2"})
	if err != nil {
		t.Fatal(err)
	}
	if want := domain.RepoRegistrationKey("tenant-1", "ds-2", "/srv/app"); got.DevServerID != "ds-2" || got.RegistrationKey != want {
		t.Errorf("got host %q key %q, want ds-2 / %q", got.DevServerID, got.RegistrationKey, want)
	}
	// The old location is free again.
	if _, err := w.addRepoUC().Execute(ctx, AddRepoInput{ProjectID: "alpha", URL: "/srv/app", DevServerID: "ds-1"}); err != nil {
		t.Errorf("old location must be reusable: %v", err)
	}
}

// A legacy duplicate keeps working: "rebinding" it onto the host it already
// sits on changes nothing and must not trip over its twin.
func TestRebindRepoDevServer_SameHostOnLegacyDuplicateIsANoOp(t *testing.T) {
	w := newRegistrationWorld()
	w.seed("r-a", "alpha", "ds-1", "/srv/codex", false)
	w.seed("r-b", "beta", "ds-1", "/srv/codex", false)
	uc := NewRebindRepoDevServer(w.repos, w.projects, allowAllOPA(), &fakeExecutionChecker{}, &fakeExecutionChecker{}, &fakeDevServerLister{exists: true}, RepoHostPolicy{})
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	got, err := uc.Execute(ctx, RebindRepoDevServerInput{RepoID: "r-a", NewDevServerID: "ds-1"})
	if err != nil {
		t.Fatalf("no-op rebind of a legacy duplicate must work: %v", err)
	}
	if got.RegistrationKey != "" {
		t.Errorf("legacy row must stay unkeyed, got %q", got.RegistrationKey)
	}
}

func TestUpdateRepo_ChangingTheUrlToATakenLocationIsRejected(t *testing.T) {
	w := newRegistrationWorld()
	w.seed("r1", "alpha", "ds-1", "/srv/one", true)
	w.seed("r2", "beta", "ds-1", "/srv/two", true)
	uc := NewUpdateRepo(w.repos, w.projects, allowAllOPA())
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	_, err := uc.Execute(ctx, UpdateRepoInput{RepoID: "r1", URL: "/srv/two/"})
	assertAppError(t, err, apperrors.KindAlreadyExists, CodeRepoAlreadyRegistered)
	if w.repos.repos["r1"].URL != "/srv/one" {
		t.Errorf("url must be unchanged, got %q", w.repos.repos["r1"].URL)
	}
}

func TestUpdateRepo_RespellingOrRenamingNeverCountsAsANewRegistration(t *testing.T) {
	w := newRegistrationWorld()
	w.seed("r-a", "alpha", "ds-1", "/srv/codex", false)
	w.seed("r-b", "beta", "ds-1", "/srv/codex", false) // legacy twin
	uc := NewUpdateRepo(w.repos, w.projects, allowAllOPA())
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	if _, err := uc.Execute(ctx, UpdateRepoInput{RepoID: "r-a", DisplayName: "renamed"}); err != nil {
		t.Fatalf("renaming a legacy duplicate must work: %v", err)
	}
	if _, err := uc.Execute(ctx, UpdateRepoInput{RepoID: "r-a", URL: "/srv/codex/"}); err != nil {
		t.Fatalf("respelling the same location must work: %v", err)
	}
}

func TestUpdateRepo_MovingToAFreeLocationRekeys(t *testing.T) {
	w := newRegistrationWorld()
	w.seed("r1", "alpha", "ds-1", "/srv/one", true)
	uc := NewUpdateRepo(w.repos, w.projects, allowAllOPA())
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	got, err := uc.Execute(ctx, UpdateRepoInput{RepoID: "r1", URL: "/srv/three"})
	if err != nil {
		t.Fatal(err)
	}
	if want := domain.RepoRegistrationKey("tenant-1", "ds-1", "/srv/three"); got.RegistrationKey != want {
		t.Errorf("key = %q, want %q", got.RegistrationKey, want)
	}
}

func TestImportNested_RejectsAlreadyRegisteredAndDuplicateSelections(t *testing.T) {
	w := newRegistrationWorld()
	w.seed("r1", "alpha", "ds-1", "/home/dev/repo-a", false)
	groupRepo := newFakeProjectGroupRepository()
	uc := NewImportNested(groupRepo, w.repos, w.projects, RepoHostPolicy{})
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	_, _, err := uc.Execute(ctx, ImportNestedInput{DevServerID: "ds-1", Selected: []domain.NestedRepoCandidate{
		{Path: "/home/dev/repo-b"}, {Path: "/home/dev/repo-a/"},
	}})
	assertAppError(t, err, apperrors.KindAlreadyExists, CodeRepoAlreadyRegistered)

	_, _, err = uc.Execute(ctx, ImportNestedInput{DevServerID: "ds-1", Selected: []domain.NestedRepoCandidate{
		{Path: "/home/dev/repo-b"}, {Path: "/home/dev/repo-b/"},
	}})
	assertAppError(t, err, apperrors.KindInvalidArgument, CodeImportNestedDuplicatePath)

	// Another host does not conflict.
	if _, _, err := uc.Execute(ctx, ImportNestedInput{DevServerID: "ds-2", Selected: []domain.NestedRepoCandidate{{Path: "/home/dev/repo-a"}}}); err != nil {
		t.Fatalf("other host must not conflict: %v", err)
	}
}

func TestImportNested_LostRaceIsReportedNotInternal(t *testing.T) {
	w := newRegistrationWorld()
	groupRepo := newFakeProjectGroupRepository()
	groupRepo.importNestedErr = domain.ErrRepoAlreadyRegistered
	uc := NewImportNested(groupRepo, w.repos, w.projects, RepoHostPolicy{})
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	_, _, err := uc.Execute(ctx, ImportNestedInput{DevServerID: "ds-1", Selected: []domain.NestedRepoCandidate{{Path: "/home/dev/repo-a"}}})
	assertAppError(t, err, apperrors.KindAborted, CodeRepoRegistrationConflict)
}

func TestSetupExistingFolder_TakenLocationLeavesTheSetupPendingAndCreatesNothing(t *testing.T) {
	w := newRegistrationWorld()
	hostSetups := newHostSetupFixture(domain.HostSetupPending)
	setup := hostSetups.setups["hs1"]
	w.seed("r1", "alpha", setup.DevServerID, setup.FolderPath, false)
	relay := &fakeDevServerRelay{relayResult: []byte(`{"exists":true,"isDir":true}`)}
	uc := NewSetupExistingFolder(hostSetups, w.projects, w.repos, relay)
	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

	_, _, err := uc.Execute(ctx, SetupExistingFolderInput{ID: "hs1"})
	assertAppError(t, err, apperrors.KindAlreadyExists, CodeRepoAlreadyRegistered)
	if got := hostSetups.setups["hs1"].Status; got != domain.HostSetupPending {
		t.Errorf("a taken location is not the setup's fault: status = %q", got)
	}
	if w.projects.createWithOwnerCalls != 0 {
		t.Error("no project may be created")
	}
}
