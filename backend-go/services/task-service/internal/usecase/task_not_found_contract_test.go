package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// erroringTaskRepository fails every lookup/write a caller-supplied id can reach with a fixed error,
// to prove usecases classify only the domain sentinel as NotFound.
type erroringTaskRepository struct {
	*fakeTaskRepository
	err error
}

func (r erroringTaskRepository) Get(context.Context, string, string) (domain.Task, error) {
	return domain.Task{}, r.err
}

func (r erroringTaskRepository) GetAncestors(context.Context, string, string, int) ([]domain.Task, error) {
	return nil, r.err
}

func (r erroringTaskRepository) FindByNumber(context.Context, string, string, int64) (domain.Task, error) {
	return domain.Task{}, r.err
}

func (r erroringTaskRepository) GetByShareToken(context.Context, string) (domain.Task, error) {
	return domain.Task{}, r.err
}

func (r erroringTaskRepository) GetSubtree(context.Context, string, string, int) ([]domain.Task, []domain.TaskEdge, error) {
	return nil, nil, r.err
}

func (r erroringTaskRepository) GetSubtreeWithChildPercents(context.Context, string, string) ([]SubtreeProgressNode, error) {
	return nil, r.err
}

func (r erroringTaskRepository) Update(context.Context, string, domain.Task, []domain.OutboxEvent) error {
	return r.err
}

type erroringShareLinkRepository struct {
	*fakeShareLinkRepository
	err error
}

func (r erroringShareLinkRepository) ResolveActive(context.Context, string, string) (string, error) {
	return "", r.err
}

func (r erroringShareLinkRepository) TaskIDFor(context.Context, string, string) (string, error) {
	return "", r.err
}

func (r erroringShareLinkRepository) Revoke(context.Context, string, string) error { return r.err }

func assertKindAndCode(t *testing.T, err error, kind apperrors.Kind, code string) {
	t.Helper()
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected an AppError, got %v", err)
	}
	if ae.Kind != kind {
		t.Fatalf("kind = %v, want %v (err: %v)", ae.Kind, kind, err)
	}
	if code != "" && ae.Code != code {
		t.Fatalf("code = %q, want %q", ae.Code, code)
	}
}

// TestTaskUsecases_RepoNotFoundIsNotFoundAndOtherErrorsAreInternal pins the not-found convention:
// ErrTaskNotFound (wrapped with %w) -> NotFound, any other repo failure -> Internal, never 404.
func TestTaskUsecases_RepoNotFoundIsNotFoundAndOtherErrorsAreInternal(t *testing.T) {
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	notFound := fmt.Errorf("repo: task x: %w", domain.ErrTaskNotFound)
	boom := errors.New("connection refused")

	cases := []struct {
		name         string
		notFoundCode string
		internalCode string
		run          func(repo TaskRepository) error
	}{
		{"GetTask", "TASK_NOT_FOUND", "TASK_FETCH_FAILED", func(repo TaskRepository) error {
			_, err := NewGetTask(repo).Execute(ctx, "t1")
			return err
		}},
		{"UpdateTask", "TASK_NOT_FOUND", "TASK_FETCH_FAILED", func(repo TaskRepository) error {
			_, err := NewUpdateTask(repo, &fakeEdgeRepository{}).Execute(ctx, UpdateTaskInput{ID: "t1"})
			return err
		}},
		{"FindTaskByNumber", "TASK_NOT_FOUND_BY_NUMBER", "TASK_FIND_BY_NUMBER_FAILED", func(repo TaskRepository) error {
			_, err := NewFindTaskByNumber(repo).Execute(ctx, FindTaskByNumberInput{ProjectID: "p1", TaskNumber: 1})
			return err
		}},
		{"GetTaskByShareToken", "TASK_NOT_FOUND", "TASK_SHARE_TOKEN_LOOKUP_FAILED", func(repo TaskRepository) error {
			_, err := NewGetTaskByShareToken(repo).Execute(ctx, "token")
			return err
		}},
		{"GetSubtree", "TASK_NOT_FOUND", "TASK_SUBTREE_FAILED", func(repo TaskRepository) error {
			_, err := NewGetSubtree(repo, &fakeGrantRepository{}, &fakeTeamScopeResolver{}).Execute(ctx, GetSubtreeInput{RootID: "t1"})
			return err
		}},
		{"RecalculateProgress", "TASK_NOT_FOUND", "TASK_SUBTREE_FAILED", func(repo TaskRepository) error {
			_, err := NewRecalculateProgress(repo).Execute(ctx, "t1")
			return err
		}},
		{"ResolvePermission", "TASK_NOT_FOUND", "TASK_ANCESTORS_FAILED", func(repo TaskRepository) error {
			_, err := NewResolvePermission(repo, &fakeGrantRepository{}, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true}, nil).
				Execute(ctx, ResolvePermissionInput{TaskID: "t1", UserID: "user-1", Action: "read"})
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name+"/not_found", func(t *testing.T) {
			err := tc.run(erroringTaskRepository{fakeTaskRepository: newFakeTaskRepository(), err: notFound})
			assertKindAndCode(t, err, apperrors.KindNotFound, tc.notFoundCode)
		})
		t.Run(tc.name+"/infrastructure_failure", func(t *testing.T) {
			err := tc.run(erroringTaskRepository{fakeTaskRepository: newFakeTaskRepository(), err: boom})
			assertKindAndCode(t, err, apperrors.KindInternal, tc.internalCode)
		})
	}
}

