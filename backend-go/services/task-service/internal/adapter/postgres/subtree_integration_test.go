//go:build integration

// See repository_test.go's file header for the shared testcontainers setup.
package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// TestRepository_GetSubtree_ReturnsOnlyDescendants builds a 5-node,
// 2-branch tree (root -> {a -> a1, b}) plus a same-id-shaped task under a
// different tenant, and asserts: root's subtree is all 5 of its own nodes;
// a mid-tree node (a)'s subtree is {a, a1} only — never sibling b, never
// root; and the other tenant's task never leaks in despite sharing no
// actual overlap in ID space (UUIDs), proving the explicit tenant_id filter
// does its job rather than relying on RLS alone.
func TestRepository_GetSubtree_ReturnsOnlyDescendants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	otherTenantID := uuid.NewString()

	root, _ := domain.NewTask(uuid.NewString(), tenantID, "root", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, root); err != nil {
		t.Fatalf("creating root: %v", err)
	}
	a, _ := domain.NewTask(uuid.NewString(), tenantID, "a", domain.StatusOpen, root.ID, "")
	if _, err := repo.Create(ctx, a); err != nil {
		t.Fatalf("creating a: %v", err)
	}
	b, _ := domain.NewTask(uuid.NewString(), tenantID, "b", domain.StatusOpen, root.ID, "")
	if _, err := repo.Create(ctx, b); err != nil {
		t.Fatalf("creating b: %v", err)
	}
	a1, _ := domain.NewTask(uuid.NewString(), tenantID, "a1", domain.StatusOpen, a.ID, "")
	if _, err := repo.Create(ctx, a1); err != nil {
		t.Fatalf("creating a1: %v", err)
	}

	otherRoot, _ := domain.NewTask(uuid.NewString(), otherTenantID, "other-root", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, otherRoot); err != nil {
		t.Fatalf("creating other-tenant root: %v", err)
	}

	rootSubtree, err := repo.GetSubtree(ctx, tenantID, root.ID)
	if err != nil {
		t.Fatalf("GetSubtree(root): %v", err)
	}
	if len(rootSubtree) != 4 {
		t.Fatalf("expected root's subtree to have 4 nodes (root, a, b, a1), got %d: %+v", len(rootSubtree), rootSubtree)
	}

	aSubtree, err := repo.GetSubtree(ctx, tenantID, a.ID)
	if err != nil {
		t.Fatalf("GetSubtree(a): %v", err)
	}
	if len(aSubtree) != 2 {
		t.Fatalf("expected a's subtree to have 2 nodes (a, a1), got %d: %+v", len(aSubtree), aSubtree)
	}
	ids := map[string]bool{}
	for _, task := range aSubtree {
		ids[task.ID] = true
	}
	if !ids[a.ID] || !ids[a1.ID] {
		t.Errorf("expected a's subtree to contain a and a1, got %+v", aSubtree)
	}
	if ids[b.ID] || ids[root.ID] {
		t.Errorf("expected a's subtree to exclude sibling b and root, got %+v", aSubtree)
	}

	// Cross-tenant isolation: querying otherRoot's id under tenantID (the
	// wrong tenant) must return nothing, not otherRoot's real subtree.
	crossTenant, err := repo.GetSubtree(ctx, tenantID, otherRoot.ID)
	if err != nil {
		t.Fatalf("GetSubtree(otherRoot under wrong tenant): %v", err)
	}
	if len(crossTenant) != 0 {
		t.Errorf("expected no rows for a task ID under the wrong tenant, got %+v", crossTenant)
	}
}
