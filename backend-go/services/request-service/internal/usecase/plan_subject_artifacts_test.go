package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func planTree(req domain.Request) (*exTasks, domain.TaskView, domain.TaskView) {
	tasks := newExTasks()
	plan := tasks.add(domain.TaskView{Type: domain.TaskTypePlan, Status: "open", RequestID: req.ID, Title: "Plan"})
	leaf := tasks.add(domain.TaskView{Type: "task", Status: "open", RequestID: req.ID, ParentID: plan.ID, Title: "Do it"})
	return tasks, plan, leaf
}

func TestPlanArtifacts_DigestFollowsTheContentTheApproverSaw(t *testing.T) {
	req := domain.Request{ID: "r1", Type: domain.RequestTypeTask}
	tasks, plan, leaf := planTree(req)
	req.PlanTaskID = plan.ID
	a := &PlanArtifacts{Tasks: tasks}

	id, d1, err := a.Describe(context.Background(), req, domain.SubjectTaskList, "")
	if err != nil || id != plan.ID || d1 == "" {
		t.Fatalf("describe: %q %q %v", id, d1, err)
	}
	_, same, _ := a.Describe(context.Background(), req, domain.SubjectTaskList, plan.ID)
	if same != d1 {
		t.Fatal("the digest must be stable")
	}
	edited := tasks.tasks[leaf.ID]
	edited.Title = "Do something else"
	tasks.tasks[leaf.ID] = edited
	_, d2, _ := a.Describe(context.Background(), req, domain.SubjectTaskList, "")
	if d2 == d1 {
		t.Fatal("editing a task after the approval opened must change the digest")
	}
	_, other, _ := a.Describe(context.Background(), req, domain.SubjectPlan, "")
	if other == d2 {
		t.Fatal("the digest is bound to the subject type")
	}
}

func TestPlanArtifacts_Refusals(t *testing.T) {
	req := domain.Request{ID: "r1", Type: domain.RequestTypeTask}
	tasks, plan, _ := planTree(req)
	a := &PlanArtifacts{Tasks: tasks}

	_, _, err := a.Describe(context.Background(), req, domain.SubjectPlan, "")
	requireCode(t, err, "REQUEST_PLAN_NOT_COMMITTED")

	req.PlanTaskID = plan.ID
	_, _, err = a.Describe(context.Background(), req, domain.SubjectPlan, "someone-elses")
	requireCode(t, err, "REQUEST_PHASE_NOT_IN_PLAN")

	foreign := req
	foreign.ID = "r2"
	_, _, err = a.Describe(context.Background(), foreign, domain.SubjectPlan, "")
	requireCode(t, err, "REQUEST_PHASE_NOT_IN_PLAN")

	cancelled := tasks.tasks[plan.ID]
	cancelled.Status = domain.TaskStatusCancelled
	tasks.tasks[plan.ID] = cancelled
	_, _, err = a.Describe(context.Background(), req, domain.SubjectPlan, "")
	requireCode(t, err, "REQUEST_PHASE_NOT_IN_PLAN")

	tasks.listErr = errors.New("down")
	_, _, err = a.Describe(context.Background(), req, domain.SubjectPlan, "")
	if err == nil {
		t.Fatal("a task-service outage must refuse, not approve blind")
	}
}
