package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestListSparsePresets_ProjectOwnerSeesPresets(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	presets := newFakeSparsePresetRepository()
	presets.presets["preset-1"] = domain.SparsePreset{ID: "preset-1", RepoID: "r1", Name: "Backend only", Directories: []string{"src"}}
	uc := NewListSparsePresets(presets, repos, ownerMembership("p1", "owner-1"), &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	got, err := uc.Execute(ctx, ListSparsePresetsInput{RepoID: "r1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Backend only" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestListSparsePresets_PlainMemberWithoutGrantDenied(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewListSparsePresets(newFakeSparsePresetRepository(), repos, membership, &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	_, err := uc.Execute(ctx, ListSparsePresetsInput{RepoID: "r1"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
}

func TestListSparsePresets_MemberWithFunctionalGrantAllowed(t *testing.T) {
	repos := newFakeRepoRepository()
	repos.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	repos.repoMembers = append(repos.repoMembers, domain.RepoMember{RepoID: "r1", UserID: "dev-1", Role: domain.RepoRoleDeveloper})
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "dev-1", Role: domain.ProjectRoleMember})
	uc := NewListSparsePresets(newFakeSparsePresetRepository(), repos, membership, &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "dev-1")
	if _, err := uc.Execute(ctx, ListSparsePresetsInput{RepoID: "r1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListSparsePresets_RepoNotFound(t *testing.T) {
	uc := NewListSparsePresets(newFakeSparsePresetRepository(), newFakeRepoRepository(), newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, ListSparsePresetsInput{RepoID: "missing"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND")
}
