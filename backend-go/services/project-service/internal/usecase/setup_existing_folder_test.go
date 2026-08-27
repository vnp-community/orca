package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func newHostSetupFixture(status domain.HostSetupStatus) *fakeHostSetupRepository {
	repo := newFakeHostSetupRepository()
	repo.setups["hs1"] = domain.HostSetup{
		ID: "hs1", TenantID: "tenant-1", DevServerID: "dev-1", FolderPath: "/home/dev/repo",
		DisplayName: "My Setup", Status: status, CreatedBy: "u1",
	}
	return repo
}

func TestSetupExistingFolder_PathCheckFailureMarksFailedNoProjectCreated(t *testing.T) {
	hostSetups := newHostSetupFixture(domain.HostSetupPending)
	projects := newFakeProjectRepository()
	repos := newFakeRepoRepository()
	relay := &fakeDevServerRelay{relayResult: []byte(`{"exists":false}`)}
	uc := NewSetupExistingFolder(hostSetups, projects, repos, relay)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, _, err := uc.Execute(ctx, SetupExistingFolderInput{ID: "hs1"})
	assertAppError(t, err, apperrors.KindFailedPrecondition, "PROJECT_FOLDER_NOT_FOUND_ON_HOST")

	if got := hostSetups.setups["hs1"].Status; got != domain.HostSetupFailed {
		t.Errorf("expected Status=%q, got %q", domain.HostSetupFailed, got)
	}
	if len(projects.projects) != 0 {
		t.Errorf("expected no project created, got %+v", projects.projects)
	}
}

func TestSetupExistingFolder_RelayErrorMarksFailed(t *testing.T) {
	hostSetups := newHostSetupFixture(domain.HostSetupPending)
	projects := newFakeProjectRepository()
	repos := newFakeRepoRepository()
	relay := &fakeDevServerRelay{relayErr: errors.New("transport failure")}
	uc := NewSetupExistingFolder(hostSetups, projects, repos, relay)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, _, err := uc.Execute(ctx, SetupExistingFolderInput{ID: "hs1"})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_CHECK_PATH_FAILED")

	if got := hostSetups.setups["hs1"].Status; got != domain.HostSetupFailed {
		t.Errorf("expected Status=%q, got %q", domain.HostSetupFailed, got)
	}
	if len(projects.projects) != 0 {
		t.Errorf("expected no project created, got %+v", projects.projects)
	}
}

func TestSetupExistingFolder_SuccessCreatesExactlyOneProjectAndRepo(t *testing.T) {
	hostSetups := newHostSetupFixture(domain.HostSetupPending)
	projects := newFakeProjectRepository()
	repos := newFakeRepoRepository()
	relay := &fakeDevServerRelay{relayResult: []byte(`{"exists":true,"isDir":true}`)}
	uc := NewSetupExistingFolder(hostSetups, projects, repos, relay)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	setup, project, err := uc.Execute(ctx, SetupExistingFolderInput{ID: "hs1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects.projects) != 1 {
		t.Fatalf("expected exactly 1 project created, got %d: %+v", len(projects.projects), projects.projects)
	}
	if len(repos.repos) != 1 {
		t.Fatalf("expected exactly 1 repo created, got %d: %+v", len(repos.repos), repos.repos)
	}
	if setup.Status != domain.HostSetupCompleted {
		t.Errorf("expected Status=%q, got %q", domain.HostSetupCompleted, setup.Status)
	}
	if setup.ProjectID != project.ID {
		t.Errorf("expected setup.ProjectID=%q to match created project id %q", setup.ProjectID, project.ID)
	}
}

func TestSetupExistingFolder_RejectsAlreadyCompleted(t *testing.T) {
	hostSetups := newHostSetupFixture(domain.HostSetupCompleted)
	projects := newFakeProjectRepository()
	repos := newFakeRepoRepository()
	relay := &fakeDevServerRelay{}
	uc := NewSetupExistingFolder(hostSetups, projects, repos, relay)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, _, err := uc.Execute(ctx, SetupExistingFolderInput{ID: "hs1"})
	assertAppError(t, err, apperrors.KindFailedPrecondition, "PROJECT_HOST_SETUP_ALREADY_COMPLETED")
	if len(relay.createConnectionCalls) != 0 {
		t.Errorf("expected no relay call at all, got %d CreateConnection calls", len(relay.createConnectionCalls))
	}
}

// TestSetupExistingFolder_NeverStatsLocalFilesystem is the regression this
// solution exists to prevent: SetupExistingFolder's only path-existence
// source is the fake DevServerRelay wired in above — no local filesystem
// port exists in its dependency list (repo/projects/repos/relay), so this
// test passes structurally: if the usecase ever grew a local os.Stat/
// os.ReadDir call it would either not compile (no filesystem port to call)
// or the assertions above (relay is the only source of the exists/isDir
// answer) would fail once a real folder-shaped local path diverged from
// the fake relay's canned answer.
func TestSetupExistingFolder_NeverStatsLocalFilesystem(t *testing.T) {
	hostSetups := newHostSetupFixture(domain.HostSetupPending)
	projects := newFakeProjectRepository()
	repos := newFakeRepoRepository()
	// folder_path points at a path this test process cannot possibly have
	// created locally — the ONLY way this call can succeed is via the fake
	// relay's canned {"exists":true,"isDir":true} answer, never a real
	// local stat.
	hostSetups.setups["hs1"] = domain.HostSetup{
		ID: "hs1", TenantID: "tenant-1", DevServerID: "dev-1",
		FolderPath: "/definitely/does/not/exist/on/this/machine", Status: domain.HostSetupPending, CreatedBy: "u1",
	}
	relay := &fakeDevServerRelay{relayResult: []byte(`{"exists":true,"isDir":true}`)}
	uc := NewSetupExistingFolder(hostSetups, projects, repos, relay)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	setup, _, err := uc.Execute(ctx, SetupExistingFolderInput{ID: "hs1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if setup.Status != domain.HostSetupCompleted {
		t.Errorf("expected Status=%q (proving the relay's answer, not a local stat, decided this), got %q", domain.HostSetupCompleted, setup.Status)
	}
}
