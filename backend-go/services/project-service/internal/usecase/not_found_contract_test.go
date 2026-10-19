package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// Each wrapper fails one lookup with a fixed error, to prove usecases classify only the %w-wrapped
// domain sentinel as NotFound and every other failure as Internal.
type erroringProjectGet struct {
	*fakeProjectRepository
	err error
}

func (r erroringProjectGet) Get(context.Context, string, string) (domain.Project, error) {
	return domain.Project{}, r.err
}

type erroringGroupGet struct {
	*fakeProjectGroupRepository
	err error
}

func (r erroringGroupGet) GetProjectGroup(context.Context, string, string) (domain.ProjectGroup, error) {
	return domain.ProjectGroup{}, r.err
}

type erroringWorktreeGet struct {
	*fakeWorktreeRepository
	err error
}

func (r erroringWorktreeGet) GetWorktree(context.Context, string) (domain.Worktree, error) {
	return domain.Worktree{}, r.err
}

func ownedProjectRepo() *fakeProjectRepository {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "owner-1", Role: domain.ProjectRoleOwner})
	return repo
}

func TestUsecases_RepoNotFoundIsNotFoundAndOtherErrorsAreInternal(t *testing.T) {
	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	opa := &fakeOPAClient{decide: projectRegoDecide}
	boom := errors.New("connection refused")

	cases := []struct {
		name         string
		notFoundErr  error
		notFoundCode string
		internalCode string
		run          func(err error) error
	}{
		{"GetProjectContext", domain.ErrProjectNotFound, "PROJECT_NOT_FOUND", "PROJECT_FETCH_FAILED", func(err error) error {
			uc := NewGetProjectContext(erroringProjectGet{ownedProjectRepo(), err}, newFakeRepoRepository(), newFakeDevServerHostnameResolver(), opa)
			_, e := uc.Execute(ctx, "p1")
			return e
		}},
		{"MoveProject/project", domain.ErrProjectNotFound, "PROJECT_NOT_FOUND", "PROJECT_FETCH_FAILED", func(err error) error {
			uc := NewMoveProject(erroringProjectGet{ownedProjectRepo(), err}, newFakeProjectGroupRepository(), opa)
			_, e := uc.Execute(ctx, "tenant-1", MoveProjectInput{ProjectID: "p1"})
			return e
		}},
		{"MoveProject/target_group", domain.ErrProjectGroupNotFound, "PROJECT_GROUP_NOT_FOUND", "PROJECT_GROUP_FETCH_FAILED", func(err error) error {
			uc := NewMoveProject(ownedProjectRepo(), erroringGroupGet{newFakeProjectGroupRepository(), err}, opa)
			_, e := uc.Execute(ctx, "tenant-1", MoveProjectInput{ProjectID: "p1", TargetParentGroupID: "g1"})
			return e
		}},
		{"ImportNested/parent_group", domain.ErrProjectGroupNotFound, "PROJECT_GROUP_NOT_FOUND", "PROJECT_GROUP_FETCH_FAILED", func(err error) error {
			uc := NewImportNested(erroringGroupGet{newFakeProjectGroupRepository(), err}, newFakeRepoRepository(), newFakeProjectRepository(), RepoHostPolicy{AllowLocalRepos: true})
			_, _, e := uc.Execute(ctx, ImportNestedInput{ParentGroupID: "g1"})
			return e
		}},
		{"GetWorktree", domain.ErrWorktreeNotFound, "PROJECT_WORKTREE_NOT_FOUND", "PROJECT_GET_WORKTREE_FAILED", func(err error) error {
			_, e := NewGetWorktree(erroringWorktreeGet{newFakeWorktreeRepository(), err}).Execute(ctx, "w1")
			return e
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name+"/not_found", func(t *testing.T) {
			assertAppError(t, tc.run(fmt.Errorf("repo: lookup: %w", tc.notFoundErr)), apperrors.KindNotFound, tc.notFoundCode)
		})
		t.Run(tc.name+"/infrastructure_failure", func(t *testing.T) {
			assertAppError(t, tc.run(boom), apperrors.KindInternal, tc.internalCode)
		})
	}
}
