//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

const (
	testGroupParent = "88888888-8888-8888-8888-888888888888"
	testGroupChild  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
)

func setupDevServerGroupStore(t *testing.T) (*Repository, *DevServerGroupStore) {
	t.Helper()
	repo, _ := setupSshTargetStore(t)
	// Same package as Repository — .pool is unexported but this file is
	// part of package postgres, same access Repository's own methods have.
	return repo, NewDevServerGroupStore(repo.pool)
}

// TestDevServerGroupStore_CreateAndList covers CR-DS-006 Phase 1's
// DevServerGroup persistence — tenant scoping and parent_group_id
// round-tripping through NULL (root groups).
func TestDevServerGroupStore_CreateAndList(t *testing.T) {
	_, store := setupDevServerGroupStore(t)
	ctx := context.Background()

	root, err := domain.NewDevServerGroup(testGroupParent, testTenant1, "Backend Team", "")
	if err != nil {
		t.Fatalf("building root group: %v", err)
	}
	if _, err := store.Create(ctx, root); err != nil {
		t.Fatalf("creating root group: %v", err)
	}

	child, err := domain.NewDevServerGroup(testGroupChild, testTenant1, "Backend Team - Staging", testGroupParent)
	if err != nil {
		t.Fatalf("building child group: %v", err)
	}
	if _, err := store.Create(ctx, child); err != nil {
		t.Fatalf("creating child group: %v", err)
	}

	// Cross-tenant group must never show up in tenant1's listing.
	other, _ := domain.NewDevServerGroup(testUnknownID, testTenant2, "Other Tenant Team", "")
	if _, err := store.Create(ctx, other); err != nil {
		t.Fatalf("creating other-tenant group: %v", err)
	}

	got, err := store.List(ctx, testTenant1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 groups for tenant1, got %+v", got)
	}

	var foundRoot, foundChild bool
	for _, g := range got {
		if g.ID == testGroupParent {
			foundRoot = true
			if g.ParentGroupID != "" {
				t.Errorf("expected root group to have no parent, got %q", g.ParentGroupID)
			}
		}
		if g.ID == testGroupChild {
			foundChild = true
			if g.ParentGroupID != testGroupParent {
				t.Errorf("expected child group's parent to be %q, got %q", testGroupParent, g.ParentGroupID)
			}
		}
	}
	if !foundRoot || !foundChild {
		t.Errorf("expected both root and child groups in listing, got %+v", got)
	}
}

// TestRepository_RegisterAndGet_PersistsStatusAndGroupID guards CR-DS-006
// Phase 1's DevServer.Status/GroupID round-trip through Postgres — the
// exact bug class the earlier repo.list/PROJECT_MEMBERSHIP_LOOKUP_FAILED
// investigation this session found (a field silently dropped between the
// domain struct and the SQL layer).
func TestRepository_RegisterAndGet_PersistsStatusAndGroupID(t *testing.T) {
	repo, store := setupDevServerGroupStore(t)
	ctx := context.Background()

	group, err := domain.NewDevServerGroup(testGroupParent, testTenant1, "Backend Team", "")
	if err != nil {
		t.Fatalf("building group: %v", err)
	}
	if _, err := store.Create(ctx, group); err != nil {
		t.Fatalf("creating group: %v", err)
	}

	ds, err := domain.NewDevServer(testDevServer1, testTenant1, "10.0.0.1", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	ds.GroupID = testGroupParent
	if _, err := repo.Register(ctx, ds); err != nil {
		t.Fatalf("registering dev server: %v", err)
	}

	got, err := repo.Get(ctx, testTenant1, testDevServer1)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != domain.DevServerStatusPendingApproval {
		t.Errorf("want Status=%q, got %q", domain.DevServerStatusPendingApproval, got.Status)
	}
	if got.GroupID != testGroupParent {
		t.Errorf("want GroupID=%q, got %q", testGroupParent, got.GroupID)
	}
}

// TestRepository_UpdateApprovalStatus_And_AssignGroup covers CR-DS-006
// Phase 2's two new mutations against a real database.
func TestRepository_UpdateApprovalStatus_And_AssignGroup(t *testing.T) {
	repo, groupStore := setupDevServerGroupStore(t)
	ctx := context.Background()

	group, _ := domain.NewDevServerGroup(testGroupChild, testTenant1, "Backend Team", "")
	if _, err := groupStore.Create(ctx, group); err != nil {
		t.Fatalf("creating group: %v", err)
	}

	ds, err := domain.NewDevServer(testDevServer2, testTenant1, "10.0.0.2", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	if _, err := repo.Register(ctx, ds); err != nil {
		t.Fatalf("registering dev server: %v", err)
	}

	approved, err := repo.UpdateApprovalStatus(ctx, testTenant1, testDevServer2, domain.DevServerStatusApproved)
	if err != nil {
		t.Fatalf("update approval status: %v", err)
	}
	if approved.Status != domain.DevServerStatusApproved {
		t.Errorf("want Status=approved, got %q", approved.Status)
	}

	assigned, err := repo.AssignGroup(ctx, testTenant1, testDevServer2, testGroupChild)
	if err != nil {
		t.Fatalf("assign group: %v", err)
	}
	if assigned.GroupID != testGroupChild {
		t.Errorf("want GroupID=%q, got %q", testGroupChild, assigned.GroupID)
	}

	unassigned, err := repo.AssignGroup(ctx, testTenant1, testDevServer2, "")
	if err != nil {
		t.Fatalf("unassign group: %v", err)
	}
	if unassigned.GroupID != "" {
		t.Errorf("want GroupID cleared, got %q", unassigned.GroupID)
	}
}
