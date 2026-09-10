package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// fakeSubscriptionRepository is an in-memory SubscriptionRepository — the
// "test against fakes, not a real database" pattern from
// specs/backend-go/standards/testing-strategy.md's unit-test section.
type fakeSubscriptionRepository struct {
	saved       []domain.PushSubscription
	saveErr     error
	deleteErr   error
	markExpired []string
	markErr     error
	listErr     error
}

func (f *fakeSubscriptionRepository) Save(ctx context.Context, sub domain.PushSubscription) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, sub)
	return nil
}

func (f *fakeSubscriptionRepository) ListByUser(ctx context.Context, tenantID, userID string) ([]domain.PushSubscription, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.PushSubscription
	for _, s := range f.saved {
		if s.TenantID == tenantID && s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeSubscriptionRepository) DeleteByEndpoint(ctx context.Context, endpoint string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	out := f.saved[:0]
	for _, s := range f.saved {
		if s.Endpoint != endpoint {
			out = append(out, s)
		}
	}
	f.saved = out
	return nil
}

<<<<<<< HEAD
func (f *fakeSubscriptionRepository) DeviceIDFor(ctx context.Context, subscriptionID string) (string, error) {
	for _, s := range f.saved {
		if s.ID == subscriptionID && s.DeviceID != nil {
			return *s.DeviceID, nil
		}
	}
	return "", nil
=======
func (f *fakeSubscriptionRepository) MarkExpired(ctx context.Context, endpoint string) error {
	if f.markErr != nil {
		return f.markErr
	}
	f.markExpired = append(f.markExpired, endpoint)
	return nil
>>>>>>> feat/team-rbac-implementation
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

// TestSubscribe_EmptyChannelDefaultsToWeb is a regression guard —
// TestSubscribe_SavesWebSubscription above already exercises this (no
// Channel set, expects domain.ChannelWeb) but this test names the
// guarantee explicitly, matching TASK-BE-MOBILE-001's required test list.
func TestSubscribe_EmptyChannelDefaultsToWeb(t *testing.T) {
	repo := &fakeSubscriptionRepository{}
	uc := NewSubscribe(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, SubscribeInput{
		UserID: "user-1", Endpoint: "https://push.example/ep", P256dhKey: "p256dh", AuthKey: "auth",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Channel != domain.ChannelWeb {
		t.Errorf("expected an empty Channel input to default to web, got %s", got.Channel)
	}
}

func TestSubscribe_IOSChannelWithoutWebKeysSucceeds(t *testing.T) {
	repo := &fakeSubscriptionRepository{}
	uc := NewSubscribe(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, SubscribeInput{
		UserID: "user-1", Endpoint: "ios-device-token", Channel: "ios",
	})
	if err != nil {
		t.Fatalf("unexpected error for an ios subscription with no p256dh/auth key: %v", err)
	}
	if got.Channel != domain.ChannelIOS {
		t.Errorf("expected ios channel, got %s", got.Channel)
	}
}

func TestSubscribe_UnknownChannelReturnsInvalidArgument(t *testing.T) {
	repo := &fakeSubscriptionRepository{}
	uc := NewSubscribe(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, SubscribeInput{UserID: "user-1", Endpoint: "ep", Channel: "not-a-real-channel"})
	if err == nil {
		t.Fatal("expected an error for an unknown channel")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Kind != apperrors.KindInvalidArgument {
		t.Fatalf("expected apperrors.KindInvalidArgument, got %v", err)
	}
}

func TestSubscribe_DeviceLabelPersisted(t *testing.T) {
	repo := &fakeSubscriptionRepository{}
	uc := NewSubscribe(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, SubscribeInput{
		UserID: "user-1", Endpoint: "https://push.example/ep", P256dhKey: "p", AuthKey: "a", DeviceLabel: "Chrome on Mac",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DeviceLabel != "Chrome on Mac" {
		t.Errorf("expected device_label to be persisted, got %q", got.DeviceLabel)
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
