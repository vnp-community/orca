package grpc

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// recordingWorktrees remembers the ref EnsureWorktree was asked to create from.
type recordingWorktrees struct{ gotBaseRef string }

func (r *recordingWorktrees) EnsureWorktree(_ context.Context, _ string, _ domain.Task, baseRef string) (string, string, error) {
	r.gotBaseRef = baseRef
	return "wt-1", "/srv/worktrees/wt-1", nil
}

func TestServer_Execute_BaseRefReachesTheWorktreeProvisioner(t *testing.T) {
	for _, ref := range []string{"release/x", ""} {
		tasks := newFakeTaskRepository()
		task, err := domain.NewTask("task-1", "tenant-1", "Task", domain.StatusOpen, "", "proj-1")
		if err != nil {
			t.Fatal(err)
		}
		tasks.tasks["task-1"] = task
		tasks.grants = append(tasks.grants, domain.Grant{TaskID: "task-1", SubjectID: "user-1", Level: domain.GrantLevelOwner})
		wt := &recordingWorktrees{gotBaseRef: "unset"}
		s := newTestServerWithWorktrees(tasks, &fakeEdgeRepository{}, wt)

		ctx := tenant.WithUserID(ctxWithTenant(t), "user-1")
		if _, err := s.Execute(ctx, &taskv1.TaskServiceExecuteRequest{TaskId: "task-1", RequestId: "req:r:task-1:1", BaseRef: ref}); err != nil {
			t.Fatalf("Execute(base_ref=%q): %v", ref, err)
		}
		if wt.gotBaseRef != ref {
			t.Fatalf("base_ref %q reached EnsureWorktree as %q", ref, wt.gotBaseRef)
		}
	}
}
