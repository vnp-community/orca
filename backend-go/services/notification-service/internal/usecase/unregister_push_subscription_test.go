package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

func TestUnregisterPushSubscription_DeletesKnownEndpoint(t *testing.T) {
	repo := &fakeSubscriptionRepository{saved: []domain.PushSubscription{
		{ID: "sub-1", TenantID: "tenant-1", UserID: "user-1", Endpoint: "https://push.example/ep-1"},
		{ID: "sub-2", TenantID: "tenant-1", UserID: "user-1", Endpoint: "https://push.example/ep-2"},
	}}
	uc := NewUnregisterPushSubscription(repo)

	if err := uc.Execute(context.Background(), "https://push.example/ep-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.saved) != 1 || repo.saved[0].Endpoint != "https://push.example/ep-2" {
		t.Fatalf("expected only ep-2 to remain, got %+v", repo.saved)
	}
}

// TestUnregisterPushSubscription_UnknownEndpointIsNoop is the idempotency
// guard SOL-003 calls out explicitly: re-deleting an already-gone endpoint
// must not be an error.
func TestUnregisterPushSubscription_UnknownEndpointIsNoop(t *testing.T) {
	repo := &fakeSubscriptionRepository{}
	uc := NewUnregisterPushSubscription(repo)

	if err := uc.Execute(context.Background(), "https://push.example/never-subscribed"); err != nil {
		t.Fatalf("unexpected error for an unknown endpoint: %v", err)
	}
}

func TestUnregisterPushSubscription_RequiresEndpoint(t *testing.T) {
	uc := NewUnregisterPushSubscription(&fakeSubscriptionRepository{})
	if err := uc.Execute(context.Background(), ""); err == nil {
		t.Fatal("expected an error for an empty endpoint")
	}
}

func TestUnregisterPushSubscription_RepositoryFailurePropagates(t *testing.T) {
	repo := &fakeSubscriptionRepository{deleteErr: context.DeadlineExceeded}
	uc := NewUnregisterPushSubscription(repo)
	if err := uc.Execute(context.Background(), "https://push.example/ep-1"); err == nil {
		t.Fatal("expected error to propagate from repository failure")
	}
}
