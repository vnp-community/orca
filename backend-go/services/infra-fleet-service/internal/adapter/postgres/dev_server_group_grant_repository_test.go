//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

const (
	testGrant1 = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	testGrant2 = "cccccccc-cccc-cccc-cccc-cccccccccccc"
)

func setupDevServerGroupGrantStore(t *testing.T) (*DevServerGroupGrantStore, *DevServerGroupStore) {
	t.Helper()
	repo, groupStore := setupDevServerGroupStore(t)
	return NewDevServerGroupGrantStore(repo.pool), groupStore
}

func TestDevServerGroupGrantStore_CreateListDelete(t *testing.T) {
	grantStore, groupStore := setupDevServerGroupGrantStore(t)
	ctx := context.Background()

	group, err := domain.NewDevServerGroup(testGroupParent, testTenant1, "Backend Team", "")
	if err != nil {
		t.Fatalf("building group: %v", err)
	}
	if _, err := groupStore.Create(ctx, group); err != nil {
		t.Fatalf("creating group: %v", err)
	}

	grant, err := domain.NewDevServerGroupGrant(testGrant1, testTenant1, testGroupParent, domain.GranteeKindDepartment, "dept-1")
	if err != nil {
		t.Fatalf("building grant: %v", err)
	}
	if _, err := grantStore.Create(ctx, grant); err != nil {
		t.Fatalf("creating grant: %v", err)
	}

	got, err := grantStore.ListByGroup(ctx, testTenant1, testGroupParent)
	if err != nil {
		t.Fatalf("list by group: %v", err)
	}
	if len(got) != 1 || got[0].ID != testGrant1 || got[0].GranteeKind != domain.GranteeKindDepartment {
		t.Fatalf("unexpected grants: %+v", got)
	}

	all, err := grantStore.ListAll(ctx, testTenant1)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 grant in ListAll, got %+v", all)
	}

	if err := grantStore.Delete(ctx, testTenant1, testGrant1); err != nil {
		t.Fatalf("delete: %v", err)
	}
	afterDelete, err := grantStore.ListAll(ctx, testTenant1)
	if err != nil {
		t.Fatalf("list all after delete: %v", err)
	}
	if len(afterDelete) != 0 {
		t.Errorf("expected 0 grants after delete, got %+v", afterDelete)
	}
}

func TestDevServerGroupGrantStore_ListAll_ScopedToTenant(t *testing.T) {
	grantStore, groupStore := setupDevServerGroupGrantStore(t)
	ctx := context.Background()

	g1, _ := domain.NewDevServerGroup(testGroupParent, testTenant1, "Backend Team", "")
	g2, _ := domain.NewDevServerGroup(testGroupChild, testTenant2, "Other Tenant Team", "")
	_, _ = groupStore.Create(ctx, g1)
	_, _ = groupStore.Create(ctx, g2)

	grant1, _ := domain.NewDevServerGroupGrant(testGrant1, testTenant1, testGroupParent, domain.GranteeKindTeam, "team-1")
	grant2, _ := domain.NewDevServerGroupGrant(testGrant2, testTenant2, testGroupChild, domain.GranteeKindTeam, "team-2")
	_, _ = grantStore.Create(ctx, grant1)
	_, _ = grantStore.Create(ctx, grant2)

	got, err := grantStore.ListAll(ctx, testTenant1)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(got) != 1 || got[0].ID != testGrant1 {
		t.Errorf("expected only tenant1's grant, got %+v", got)
	}
}