func TestPublicLink_RepoNotFoundIsNotFoundAndOtherErrorsAreInternal(t *testing.T) {
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	notFound := fmt.Errorf("repo: link: %w", domain.ErrShareLinkNotFound)
	boom := errors.New("connection refused")
	tasks := newFakeTaskRepository()
	perm := NewResolvePermission(tasks, &fakeGrantRepository{}, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true}, nil)

	cases := []struct {
		name         string
		internalCode string
		run          func(links ShareLinkRepository) error
	}{
		{"ResolvePublicLink", "TASK_PUBLIC_LINK_RESOLVE_FAILED", func(links ShareLinkRepository) error {
			_, err := NewResolvePublicLink(links).Execute(ctx, "tenant-1", "token")
			return err
		}},
		{"RevokePublicLink/lookup", "TASK_PUBLIC_LINK_LOOKUP_FAILED", func(links ShareLinkRepository) error {
			return NewRevokePublicLink(links, perm, tasks).Execute(ctx, "link-1")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name+"/not_found", func(t *testing.T) {
			err := tc.run(erroringShareLinkRepository{fakeShareLinkRepository: newFakeShareLinkRepository(), err: notFound})
			assertKindAndCode(t, err, apperrors.KindNotFound, "TASK_PUBLIC_LINK_NOT_FOUND")
		})
		t.Run(tc.name+"/infrastructure_failure", func(t *testing.T) {
			err := tc.run(erroringShareLinkRepository{fakeShareLinkRepository: newFakeShareLinkRepository(), err: boom})
			assertKindAndCode(t, err, apperrors.KindInternal, tc.internalCode)
		})
	}
}

// A link that disappears between TaskIDFor and Revoke (concurrent revoke) is NotFound, not a 5xx.
func TestRevokePublicLink_ConcurrentRevoke_IsNotFound(t *testing.T) {
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "user-1"}
	links := newFakeShareLinkRepository()
	links.links["link-1"] = fakeShareLink{taskID: "t1", tenantID: "tenant-1", revoked: true}
	perm := NewResolvePermission(tasks, &fakeGrantRepository{}, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true}, nil)

	err := NewRevokePublicLink(links, perm, tasks).Execute(ctx, "link-1")
	assertKindAndCode(t, err, apperrors.KindNotFound, "TASK_PUBLIC_LINK_NOT_FOUND")
}

func TestAddEdge_UnknownEndpoint_IsNotFoundAndOtherWriteFailuresAreInternal(t *testing.T) {
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	in := AddEdgeInput{FromTaskID: "a", ToTaskID: "b", Kind: domain.EdgeKindDependsOn}

	edges := &fakeEdgeRepository{addErr: fmt.Errorf("repo: edge: %w", domain.ErrTaskNotFound)}
	_, err := NewAddEdge(newFakeTaskRepository(), edges).Execute(ctx, in)
	assertKindAndCode(t, err, apperrors.KindNotFound, "TASK_NOT_FOUND")

	edges = &fakeEdgeRepository{addErr: errors.New("connection refused")}
	_, err = NewAddEdge(newFakeTaskRepository(), edges).Execute(ctx, in)
	assertKindAndCode(t, err, apperrors.KindInternal, "TASK_EDGE_ADD_FAILED")
}

// A missing parent stays FailedPrecondition (existing contract); a parent lookup outage must not look like one.
func TestCreateTask_ParentLookup_ClassifiesMissingVsOutage(t *testing.T) {
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	in := CreateTaskInput{Title: "child", ParentID: "parent-1"}

	_, err := NewCreateTask(newFakeTaskRepository(), &fakeGrantRepository{}).Execute(ctx, in)
	assertKindAndCode(t, err, apperrors.KindFailedPrecondition, "TASK_PARENT_NOT_FOUND")

	_, err = NewCreateTask(erroringTaskRepository{fakeTaskRepository: newFakeTaskRepository(), err: errors.New("connection refused")}, &fakeGrantRepository{}).Execute(ctx, in)
	assertKindAndCode(t, err, apperrors.KindInternal, "TASK_PARENT_LOOKUP_FAILED")
}
