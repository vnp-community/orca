package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestRemoveSparsePreset_OwnerCanRemove(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	presets := newFakeSparsePresetRepository()
	presets.presets["preset-1"] = domain.SparsePreset{ID: "preset-1", RepoID: "r1", Name: "Backend"}
	uc := NewRemoveSparsePreset(presets, repos, ownerMembership("p1", "owner-1"), &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	if err := uc.Execute(ctx, RemoveSparsePresetInput{RepoID: "r1", PresetID: "preset-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := presets.presets["preset-1"]; ok {
		t.Error("expected preset to be removed")
	}
}

func TestRemoveSparsePreset_PlainMemberWithoutGrantDenied(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	presets := newFakeSparsePresetRepository()
	presets.presets["preset-1"] = domain.SparsePreset{ID: "preset-1", RepoID: "r1", Name: "Backend"}
	uc := NewRemoveSparsePreset(presets, repos, membership, &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	err := uc.Execute(ctx, RemoveSparsePresetInput{RepoID: "r1", PresetID: "preset-1"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
}

func TestRemoveSparsePreset_NotFound(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	uc := NewRemoveSparsePreset(newFakeSparsePresetRepository(), repos, ownerMembership("p1", "owner-1"), &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	err := uc.Execute(ctx, RemoveSparsePresetInput{RepoID: "r1", PresetID: "missing"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_SPARSE_PRESET_NOT_FOUND")
}

func TestRemoveSparsePreset_RepoNotFound(t *testing.T) {
	uc := NewRemoveSparsePreset(newFakeSparsePresetRepository(), newFakeRepoRepository(), newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	err := uc.Execute(ctx, RemoveSparsePresetInput{RepoID: "missing", PresetID: "preset-1"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND")
}
