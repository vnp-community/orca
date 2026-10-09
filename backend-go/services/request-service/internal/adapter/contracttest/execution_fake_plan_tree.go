package contracttest

import (
	"context"

	"github.com/google/uuid"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreatePlanTree follows task-service: one active Plan per Request (a repeat answers already_exists), a replan cancels
// the old tree, a task that depends on a sibling starts blocked and containers start open.
func (f *FakeTaskService) CreatePlanTree(ctx context.Context, req *taskv1.CreatePlanTreeRequest) (*taskv1.CreatePlanTreeResponse, error) {
	if req.GetRequestId() == "" || req.GetProjectId() == "" || req.GetTitle() == "" {
		return nil, status.Error(codes.InvalidArgument, "TASK_PLAN_TREE_INVALID: request_id, project_id and title are required")
	}
	if len(req.GetPhases()) > 0 && len(req.GetTasks()) > 0 {
		return nil, status.Error(codes.InvalidArgument, "TASK_PLAN_TREE_MIXED_CHILDREN: a plan has phases or direct tasks, not both")
	}
	if err := validatePlanTreeRequest(req); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if active := f.activePlanLocked(req.GetRequestId()); active != nil {
		switch {
		case req.GetSupersedesPlanId() == "":
			return f.planTreeResponseLocked(active, true), nil
		case req.GetSupersedesPlanId() != active.GetId():
			return nil, status.Error(codes.FailedPrecondition, "TASK_PLAN_SUPERSEDE_MISMATCH: supersedes_plan_id is not the request's active plan")
		}
		if err := f.cancelTreeLocked(active); err != nil {
			return nil, err
		}
	}

	tenantID, owner := tenantOfCall(ctx), callerOf(ctx)
	add := func(taskType, title, desc, parent, taskStatus string, labels []string, aiContext string) *taskv1.Task {
		id := uuid.NewString()
		t := &taskv1.Task{
			Id: id, RequestId: req.GetRequestId(), ProjectId: req.GetProjectId(), TaskType: taskType, Title: title, Description: desc, ParentId: parent,
			Status: taskStatus, Labels: labels, AiContext: aiContext, OwnerId: owner,
		}
		f.order = append(f.order, id)
		f.tasks[id] = t
		f.tenants[id] = tenantID
		return t
	}
	addTasks := func(parent string, in []*taskv1.PlanTreeTask) []*taskv1.Task {
		made := make([]*taskv1.Task, len(in))
		for i, pt := range in {
			st := "open"
			if len(pt.GetDependsOnIndices()) > 0 {
				st = "blocked"
			}
			made[i] = add(pt.GetTaskType(), pt.GetTitle(), pt.GetDescription(), parent, st, pt.GetLabels(), pt.GetAiContext())
		}
		for i, pt := range in {
			for _, d := range pt.GetDependsOnIndices() {
				f.deps[made[i].GetId()] = append(f.deps[made[i].GetId()], made[d].GetId())
			}
		}
		return made
	}

	plan := add("plan", req.GetTitle(), req.GetDescription(), "", "open", nil, "")
	var phases, tasks []*taskv1.Task
	for _, ph := range req.GetPhases() {
		phase := add("phase", ph.GetTitle(), ph.GetDescription(), plan.GetId(), "open", nil, "")
		phases = append(phases, phase)
		tasks = append(tasks, addTasks(phase.GetId(), ph.GetTasks())...)
	}
	for i, ph := range req.GetPhases() {
		for _, d := range ph.GetDependsOnPhaseIndices() {
			f.deps[phases[i].GetId()] = append(f.deps[phases[i].GetId()], phases[d].GetId())
		}
	}
	tasks = append(tasks, addTasks(plan.GetId(), req.GetTasks())...)

	resp := &taskv1.CreatePlanTreeResponse{Plan: clone(plan)}
	for _, p := range phases {
		resp.Phases = append(resp.Phases, clone(p))
	}
	for _, t := range tasks {
		resp.Tasks = append(resp.Tasks, clone(t))
	}
	return resp, nil
}

// validatePlanTreeRequest checks every index before anything is stored, so a refused call leaves no tasks behind.
func validatePlanTreeRequest(req *taskv1.CreatePlanTreeRequest) error {
	invalid := status.Error(codes.InvalidArgument, "TASK_PLAN_TREE_INVALID: depends_on index out of range or self-referencing")
	check := func(tasks []*taskv1.PlanTreeTask) bool {
		for i, t := range tasks {
			if t.GetTitle() == "" {
				return false
			}
			for _, d := range t.GetDependsOnIndices() {
				if d < 0 || int(d) >= len(tasks) || int(d) == i {
					return false
				}
			}
		}
		return true
	}
	if !check(req.GetTasks()) {
		return invalid
	}
	for i, ph := range req.GetPhases() {
		if ph.GetTitle() == "" || !check(ph.GetTasks()) {
			return invalid
		}
		for _, d := range ph.GetDependsOnPhaseIndices() {
			if d < 0 || int(d) >= len(req.GetPhases()) || int(d) == i {
				return invalid
			}
		}
	}
	return nil
}

func (f *FakeTaskService) activePlanLocked(requestID string) *taskv1.Task {
	for _, id := range f.order {
		if t := f.tasks[id]; t.GetTaskType() == "plan" && t.GetRequestId() == requestID && t.GetStatus() != "cancelled" {
			return t
		}
	}
	return nil
}

func (f *FakeTaskService) planTreeResponseLocked(plan *taskv1.Task, existing bool) *taskv1.CreatePlanTreeResponse {
	resp := &taskv1.CreatePlanTreeResponse{Plan: clone(plan), AlreadyExists: existing}
	for _, id := range f.order {
		t := f.tasks[id]
		switch {
		case t.GetParentId() == plan.GetId() && t.GetTaskType() == "phase":
			resp.Phases = append(resp.Phases, clone(t))
		case t.GetTaskType() == "phase" || t.GetTaskType() == "plan":
		case t.GetParentId() == plan.GetId() || f.isPhaseOfLocked(t.GetParentId(), plan.GetId()):
			resp.Tasks = append(resp.Tasks, clone(t))
		}
	}
	return resp
}

func (f *FakeTaskService) isPhaseOfLocked(parentID, planID string) bool {
	p := f.tasks[parentID]
	return p != nil && p.GetTaskType() == "phase" && p.GetParentId() == planID
}

// cancelTreeLocked refuses a replan while a task runs, like task-service, then cancels the unfinished tasks and the containers.
func (f *FakeTaskService) cancelTreeLocked(plan *taskv1.Task) error {
	var members []*taskv1.Task
	for _, id := range f.order {
		t := f.tasks[id]
		if t.GetId() == plan.GetId() || t.GetParentId() == plan.GetId() || f.isPhaseOfLocked(t.GetParentId(), plan.GetId()) {
			members = append(members, t)
		}
	}
	for _, t := range members {
		if t.GetTaskType() != "plan" && t.GetTaskType() != "phase" && t.GetStatus() == "in_progress" {
			return status.Error(codes.FailedPrecondition, "TASK_PLAN_HAS_RUNNING_TASKS: the plan has running tasks and cannot be superseded")
		}
	}
	for _, t := range members {
		if t.GetStatus() != "done" && t.GetStatus() != "cancelled" {
			t.Status = "cancelled"
		}
	}
	return nil
}
