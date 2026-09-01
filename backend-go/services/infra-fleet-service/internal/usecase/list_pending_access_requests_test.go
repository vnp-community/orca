package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestListPendingAccessRequests_RequiresAdmin(t *testing.T) {
	uc := NewListPendingAccessRequests(&fakeAccessRequestRepository{})
	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx); err == nil {
		t.Fatal("expected an error for a non-admin caller")
	}
}

func TestListPendingAccessRequests_ReturnsPending(t *testing.T) {
	repo := &fakeAccessRequestRepository{pending: []domain.DevServerAccessRequest{
		{ID: "req1", Status: domain.AccessRequestStatusPending},
	}}
	uc := NewListPendingAccessRequests(repo)

	ctx := withAdminTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "req1" {
		t.Errorf("unexpected result: %+v", got)
	}
}
