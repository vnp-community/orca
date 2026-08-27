package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestGetDependencies_RequiresTenantContext(t *testing.T) {
	uc := NewGetDependencies(newFakeTaskRepository(), &fakeEdgeRepository{})
	if _, err := uc.Execute(context.Background(), GetDependenciesInput{TaskID: "t1"}); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestGetDependencies_RequiresTaskID(t *testing.T) {
	uc := NewGetDependencies(newFakeTaskRepository(), &fakeEdgeRepository{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	if _, err := uc.Execute(ctx, GetDependenciesInput{}); err == nil {
		t.Fatal("expected an error for a missing task_id")
	}
}

func TestGetDependencies_HydratesDependsOnTargetsInOrder(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["a"] = domain.Task{ID: "a", TenantID: "tenant-1", Title: "A"}
	tasks.tasks["b"] = domain.Task{ID: "b", TenantID: "tenant-1", Title: "B"}
	tasks.tasks["c"] = domain.Task{ID: "c", TenantID: "tenant-1", Title: "C"}
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "root", ToTaskID: "a", Kind: domain.EdgeKindDependsOn},
		{FromTaskID: "root", ToTaskID: "b", Kind: domain.EdgeKindDependsOn},
		// A parent_child edge from root — must not appear in the result.
		{FromTaskID: "root", ToTaskID: "c", Kind: domain.EdgeKindParentChild},
	}}
	uc := NewGetDependencies(tasks, edges)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, GetDependenciesInput{TaskID: "root"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("expected [a, b] matching edge order, got %+v", got)
	}
}

// TestGetDependencies_HydrationFailurePropagates asserts that a failure
// hydrating one edge's target task surfaces as an error rather than
// silently skipping it.
func TestGetDependencies_HydrationFailurePropagates(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["a"] = domain.Task{ID: "a", TenantID: "tenant-1"}
	// "missing" is never seeded into tasks, so hydrating it fails.
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "root", ToTaskID: "a", Kind: domain.EdgeKindDependsOn},
		{FromTaskID: "root", ToTaskID: "missing", Kind: domain.EdgeKindDependsOn},
	}}
	uc := NewGetDependencies(tasks, edges)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, GetDependenciesInput{TaskID: "root"})
	if err == nil {
		t.Fatal("expected an error when one dependency's task fails to hydrate")
	}
}
