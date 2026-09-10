package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestAddEdge_RequiresTenantContext(t *testing.T) {
	uc := NewAddEdge(newFakeTaskRepository(), &fakeEdgeRepository{})
	_, err := uc.Execute(context.Background(), AddEdgeInput{FromTaskID: "a", ToTaskID: "b", Kind: domain.EdgeKindDependsOn})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestAddEdge_PersistsAValidEdge(t *testing.T) {
	edges := &fakeEdgeRepository{}
	tasks := newFakeTaskRepository()
	fromTask, err := domain.NewTask("a", "tenant-1", "From", domain.StatusOpen, "", "")
	if err != nil {
		t.Fatalf("building from-task: %v", err)
	}
	tasks.tasks["a"] = fromTask
	uc := NewAddEdge(tasks, edges)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, AddEdgeInput{FromTaskID: "a", ToTaskID: "b", Kind: domain.EdgeKindDependsOn})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.FromTaskID != "a" || got.ToTaskID != "b" {
		t.Errorf("unexpected edge: %+v", got)
	}
	if len(edges.edges) != 1 {
		t.Fatalf("expected 1 persisted edge, got %d", len(edges.edges))
	}
}

// TestAddEdge_AutoBlocksDependentOnUnmetDependency locks in BE-SOL-001's
// auto-block behavior: a fresh depends_on edge onto a not-done dependency
// immediately flips the dependent task to StatusBlocked.
func TestAddEdge_AutoBlocksDependentOnUnmetDependency(t *testing.T) {
	edges := &fakeEdgeRepository{}
	tasks := newFakeTaskRepository()
	fromTask, err := domain.NewTask("a", "tenant-1", "From", domain.StatusOpen, "", "")
	if err != nil {
		t.Fatalf("building from-task: %v", err)
	}
	tasks.tasks["a"] = fromTask
	toTask, err := domain.NewTask("b", "tenant-1", "To", domain.StatusOpen, "", "")
	if err != nil {
		t.Fatalf("building to-task: %v", err)
	}
	tasks.tasks["b"] = toTask
	uc := NewAddEdge(tasks, edges)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, AddEdgeInput{FromTaskID: "a", ToTaskID: "b", Kind: domain.EdgeKindDependsOn}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tasks.tasks["b"].Status; got != domain.StatusBlocked {
		t.Errorf("expected dependent task to be auto-blocked, got status %q", got)
	}
}

// TestAddEdge_DoesNotAutoBlockWhenDependencyAlreadyDone locks in the
// counterpart: a depends_on edge onto an already-done dependency must NOT
// flip the dependent task's status.
func TestAddEdge_DoesNotAutoBlockWhenDependencyAlreadyDone(t *testing.T) {
	edges := &fakeEdgeRepository{}
	tasks := newFakeTaskRepository()
	fromTask, err := domain.NewTask("a", "tenant-1", "From", domain.StatusDone, "", "")
	if err != nil {
		t.Fatalf("building from-task: %v", err)
	}
	tasks.tasks["a"] = fromTask
	toTask, err := domain.NewTask("b", "tenant-1", "To", domain.StatusOpen, "", "")
	if err != nil {
		t.Fatalf("building to-task: %v", err)
	}
	tasks.tasks["b"] = toTask
	uc := NewAddEdge(tasks, edges)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, AddEdgeInput{FromTaskID: "a", ToTaskID: "b", Kind: domain.EdgeKindDependsOn}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tasks.tasks["b"].Status; got != domain.StatusOpen {
		t.Errorf("expected dependent task's status to stay unchanged, got %q", got)
	}
}

// TestAddEdge_RejectsCyclicDependency is the core regression test for this
// service's most valuable logic: a proposed depends_on edge that would
// close a cycle must be rejected with FailedPrecondition BEFORE ever
// reaching the repository's Add.
func TestAddEdge_RejectsCyclicDependency(t *testing.T) {
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "a", ToTaskID: "b", Kind: domain.EdgeKindDependsOn},
		{FromTaskID: "b", ToTaskID: "c", Kind: domain.EdgeKindDependsOn},
	}}
	uc := NewAddEdge(newFakeTaskRepository(), edges)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	// c -> a would close the 3-hop loop a -> b -> c -> a.
	_, err := uc.Execute(ctx, AddEdgeInput{FromTaskID: "c", ToTaskID: "a", Kind: domain.EdgeKindDependsOn})
	if err == nil {
		t.Fatal("expected an error for a cyclic dependency")
	}

	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected an *apperrors.AppError, got %T: %v", err, err)
	}
	if ae.Kind != apperrors.KindFailedPrecondition {
		t.Errorf("expected KindFailedPrecondition, got %v", ae.Kind)
	}
	if ae.Code != "TASK_CYCLIC_DEPENDENCY" {
		t.Errorf("expected code TASK_CYCLIC_DEPENDENCY, got %q", ae.Code)
	}
	if len(edges.edges) != 2 {
		t.Errorf("expected the cyclic edge to NOT be persisted, got %d edges", len(edges.edges))
	}
}

func TestAddEdge_DoesNotCycleCheckParentChildEdges(t *testing.T) {
	// parent_child edges skip the cycle check entirely (single-parent
	// invariant is DB-enforced, not a DAG-cycle concern) — this must not
	// call ListByKind at all.
	edges := &fakeEdgeRepository{listErr: errors.New("ListByKind must not be called for parent_child edges")}
	uc := NewAddEdge(newFakeTaskRepository(), edges)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, AddEdgeInput{FromTaskID: "parent", ToTaskID: "child", Kind: domain.EdgeKindParentChild})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddEdge_RejectsSelfEdgeBeforeTouchingTheRepository(t *testing.T) {
	edges := &fakeEdgeRepository{listErr: errors.New("must not be called")}
	uc := NewAddEdge(newFakeTaskRepository(), edges)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, AddEdgeInput{FromTaskID: "a", ToTaskID: "a", Kind: domain.EdgeKindDependsOn})
	if err == nil {
		t.Fatal("expected an error for a self-edge")
	}
}

func TestAddEdge_RepositoryFailurePropagates(t *testing.T) {
	edges := &fakeEdgeRepository{addErr: errors.New("db unavailable")}
	uc := NewAddEdge(newFakeTaskRepository(), edges)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, AddEdgeInput{FromTaskID: "a", ToTaskID: "b", Kind: domain.EdgeKindDependsOn})
	if err == nil {
		t.Fatal("expected error to propagate from repository failure")
	}
}
