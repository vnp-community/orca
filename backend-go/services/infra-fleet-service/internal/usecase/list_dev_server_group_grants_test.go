package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestListDevServerGroupGrants_RequiresAdmin(t *testing.T) {
	uc := NewListDevServerGroupGrants(&fakeDevServerGroupGrantRepository{})
	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, ""); err == nil {
		t.Fatal("expected an error for a non-admin caller")
	}
}

func TestListDevServerGroupGrants_EmptyGroupIDListsAll(t *testing.T) {
	repo := &fakeDevServerGroupGrantRepository{byTenant: map[string][]domain.DevServerGroupGrant{
		"tenant-1": {
			{ID: "g1", TenantID: "tenant-1", DevServerGroupID: "group-a"},
			{ID: "g2", TenantID: "tenant-1", DevServerGroupID: "group-b"},
		},
	}}
	uc := NewListDevServerGroupGrants(repo)

	ctx := withAdminTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 grants, got %+v", got)
	}
}

func TestListDevServerGroupGrants_FiltersByGroup(t *testing.T) {
	repo := &fakeDevServerGroupGrantRepository{byTenant: map[string][]domain.DevServerGroupGrant{
		"tenant-1": {
			{ID: "g1", TenantID: "tenant-1", DevServerGroupID: "group-a"},
			{ID: "g2", TenantID: "tenant-1", DevServerGroupID: "group-b"},
		},
	}}
	uc := NewListDevServerGroupGrants(repo)

	ctx := withAdminTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, "group-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "g1" {
		t.Errorf("expected only group-a's grant, got %+v", got)
	}
}
