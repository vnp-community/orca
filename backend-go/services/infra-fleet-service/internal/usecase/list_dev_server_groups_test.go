package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestListDevServerGroups_RequiresTenantContext(t *testing.T) {
	uc := NewListDevServerGroups(&fakeDevServerGroupRepository{})
	_, err := uc.Execute(context.Background())
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestListDevServerGroups_ReturnsOnlyCallerTenantGroups(t *testing.T) {
	repo := &fakeDevServerGroupRepository{
		byTenant: map[string][]domain.DevServerGroup{
			"tenant-1": {{ID: "g1", TenantID: "tenant-1", Name: "Backend Team"}},
			"tenant-2": {{ID: "g2", TenantID: "tenant-2", Name: "Other Tenant Team"}},
		},
	}
	uc := NewListDevServerGroups(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "g1" {
		t.Errorf("expected only tenant-1's group, got %+v", got)
	}
}

func TestListDevServerGroups_RepositoryFailurePropagates(t *testing.T) {
	repo := &fakeDevServerGroupRepository{listErr: errors.New("db unavailable")}
	uc := NewListDevServerGroups(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx)
	if err == nil {
		t.Fatal("expected error to propagate from repository failure")
	}
}
