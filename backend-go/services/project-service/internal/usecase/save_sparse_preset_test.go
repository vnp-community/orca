package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestSaveSparsePreset_CreatesNewPreset(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	uc := NewSaveSparsePreset(newFakeSparsePresetRepository(), repos, ownerMembership("p1", "owner-1"), &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	saved, err := uc.Execute(ctx, SaveSparsePresetInput{RepoID: "r1", Name: "  Backend  ", Directories: []string{"src/", "src/api/"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved.Name != "Backend" {
		t.Errorf("expected trimmed name %q, got %q", "Backend", saved.Name)
	}
	if len(saved.Directories) != 2 || saved.Directories[0] != "src" || saved.Directories[1] != "src/api" {
		t.Errorf("unexpected normalized directories: %+v", saved.Directories)
	}
	if saved.ID == "" {
		t.Error("expected a generated id")
	}
	if saved.CreatedAt.IsZero() || saved.UpdatedAt.IsZero() {
		t.Error("expected CreatedAt/UpdatedAt to be set")
	}
}

func TestSaveSparsePreset_UpdatesExistingPresetPreservingCreatedAt(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	presets := newFakeSparsePresetRepository()
	original := domain.SparsePreset{ID: "preset-1", RepoID: "r1", Name: "Old", Directories: []string{"a"}}
	presets.presets["preset-1"] = original
	uc := NewSaveSparsePreset(presets, repos, ownerMembership("p1", "owner-1"), &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	saved, err := uc.Execute(ctx, SaveSparsePresetInput{RepoID: "r1", ID: "preset-1", Name: "New", Directories: []string{"b"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved.ID != "preset-1" {
		t.Errorf("expected id preserved, got %q", saved.ID)
	}
	if saved.CreatedAt != original.CreatedAt {
		t.Error("expected CreatedAt preserved from the existing preset")
	}
	if saved.Name != "New" || len(saved.Directories) != 1 || saved.Directories[0] != "b" {
		t.Errorf("unexpected saved preset: %+v", saved)
	}
}

// TestSaveSparsePreset_UnmatchedIDFallsBackToNewPreset matches the legacy
// TS reference's saveSparsePresetForRepo exactly: a non-empty id that
// doesn't match any existing preset is not an error — it silently creates
// a brand-new preset instead (fresh id, fresh CreatedAt).
func TestSaveSparsePreset_UnmatchedIDFallsBackToNewPreset(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	uc := NewSaveSparsePreset(newFakeSparsePresetRepository(), repos, ownerMembership("p1", "owner-1"), &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	saved, err := uc.Execute(ctx, SaveSparsePresetInput{RepoID: "r1", ID: "never-existed", Name: "Fresh", Directories: []string{"a"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved.ID == "never-existed" {
		t.Error("expected a freshly generated id, not the unmatched input id")
	}
}

func TestSaveSparsePreset_RejectsEmptyName(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	uc := NewSaveSparsePreset(newFakeSparsePresetRepository(), repos, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, SaveSparsePresetInput{RepoID: "r1", Name: "   ", Directories: []string{"a"}})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_SPARSE_PRESET_NAME_REQUIRED")
}

func TestSaveSparsePreset_RejectsAbsoluteDirectory(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	uc := NewSaveSparsePreset(newFakeSparsePresetRepository(), repos, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, SaveSparsePresetInput{RepoID: "r1", Name: "Bad", Directories: []string{"/etc/passwd"}})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_SPARSE_PRESET_DIRECTORY_INVALID")
}

func TestSaveSparsePreset_RejectsTraversalDirectory(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	uc := NewSaveSparsePreset(newFakeSparsePresetRepository(), repos, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, SaveSparsePresetInput{RepoID: "r1", Name: "Bad", Directories: []string{"a/../../etc"}})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_SPARSE_PRESET_DIRECTORY_INVALID")
}

func TestSaveSparsePreset_RejectsNoDirectories(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	uc := NewSaveSparsePreset(newFakeSparsePresetRepository(), repos, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, SaveSparsePresetInput{RepoID: "r1", Name: "Bad", Directories: []string{"  ", "."}})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_SPARSE_PRESET_DIRECTORIES_REQUIRED")
}

func TestSaveSparsePreset_PlainMemberWithoutGrantDenied(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewSaveSparsePreset(newFakeSparsePresetRepository(), repos, membership, &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	_, err := uc.Execute(ctx, SaveSparsePresetInput{RepoID: "r1", Name: "Denied", Directories: []string{"a"}})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
}

func TestSaveSparsePreset_RepoNotFound(t *testing.T) {
	uc := NewSaveSparsePreset(newFakeSparsePresetRepository(), newFakeRepoRepository(), newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, SaveSparsePresetInput{RepoID: "missing", Name: "x", Directories: []string{"a"}})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND")
}
