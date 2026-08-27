package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestAddEdge_RequiresTenantContext(t *testing.T) {
	uc := NewAddEdge(&fakeEdgeRepository{})
	_, err := uc.Execute(context.Background(), AddEdgeInput{FromTaskID: "a", ToTaskID: "b", Kind: domain.EdgeKindDependsOn})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestAddEdge_PersistsAValidEdge(t *testing.T) {
	edges := &fakeEdgeRepository{}
	uc := NewAddEdge(edges)
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

// TestAddEdge_RejectsCyclicDependency is the core regression test for this
// service's most valuable logic: a proposed depends_on edge that would
// close a cycle must be rejected with FailedPrecondition BEFORE ever
// reaching the repository's Add.
func TestAddEdge_RejectsCyclicDependency(t *testing.T) {
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "a", ToTaskID: "b", Kind: domain.EdgeKindDependsOn},
		{FromTaskID: "b", ToTaskID: "c", Kind: domain.EdgeKindDependsOn},
	}}
	uc := NewAddEdge(edges)
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
	uc := NewAddEdge(edges)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, AddEdgeInput{FromTaskID: "parent", ToTaskID: "child", Kind: domain.EdgeKindParentChild})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddEdge_RejectsSelfEdgeBeforeTouchingTheRepository(t *testing.T) {
	edges := &fakeEdgeRepository{listErr: errors.New("must not be called")}
	uc := NewAddEdge(edges)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, AddEdgeInput{FromTaskID: "a", ToTaskID: "a", Kind: domain.EdgeKindDependsOn})
	if err == nil {
		t.Fatal("expected an error for a self-edge")
	}
}

func TestAddEdge_RepositoryFailurePropagates(t *testing.T) {
	edges := &fakeEdgeRepository{addErr: errors.New("db unavailable")}
	uc := NewAddEdge(edges)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, AddEdgeInput{FromTaskID: "a", ToTaskID: "b", Kind: domain.EdgeKindDependsOn})
	if err == nil {
		t.Fatal("expected error to propagate from repository failure")
	}
}
