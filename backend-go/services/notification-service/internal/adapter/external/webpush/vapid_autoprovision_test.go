package webpush

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"
)

type emptyThenStoredKeys struct {
	row     *domain.VapidKeyMetadata
	inserts atomic.Int32
}

func (k *emptyThenStoredKeys) GetPublicKey(context.Context, string) (domain.VapidKeyMetadata, error) {
	if k.row == nil {
		return domain.VapidKeyMetadata{}, domain.ErrNoActiveVapidKey
	}
	return *k.row, nil
}

func (k *emptyThenStoredKeys) InsertActiveIfAbsent(_ context.Context, key domain.VapidKeyMetadata) (domain.VapidKeyMetadata, bool, error) {
	k.inserts.Add(1)
	k.row = &key
	return key, true, nil
}

type pubBroker struct{ calls atomic.Int32 }

func (b *pubBroker) EnsureVapidSigningKey(context.Context, string) (string, error) {
	b.calls.Add(1)
	return "PROVISIONED-PUB", nil
}

func TestVapidAuthorization_ProvisionsMissingKeyOnFirstSend(t *testing.T) {
	a, _, _ := newVapid(t)
	keys, broker := &emptyThenStoredKeys{}, &pubBroker{}
	a.keys = keys
	a.WithKeyEnsurer(usecase.NewEnsureVapidKey(keys, broker, nil))

	h, err := a.Authorization(context.Background(), "tenant-new", "https://push.example/ep")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(h, ", k=PROVISIONED-PUB") {
		t.Fatalf("header %q", h)
	}
	if broker.calls.Load() != 1 || keys.inserts.Load() != 1 {
		t.Fatalf("broker=%d inserts=%d", broker.calls.Load(), keys.inserts.Load())
	}
}

func TestVapidAuthorization_MissingKeyWithoutEnsurerStillFails(t *testing.T) {
	a, _, _ := newVapid(t)
	a.keys = &emptyThenStoredKeys{}
	if _, err := a.Authorization(context.Background(), "t", "https://push.example/ep"); err == nil {
		t.Fatal("expected error")
	}
}
