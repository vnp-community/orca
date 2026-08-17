package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestExecuteTask_RequiresTenantContext(t *testing.T) {
	uc := NewExecuteTask(&fakeEdgeRepository{}, &fakeExecutor{}, &fakeExecutor{})
	_, err := uc.Execute(context.Background(), ExecuteTaskInput{TaskID: "t1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// TestExecuteTask_SimplePath_NoSubtasksNoDependencies is the core branching
// regression: a task with neither parent_child nor depends_on edges FROM it
// must dispatch to SimpleExecutor, never ComplexExecutor.
func TestExecuteTask_SimplePath_NoSubtasksNoDependencies(t *testing.T) {
	edges := &fakeEdgeRepository{}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc := NewExecuteTask(edges, simple, complex)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	ref, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "infra-fleet-ref-1" {
		t.Errorf("expected the simple executor's ref, got %q", ref)
	}
	if !simple.called {
		t.Error("expected SimpleExecutor to be called")
	}
	if complex.called {
		t.Error("expected ComplexExecutor NOT to be called")
	}
}

func TestExecuteTask_ComplexPath_HasSubtasks(t *testing.T) {
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "task-1", ToTaskID: "subtask-1", Kind: domain.EdgeKindParentChild},
	}}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc := NewExecuteTask(edges, simple, complex)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	ref, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "orchestration-ref-1" {
		t.Errorf("expected the complex executor's ref, got %q", ref)
	}
	if !complex.called {
		t.Error("expected ComplexExecutor to be called")
	}
	if simple.called {
		t.Error("expected SimpleExecutor NOT to be called")
	}
}

func TestExecuteTask_ComplexPath_HasDependencies(t *testing.T) {
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "task-1", ToTaskID: "blocking-task", Kind: domain.EdgeKindDependsOn},
	}}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc := NewExecuteTask(edges, simple, complex)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	ref, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "orchestration-ref-1" {
		t.Errorf("expected the complex executor's ref, got %q", ref)
	}
	if !complex.called || simple.called {
		t.Errorf("expected only ComplexExecutor to be called, complex=%v simple=%v", complex.called, simple.called)
	}
}

func TestExecuteTask_IgnoresEdgesToTheTaskWhenDecidingComplexity(t *testing.T) {
	// task-1 is someone else's dependency (an edge TO it, not FROM it) —
	// that must not make task-1 itself "complex".
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "other-task", ToTaskID: "task-1", Kind: domain.EdgeKindDependsOn},
	}}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc := NewExecuteTask(edges, simple, complex)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !simple.called || complex.called {
		t.Errorf("expected only SimpleExecutor to be called, simple=%v complex=%v", simple.called, complex.called)
	}
}

func TestExecuteTask_ExecutorFailurePropagates(t *testing.T) {
	edges := &fakeEdgeRepository{}
	simple := &fakeExecutor{err: errors.New("infra-fleet-service unavailable")}
	uc := NewExecuteTask(edges, simple, &fakeExecutor{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err == nil {
		t.Fatal("expected error to propagate from executor failure")
	}
}

func TestExecuteTask_RequiresTaskID(t *testing.T) {
	uc := NewExecuteTask(&fakeEdgeRepository{}, &fakeExecutor{}, &fakeExecutor{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{RequestID: "req-1"}); err == nil {
		t.Fatal("expected an error for an empty task_id")
	}
}
