package contracttest

import (
	"context"
	"slices"

	"github.com/google/uuid"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeGrant struct {
	id, taskID, subject string
	level               taskv1.GrantLevel
	applyTree           bool
}

// levelActions is task-service's policy table (task_grant.rego): who may do what once a grant resolves.
var levelActions = map[taskv1.GrantLevel][]string{
	taskv1.GrantLevel_GRANT_LEVEL_OWNER: {"read", "write", "execute", "admin", "manage"},
	taskv1.GrantLevel_GRANT_LEVEL_ADMIN: {"read", "write", "execute", "admin", "manage"},
	taskv1.GrantLevel_GRANT_LEVEL_USER:  {"read", "write", "execute"},
}

// OwnTree makes owner the creator of rootID and everything below it, as CreatePlanTree does for the person who commits.
func (f *FakeTaskService) OwnTree(rootID, owner string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range f.order {
		if id == rootID || f.descendsFromLocked(id, rootID) {
			f.tasks[id].OwnerId = owner
		}
	}
}

// Grants lists "<subject>:<level>" for every grant written on taskID.
func (f *FakeTaskService) Grants(taskID string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, g := range f.grants {
		if g.taskID == taskID {
			out = append(out, g.subject+":"+g.level.String())
		}
	}
	return out
}

func (f *FakeTaskService) descendsFromLocked(id, ancestor string) bool {
	for cur := f.tasks[id]; cur != nil && cur.GetParentId() != ""; cur = f.tasks[cur.GetParentId()] {
		if cur.GetParentId() == ancestor {
			return true
		}
	}
	return false
}

// permitsLocked follows task-service: the owner of the task, a grant on it, or an inheritable grant on an ancestor
// (apply_tree) allows what the level's actions say. Tasks nobody owns stay open until AllowTreeReaders restricts them.
func (f *FakeTaskService) permitsLocked(user string, t *taskv1.Task, action string) bool {
	if t.GetOwnerId() == "" && f.treeReaders == nil {
		return true
	}
	if user == "" {
		return false
	}
	if t.GetOwnerId() == user {
		return slices.Contains(levelActions[taskv1.GrantLevel_GRANT_LEVEL_OWNER], action)
	}
	if slices.Contains(f.treeReaders, user) {
		return slices.Contains(levelActions[taskv1.GrantLevel_GRANT_LEVEL_USER], action)
	}
	for depth, cur := 0, t; cur != nil && depth < 64; depth, cur = depth+1, f.tasks[cur.GetParentId()] {
		for _, g := range f.grants {
			if g.taskID == cur.GetId() && g.subject == user && (depth == 0 || g.applyTree) && slices.Contains(levelActions[g.level], action) {
				return true
			}
		}
	}
	return false
}

// Grant requires the caller to manage the task, like task-service, and never duplicates a grant.
func (f *FakeTaskService) Grant(ctx context.Context, req *taskv1.GrantRequest) (*taskv1.GrantResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tasks[req.GetTaskId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "TASK_NOT_FOUND: no such task")
	}
	if req.GetSubjectId() == "" || req.GetLevel() == taskv1.GrantLevel_GRANT_LEVEL_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "TASK_GRANT_INVALID: subject and level are required")
	}
	if !f.permitsLocked(callerOf(ctx), t, "manage") {
		return nil, status.Error(codes.PermissionDenied, "TASK_NO_GRANT: no applicable grant found for caller")
	}
	g := fakeGrant{id: uuid.NewString(), taskID: req.GetTaskId(), subject: req.GetSubjectId(), level: req.GetLevel(), applyTree: req.GetApplyTree()}
	f.grants = append(f.grants, g)
	return &taskv1.GrantResponse{Id: g.id}, nil
}

func (f *FakeTaskService) ResolvePermission(_ context.Context, req *taskv1.ResolvePermissionRequest) (*taskv1.ResolvePermissionResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tasks[req.GetTaskId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "TASK_NOT_FOUND: no such task")
	}
	action := req.GetAction()
	if action == "" {
		action = "read"
	}
	if !f.permitsLocked(req.GetUserId(), t, action) {
		return nil, status.Error(codes.PermissionDenied, "TASK_NO_GRANT: no applicable grant found for caller")
	}
	return &taskv1.ResolvePermissionResponse{EffectiveLevel: taskv1.GrantLevel_GRANT_LEVEL_USER}, nil
}
