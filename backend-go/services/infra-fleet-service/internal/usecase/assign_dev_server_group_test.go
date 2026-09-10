package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestAssignDevServerGroup_RequiresAdmin(t *testing.T) {
	repo := &fakeDevServerRepository{byID: map[string]domain.DevServer{}}
	uc := NewAssignDevServerGroup(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, AssignDevServerGroupInput{DevServerID: "ds1", GroupID: "g1"}); err == nil {
		t.Fatal("expected an error for a non-admin caller")
	}
}

func TestAssignDevServerGroup_SetsGroup(t *testing.T) {
	repo := &fakeDevServerRepository{byID: map[string]domain.DevServer{
		"ds1": {ID: "ds1", TenantID: "tenant-1"},
	}}
	uc := NewAssignDevServerGroup(repo)

	ctx := withAdminTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, AssignDevServerGroupInput{DevServerID: "ds1", GroupID: "g1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GroupID != "g1" {
		t.Errorf("want GroupID=g1, got %q", got.GroupID)
	}
}

func TestAssignDevServerGroup_EmptyGroupIDUnassigns(t *testing.T) {
	repo := &fakeDevServerRepository{byID: map[string]domain.DevServer{
		"ds1": {ID: "ds1", TenantID: "tenant-1", GroupID: "g1"},
	}}
	uc := NewAssignDevServerGroup(repo)

	ctx := withAdminTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, AssignDevServerGroupInput{DevServerID: "ds1", GroupID: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GroupID != "" {
		t.Errorf("want GroupID cleared, got %q", got.GroupID)
	}
}
