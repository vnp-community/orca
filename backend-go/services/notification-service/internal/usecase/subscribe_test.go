package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// fakeSubscriptionRepository is an in-memory SubscriptionRepository — the
// "test against fakes, not a real database" pattern from
// specs/backend-go/standards/testing-strategy.md's unit-test section.
type fakeSubscriptionRepository struct {
	saved   []domain.PushSubscription
	saveErr error
}

func (f *fakeSubscriptionRepository) Save(ctx context.Context, sub domain.PushSubscription) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, sub)
	return nil
}

func (f *fakeSubscriptionRepository) ListByUser(ctx context.Context, tenantID, userID string) ([]domain.PushSubscription, error) {
	var out []domain.PushSubscription
	for _, s := range f.saved {
		if s.TenantID == tenantID && s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}

func TestSubscribe_RequiresTenantContext(t *testing.T) {
	uc := NewSubscribe(&fakeSubscriptionRepository{})
	_, err := uc.Execute(context.Background(), SubscribeInput{
		UserID: "user-1", Endpoint: "https://push.example/ep", P256dhKey: "p", AuthKey: "a",
	})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestSubscribe_RequiresUserID(t *testing.T) {
	uc := NewSubscribe(&fakeSubscriptionRepository{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, SubscribeInput{Endpoint: "https://push.example/ep", P256dhKey: "p", AuthKey: "a"})
	if err == nil {
		t.Fatal("expected an error when user_id is empty")
	}
}

func TestSubscribe_SavesWebSubscription(t *testing.T) {
	repo := &fakeSubscriptionRepository{}
	uc := NewSubscribe(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, SubscribeInput{
		UserID: "user-1", Endpoint: "https://push.example/ep", P256dhKey: "p256dh", AuthKey: "auth",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TenantID != "tenant-1" || got.UserID != "user-1" {
		t.Errorf("expected tenant from context and user from input, got tenant=%s user=%s", got.TenantID, got.UserID)
	}
	if got.Channel != domain.ChannelWeb {
		t.Errorf("expected web channel, got %s", got.Channel)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("expected 1 saved subscription, got %d", len(repo.saved))
	}
}

func TestSubscribe_RepositoryFailurePropagates(t *testing.T) {
	repo := &fakeSubscriptionRepository{saveErr: errors.New("db unavailable")}
	uc := NewSubscribe(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, SubscribeInput{UserID: "user-1", Endpoint: "https://push.example/ep", P256dhKey: "p", AuthKey: "a"})
	if err == nil {
		t.Fatal("expected error to propagate from repository failure")
	}
}
