package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

type fakeVapidKeyRepository struct {
	key domain.VapidKeyMetadata
	err error
}

func (f *fakeVapidKeyRepository) GetPublicKey(ctx context.Context, tenantID string) (domain.VapidKeyMetadata, error) {
	return f.key, f.err
}

func TestGetVapidPublicKey_RequiresTenantContext(t *testing.T) {
	uc := NewGetVapidPublicKey(&fakeVapidKeyRepository{})
	if _, err := uc.Execute(context.Background()); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestGetVapidPublicKey_ReturnsPublicKey(t *testing.T) {
	repo := &fakeVapidKeyRepository{key: domain.VapidKeyMetadata{PublicKey: "pubkey-abc"}}
	uc := NewGetVapidPublicKey(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "pubkey-abc" {
		t.Errorf("expected pubkey-abc, got %s", got)
	}
}

func TestGetVapidPublicKey_NoActiveKeyMapsToNotFound(t *testing.T) {
	repo := &fakeVapidKeyRepository{err: domain.ErrNoActiveVapidKey}
	uc := NewGetVapidPublicKey(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx); err == nil {
		t.Fatal("expected an error when no active vapid key exists")
	}
}
