//go:build integration

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// An id naming no row and an id that is not a uuid (SQLSTATE 22P02) must both come back as the domain
// sentinel, so usecases answer 404 instead of a 500.
func TestRepositories_UnknownOrMalformedID_ReturnDomainNotFound(t *testing.T) {
	pool := setupPool(t)
	projects := New(pool)
	repos := NewRepoRepository(pool)
	worktrees := NewWorktreeRepository(pool)
	groups := NewProjectGroupRepository(pool)
	hostSetups := NewHostSetupRepository(pool)
	presets := NewSparsePresetRepository(pool)
	folders := NewFolderWorkspaceRepository(pool)
	ctx := context.Background()
	tenantID := uuid.NewString()
	patch := json.RawMessage(`{"k":"v"}`)

	for _, id := range []string{uuid.NewString(), "not-a-uuid"} {
		checks := []struct {
			name string
			want error
			call func() error
		}{
			{"Project.Get", domain.ErrProjectNotFound, func() error { _, err := projects.Get(ctx, tenantID, id); return err }},
			{"Project.UpdateDevServerID", domain.ErrProjectNotFound, func() error {
				_, err := projects.UpdateDevServerID(ctx, tenantID, id, uuid.NewString())
				return err
			}},
			{"Project.UpdateProject", domain.ErrProjectNotFound, func() error {
				_, err := projects.UpdateProject(ctx, tenantID, id, domain.ProjectUpdatePatch{})
				return err
			}},
			{"Project.DeleteProject", domain.ErrProjectNotFound, func() error { return projects.DeleteProject(ctx, tenantID, id) }},
			{"Project.GetMembership", domain.ErrMembershipNotFound, func() error {
				_, err := projects.GetMembership(ctx, id, uuid.NewString())
				return err
			}},
			{"Project.RemoveMember", domain.ErrMembershipNotFound, func() error { return projects.RemoveMember(ctx, id, uuid.NewString()) }},
			{"Project.UpdateMemberRole", domain.ErrMembershipNotFound, func() error {
				_, err := projects.UpdateMemberRole(ctx, id, uuid.NewString(), domain.ProjectRoleMember)
				return err
			}},
			{"Repo.GetRepo", domain.ErrRepoNotFound, func() error { _, err := repos.GetRepo(ctx, id); return err }},
			{"Repo.Update", domain.ErrRepoNotFound, func() error { _, err := repos.Update(ctx, domain.Repo{ID: id}); return err }},
			{"Repo.UpdateDevServerID", domain.ErrRepoNotFound, func() error {
				_, err := repos.UpdateDevServerID(ctx, id, uuid.NewString(), "")
				return err
			}},
			{"Repo.ReassignProject", domain.ErrRepoNotFound, func() error {
				_, err := repos.ReassignProject(ctx, id, uuid.NewString(), uuid.NewString())
				return err
			}},
			{"Repo.RemoveRepo", domain.ErrRepoNotFound, func() error { return repos.RemoveRepo(ctx, id) }},
			{"Repo.GetRepoMembership", domain.ErrRepoMembershipNotFound, func() error {
				_, err := repos.GetRepoMembership(ctx, id, uuid.NewString())
				return err
			}},
			{"Repo.RemoveRepoMember", domain.ErrRepoMembershipNotFound, func() error { return repos.RemoveRepoMember(ctx, id, uuid.NewString()) }},
			{"Repo.UpdateRepoMemberRole", domain.ErrRepoMembershipNotFound, func() error {
				_, err := repos.UpdateRepoMemberRole(ctx, id, uuid.NewString(), domain.RepoRoleDeveloper)
				return err
			}},
			{"Worktree.GetWorktree", domain.ErrWorktreeNotFound, func() error { _, err := worktrees.GetWorktree(ctx, id); return err }},
			{"Worktree.RecordWorktreeRemoved", domain.ErrWorktreeNotFound, func() error { return worktrees.RecordWorktreeRemoved(ctx, id) }},
			{"Worktree.SetWorktreeActivation", domain.ErrWorktreeNotFound, func() error {
				_, err := worktrees.SetWorktreeActivation(ctx, id, true)
				return err
			}},
			{"Worktree.RenameWorktree", domain.ErrWorktreeNotFound, func() error { _, err := worktrees.RenameWorktree(ctx, id, "b"); return err }},
			{"Worktree.UpdateWorktreeMeta", domain.ErrWorktreeNotFound, func() error {
				_, err := worktrees.UpdateWorktreeMeta(ctx, id, patch)
				return err
			}},
			{"Worktree.SetWorktreeLineage", domain.ErrWorktreeNotFound, func() error {
				_, err := worktrees.SetWorktreeLineage(ctx, id, nil)
				return err
			}},
			{"Group.GetProjectGroup", domain.ErrProjectGroupNotFound, func() error { _, err := groups.GetProjectGroup(ctx, tenantID, id); return err }},
			{"Group.UpdateProjectGroup", domain.ErrProjectGroupNotFound, func() error {
				_, err := groups.UpdateProjectGroup(ctx, tenantID, id, "n")
				return err
			}},
			{"Group.DeleteProjectGroup", domain.ErrProjectGroupNotFound, func() error { return groups.DeleteProjectGroup(ctx, tenantID, id) }},
			{"HostSetup.Get", domain.ErrHostSetupNotFound, func() error { _, err := hostSetups.Get(ctx, tenantID, id); return err }},
			{"HostSetup.Update", domain.ErrHostSetupNotFound, func() error {
				_, err := hostSetups.Update(ctx, tenantID, id, domain.HostSetupPatch{DisplayName: "x"})
				return err
			}},
			{"HostSetup.Delete", domain.ErrHostSetupNotFound, func() error { return hostSetups.Delete(ctx, tenantID, id) }},
			{"SparsePreset.GetSparsePreset", domain.ErrSparsePresetNotFound, func() error {
				_, err := presets.GetSparsePreset(ctx, uuid.NewString(), id)
				return err
			}},
			{"SparsePreset.RemoveSparsePreset", domain.ErrSparsePresetNotFound, func() error {
				return presets.RemoveSparsePreset(ctx, uuid.NewString(), id)
			}},
			{"FolderWorkspace.Update", domain.ErrFolderWorkspaceNotFound, func() error { _, err := folders.Update(ctx, id, "n"); return err }},
			{"FolderWorkspace.Delete", domain.ErrFolderWorkspaceNotFound, func() error { return folders.Delete(ctx, id) }},
		}
		for _, c := range checks {
			if err := c.call(); !errors.Is(err, c.want) {
				t.Errorf("%s(%q): want %v, got %v", c.name, id, c.want, err)
			}
		}
		if fw, err := folders.Get(ctx, id); err != nil || fw != nil {
			t.Errorf("FolderWorkspace.Get(%q): want (nil, nil), got (%v, %v)", id, fw, err)
		}
	}
}
