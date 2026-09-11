//go:build integration

// Integration tests run against a real MySQL via testcontainers-go — see
// repository_test.go's setupRepository, reused as-is here. Mirrors
// internal/adapter/postgres/subtree_integration_test.go's shape.
package mysql

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestGetSubtree_ReturnsAllDescendantsAndDependsOnEdges(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	root, _ := domain.NewTask(uuid.NewString(), tenantID, "root", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, root); err != nil {
		t.Fatalf("creating root: %v", err)
	}
	child, _ := domain.NewTask(uuid.NewString(), tenantID, "child", domain.StatusOpen, root.ID, "")
	if _, err := repo.Create(ctx, child); err != nil {
		t.Fatalf("creating child: %v", err)
	}
	grandchild, _ := domain.NewTask(uuid.NewString(), tenantID, "grandchild", domain.StatusOpen, child.ID, "")
	if _, err := repo.Create(ctx, grandchild); err != nil {
		t.Fatalf("creating grandchild: %v", err)
	}
	dep, _ := domain.NewTaskEdge(child.ID, grandchild.ID, domain.EdgeKindDependsOn)
	if err := repo.Add(ctx, tenantID, dep); err != nil {
		t.Fatalf("adding depends_on edge: %v", err)
	}

	tasks, edges, err := repo.GetSubtree(ctx, tenantID, root.ID, 0)
	if err != nil {
		t.Fatalf("GetSubtree: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks (root+child+grandchild), got %d: %+v", len(tasks), tasks)
	}
	if len(edges) != 1 || edges[0].FromTaskID != child.ID || edges[0].ToTaskID != grandchild.ID {
		t.Fatalf("expected exactly 1 depends_on edge, got %+v", edges)
	}
}

func TestGetSubtree_UnknownRoot_Fails(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	if _, _, err := repo.GetSubtree(ctx, uuid.NewString(), uuid.NewString(), 0); err == nil {
		t.Fatal("expected an error resolving the subtree of an unknown root")
	}
}

// TestGetSubtreeWithChildPercents_FoldsInChildProgress exercises
// JSON_ARRAYAGG (this adapter's translation of Postgres's array_agg) —
// each parent row's ChildPercents must list its direct children's CURRENT
// progress_percent, deepest node first.
func TestGetSubtreeWithChildPercents_FoldsInChildProgress(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	root, _ := domain.NewTask(uuid.NewString(), tenantID, "root", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, root); err != nil {
		t.Fatalf("creating root: %v", err)
	}
	childA, _ := domain.NewTask(uuid.NewString(), tenantID, "childA", domain.StatusOpen, root.ID, "")
	if _, err := repo.Create(ctx, childA); err != nil {
		t.Fatalf("creating childA: %v", err)
	}
	childB, _ := domain.NewTask(uuid.NewString(), tenantID, "childB", domain.StatusOpen, root.ID, "")
	if _, err := repo.Create(ctx, childB); err != nil {
		t.Fatalf("creating childB: %v", err)
	}

	if err := repo.BatchUpdateProgress(ctx, tenantID, map[string]int{childA.ID: 40, childB.ID: 60}); err != nil {
		t.Fatalf("batch update progress: %v", err)
	}

	nodes, err := repo.GetSubtreeWithChildPercents(ctx, tenantID, root.ID)
	if err != nil {
		t.Fatalf("GetSubtreeWithChildPercents: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d: %+v", len(nodes), nodes)
	}
	// deepest-first: leaf nodes (childA/childB, depth 1) come before root
	// (depth 0).
	if nodes[len(nodes)-1].Task.ID != root.ID {
		t.Fatalf("expected root to be last (deepest-first ordering), got %+v", nodes)
	}
	rootNode := nodes[len(nodes)-1]
	if len(rootNode.ChildPercents) != 2 {
		t.Fatalf("expected root's ChildPercents to list both children, got %+v", rootNode.ChildPercents)
	}
	sum := rootNode.ChildPercents[0] + rootNode.ChildPercents[1]
	if sum != 100 {
		t.Errorf("expected child percents to be {40,60} in some order (sum 100), got %+v", rootNode.ChildPercents)
	}
}

// TestGetSubtreeWithChildPercents_LeafHasEmptyChildPercents confirms the
// JSON_ARRAYAGG COALESCE-to-empty-array fallback (no children) behaves like
// Postgres's COALESCE(array_agg(...), '{}') — an empty slice, not a nil
// panic or a JSON null leaking through unmarshal.
func TestGetSubtreeWithChildPercents_LeafHasEmptyChildPercents(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	leaf, _ := domain.NewTask(uuid.NewString(), tenantID, "leaf", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, leaf); err != nil {
		t.Fatalf("creating leaf: %v", err)
	}

	nodes, err := repo.GetSubtreeWithChildPercents(ctx, tenantID, leaf.ID)
	if err != nil {
		t.Fatalf("GetSubtreeWithChildPercents: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected exactly 1 node, got %+v", nodes)
	}
	if len(nodes[0].ChildPercents) != 0 {
		t.Errorf("expected an empty ChildPercents for a leaf, got %+v", nodes[0].ChildPercents)
	}
}

func TestBatchUpdateProgress_EmptyUpdatesIsNoop(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	if err := repo.BatchUpdateProgress(ctx, uuid.NewString(), nil); err != nil {
		t.Fatalf("expected a no-op, not an error, for empty updates: %v", err)
	}
}
