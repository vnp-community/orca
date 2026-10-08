package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func seedHierarchy(repo *fakeTaskRepository) {
	repo.tasks["plan-1"] = domain.Task{ID: "plan-1", TenantID: "tenant-1", Title: "Plan", Type: domain.TypePlan, ProjectID: "proj-1", RequestID: "req-1", Status: domain.StatusOpen}
	repo.tasks["work-1"] = domain.Task{ID: "work-1", TenantID: "tenant-1", Title: "Work", Type: domain.TypeTask, ProjectID: "proj-1", Status: domain.StatusOpen}
}

func TestCreateTask_PlanWithParent_Rejected(t *testing.T) {
	repo := newFakeTaskRepository()
	seedHierarchy(repo)
	uc := NewCreateTask(repo, nil)
	_, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "u"), CreateTaskInput{Title: "p2", Type: "plan", ProjectID: "proj-1", ParentID: "plan-1"})
	if !hasCode(err, "TASK_PLAN_CANNOT_HAVE_PARENT") || !isKind(err, apperrors.KindFailedPrecondition) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateTask_PlanWithoutProject_Rejected(t *testing.T) {
	uc := NewCreateTask(newFakeTaskRepository(), nil)
	_, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "u"), CreateTaskInput{Title: "p", Type: "plan"})
	if !hasCode(err, "TASK_PLAN_PROJECT_REQUIRED") || !isKind(err, apperrors.KindInvalidArgument) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateTask_PhaseUnderTask_Rejected(t *testing.T) {
	repo := newFakeTaskRepository()
	seedHierarchy(repo)
	uc := NewCreateTask(repo, nil)
	_, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "u"), CreateTaskInput{Title: "ph", Type: "phase", ProjectID: "proj-1", ParentID: "work-1"})
	if !hasCode(err, "TASK_CONTAINER_UNDER_WORK_TASK") {
		t.Fatalf("got %v", err)
	}
}

func TestCreateTask_PhaseWithoutPlanParent_Rejected(t *testing.T) {
	uc := NewCreateTask(newFakeTaskRepository(), nil)
	_, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "u"), CreateTaskInput{Title: "ph", Type: "phase", ProjectID: "proj-1"})
	if !hasCode(err, "TASK_PHASE_REQUIRES_PLAN_PARENT") {
		t.Fatalf("got %v", err)
	}
}

func TestCreateTask_PhaseUnderPlan_OK(t *testing.T) {
	repo := newFakeTaskRepository()
	seedHierarchy(repo)
	uc := NewCreateTask(repo, nil)
	got, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "u"), CreateTaskInput{Title: "ph", Type: "phase", ProjectID: "proj-1", ParentID: "plan-1"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.Type != domain.TypePhase || got.RequestID != "req-1" {
		t.Errorf("unexpected phase: %+v", got)
	}
}

func TestCreateTask_PlanUnderWorkTask_Rejected(t *testing.T) {
	repo := newFakeTaskRepository()
	seedHierarchy(repo)
	uc := NewCreateTask(repo, nil)
	_, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "u"), CreateTaskInput{Title: "p", Type: "plan", ProjectID: "proj-1", ParentID: "work-1"})
	if !hasCode(err, "TASK_CONTAINER_UNDER_WORK_TASK") {
		t.Fatalf("got %v", err)
	}
}

func TestCreateTask_InvalidType(t *testing.T) {
	uc := NewCreateTask(newFakeTaskRepository(), nil)
	_, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "u"), CreateTaskInput{Title: "x", Type: "xyz"})
	if !hasCode(err, "TASK_INVALID_TYPE") || !isKind(err, apperrors.KindInvalidArgument) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateTask_InheritsRequestID(t *testing.T) {
	repo := newFakeTaskRepository()
	seedHierarchy(repo)
	uc := NewCreateTask(repo, nil)
	ctx := withIdentity(context.Background(), "tenant-1", "u")
	got, err := uc.Execute(ctx, CreateTaskInput{Title: "child", ParentID: "plan-1"})
	if err != nil || got.RequestID != "req-1" {
		t.Fatalf("expected inherited req-1, got %+v, %v", got, err)
	}
	explicit, err := uc.Execute(ctx, CreateTaskInput{Title: "child2", ParentID: "plan-1", RequestID: "req-2"})
	if err != nil || explicit.RequestID != "req-2" {
		t.Fatalf("explicit request id must win, got %+v, %v", explicit, err)
	}
}

func TestCreateTask_InvalidPriority(t *testing.T) {
	uc := NewCreateTask(newFakeTaskRepository(), nil)
	_, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "u"), CreateTaskInput{Title: "x", Priority: "critical"})
	if !hasCode(err, "TASK_INVALID") || !isKind(err, apperrors.KindInvalidArgument) {
		t.Fatalf("got %v", err)
	}
	_, err = uc.Execute(withIdentity(context.Background(), "tenant-1", "u"), CreateTaskInput{Title: "x", Visibility: "secret"})
	if !hasCode(err, "TASK_INVALID") {
		t.Fatalf("visibility: got %v", err)
	}
}

func TestCreateTask_PersistsTypeAndLabels(t *testing.T) {
	repo := newFakeTaskRepository()
	uc := NewCreateTask(repo, nil)
	got, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "u"), CreateTaskInput{Title: "x", Type: "bug", Priority: "high", Labels: []string{"a"}})
	if err != nil || got.Type != "bug" || got.Priority != "high" || len(got.Labels) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
}
