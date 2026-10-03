//go:build integration

package postgres

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

func newVapidKey(tenantID, pub string) domain.VapidKeyMetadata {
	return domain.VapidKeyMetadata{
		KeyID: uuid.NewString(), TenantID: tenantID, PublicKey: pub,
		VaultKeyRef: "vapid-signing-" + tenantID, Status: domain.VapidKeyActive, CreatedAt: time.Now().UTC(),
	}
}

func TestRepository_InsertActiveIfAbsent_ConcurrentInsertsYieldOneRow(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	const n = 12
	var wg sync.WaitGroup
	stored := make([]domain.VapidKeyMetadata, n)
	inserted := make([]bool, n)
	errs := make([]error, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			stored[i], inserted[i], errs[i] = repo.InsertActiveIfAbsent(ctx, newVapidKey(tenantID, "pub-"+uuid.NewString()))
		}()
	}
	close(start)
	wg.Wait()

	winners := 0
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("insert %d: %v", i, errs[i])
		}
		if inserted[i] {
			winners++
		}
		if stored[i].KeyID != stored[0].KeyID || stored[i].PublicKey != stored[0].PublicKey {
			t.Fatalf("callers saw different rows: %+v vs %+v", stored[i], stored[0])
		}
	}
	if winners != 1 {
		t.Fatalf("winners = %d, want exactly 1", winners)
	}
	got, err := repo.GetPublicKey(ctx, tenantID)
	if err != nil || got.KeyID != stored[0].KeyID || got.Status != domain.VapidKeyActive {
		t.Fatalf("GetPublicKey: %+v %v", got, err)
	}
}

func TestRepository_InsertActiveIfAbsent_ExistingKeyWinsAndTenantsIsolated(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	a, b := uuid.NewString(), uuid.NewString()

	first, inserted, err := repo.InsertActiveIfAbsent(ctx, newVapidKey(a, "pub-1"))
	if err != nil || !inserted {
		t.Fatalf("first: inserted=%v err=%v", inserted, err)
	}
	second, inserted, err := repo.InsertActiveIfAbsent(ctx, newVapidKey(a, "pub-2"))
	if err != nil || inserted || second.PublicKey != "pub-1" || second.KeyID != first.KeyID {
		t.Fatalf("second: %+v inserted=%v err=%v", second, inserted, err)
	}
	other, inserted, err := repo.InsertActiveIfAbsent(ctx, newVapidKey(b, "pub-b"))
	if err != nil || !inserted || other.PublicKey != "pub-b" {
		t.Fatalf("other tenant: %+v inserted=%v err=%v", other, inserted, err)
	}
}
