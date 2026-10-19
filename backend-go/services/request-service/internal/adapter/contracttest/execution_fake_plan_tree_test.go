package contracttest

import (
	"context"
	"testing"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func planRequest(requestID string) *taskv1.CreatePlanTreeRequest {
	return &taskv1.CreatePlanTreeRequest{
		RequestId: requestID, ProjectId: "p1", Title: "Plan",
		Phases: []*taskv1.PlanTreePhase{
			{Title: "P1", Tasks: []*taskv1.PlanTreeTask{{Title: "a", TaskType: "task"}, {Title: "b", TaskType: "task", DependsOnIndices: []int32{0}}}},
			{Title: "P2", DependsOnPhaseIndices: []int32{0}, Tasks: []*taskv1.PlanTreeTask{{Title: "c", TaskType: "task"}}},
		},
	}
}

func TestFakeTaskService_CreatePlanTreeFollowsTaskService(t *testing.T) {
	f := NewFakeTaskService()
	ctx := context.Background()
	first, err := f.CreatePlanTree(ctx, planRequest("r1"))
	if err != nil || first.GetAlreadyExists() || len(first.GetPhases()) != 2 || len(first.GetTasks()) != 3 {
		t.Fatalf("first create = %+v, %v", first, err)
	}
	byTitle := map[string]string{}
	for _, task := range f.TasksOfRequest("r1") {
		byTitle[task.GetTitle()] = task.GetStatus()
	}
	if byTitle["a"] != "open" || byTitle["b"] != "blocked" || byTitle["P2"] != "open" || byTitle["Plan"] != "open" {
		t.Fatalf("statuses = %v: a task that depends on a sibling starts blocked, containers open", byTitle)
	}

	again, err := f.CreatePlanTree(ctx, planRequest("r1"))
	if err != nil || !again.GetAlreadyExists() || again.GetPlan().GetId() != first.GetPlan().GetId() || len(again.GetTasks()) != 3 {
		t.Fatalf("a repeat must return the same Plan: %+v, %v", again, err)
	}
	if n := len(f.TasksOfRequest("r1")); n != 6 {
		t.Fatalf("a repeat created tasks: %d", n)
	}
}

func TestFakeTaskService_ReplanCancelsTheOldTreeUnlessSomethingRuns(t *testing.T) {
	f := NewFakeTaskService()
	ctx := context.Background()
	first, _ := f.CreatePlanTree(ctx, planRequest("r1"))

	bad := planRequest("r1")
	bad.SupersedesPlanId = "not-the-plan"
	if _, err := f.CreatePlanTree(ctx, bad); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("wrong supersede id: %v", err)
	}

	f.tasks[first.GetTasks()[0].GetId()].Status = "in_progress"
	replan := planRequest("r1")
	replan.SupersedesPlanId = first.GetPlan().GetId()
	if _, err := f.CreatePlanTree(ctx, replan); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("a running task blocks the replan: %v", err)
	}

	f.tasks[first.GetTasks()[0].GetId()].Status = "open"
	second, err := f.CreatePlanTree(ctx, replan)
	if err != nil || second.GetAlreadyExists() || second.GetPlan().GetId() == first.GetPlan().GetId() {
		t.Fatalf("replan = %+v, %v", second, err)
	}
	if f.Task(first.GetPlan().GetId()).GetStatus() != "cancelled" || f.Task(first.GetTasks()[1].GetId()).GetStatus() != "cancelled" {
		t.Fatal("the old tree must be cancelled")
	}
}

func TestFakeTaskService_CreatePlanTreeRejectsBadInput(t *testing.T) {
	f := NewFakeTaskService()
	for name, req := range map[string]*taskv1.CreatePlanTreeRequest{
		"no title":     {RequestId: "r", ProjectId: "p"},
		"mixed":        {RequestId: "r", ProjectId: "p", Title: "t", Phases: []*taskv1.PlanTreePhase{{Title: "x"}}, Tasks: []*taskv1.PlanTreeTask{{Title: "y"}}},
		"self-depends": {RequestId: "r", ProjectId: "p", Title: "t", Tasks: []*taskv1.PlanTreeTask{{Title: "y", DependsOnIndices: []int32{0}}}},
		"bad index":    {RequestId: "r", ProjectId: "p", Title: "t", Tasks: []*taskv1.PlanTreeTask{{Title: "y", DependsOnIndices: []int32{4}}}},
	} {
		if _, err := f.CreatePlanTree(context.Background(), req); status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: %v", name, err)
		}
	}
}
