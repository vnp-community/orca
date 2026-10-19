package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
)

func TestExecuteTask_PassesBaseRefToTheWorktreeProvisioner(t *testing.T) {
	for _, tc := range []struct{ name, requestID, baseRef string }{
		{"request task with a ref", "req-1", "release/x"},
		{"request task without a ref", "req-1", ""},
		{"task run on its own", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tasks := newFakeTaskRepository()
			grants := &fakeGrantRepository{}
			seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
			uc, worktrees, _, _, _ := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, &fakeSimpleExecutor{ref: "r"}, &fakeExecutor{}, grants)
			worktrees.gotBaseRef = "unset"

			ctx := withIdentity(context.Background(), "tenant-1", "user-1")
			if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: tc.requestID, BaseRef: tc.baseRef}); err != nil {
				t.Fatal(err)
			}
			if worktrees.gotBaseRef != tc.baseRef {
				t.Fatalf("EnsureWorktree got base ref %q, want %q", worktrees.gotBaseRef, tc.baseRef)
			}
		})
	}
}

func TestExecuteTask_WorktreeProvisionFailure_IsWorktreeFailedWithTheCause(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	simple := &fakeSimpleExecutor{ref: "r"}
	uc, worktrees, _, _, _ := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, simple, &fakeExecutor{}, grants)
	cause := errors.New("resolve default branch of repo: unavailable")
	worktrees.err = cause

	_, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "TASK_EXECUTE_WORKTREE_FAILED" || !errors.Is(err, cause) {
		t.Fatalf("want TASK_EXECUTE_WORKTREE_FAILED wrapping the cause, got %v", err)
	}
	if simple.called {
		t.Fatal("the agent must not start without a worktree")
	}
}
