package snapshotstorecontract

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/domain"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/usecase"
)

// Run executes the contract test suite for SnapshotStore.
func Run(t *testing.T, newStore func() usecase.SnapshotStore, seedTenant func() string) {
	t.Run("PutAndGet", func(t *testing.T) {
		store := newStore()
		tenant := seedTenant()
		ctx := context.Background()
		
		key := domain.SnapshotKey{
			Tenant:     tenant,
			Binding:    "repo-1",
			View:       domain.ViewKindStructure,
			HeadCommit: "commit-1",
			ParamsHash: "hash-1",
		}
		
		snap := domain.Snapshot{
			Key:         key,
			ContentETag: "\"etag1\"",
			TotalCount:  10,
			CreatedAt:   time.Now(),
			ExpiresAt:   time.Now().Add(24 * time.Hour),
		}
		
		if err := store.Put(ctx, snap); err != nil {
			t.Fatalf("failed to put snapshot: %v", err)
		}
		
		retrieved, err := store.Get(ctx, key)
		if err != nil {
			t.Fatalf("failed to get snapshot: %v", err)
		}
		
		if retrieved.TotalCount != 10 {
			t.Errorf("expected TotalCount 10, got %d", retrieved.TotalCount)
		}
	})
	
	t.Run("GetLatest", func(t *testing.T) {
		store := newStore()
		tenant := seedTenant()
		ctx := context.Background()
		
		key1 := domain.SnapshotKey{
			Tenant: tenant, Binding: "repo-1", View: domain.ViewKindStructure, 
			HeadCommit: "commit-1", ParamsHash: "hash-1",
		}
		key2 := domain.SnapshotKey{
			Tenant: tenant, Binding: "repo-1", View: domain.ViewKindStructure, 
			HeadCommit: "commit-2", ParamsHash: "hash-1",
		}
		
		now := time.Now()
		
		store.Put(ctx, domain.Snapshot{Key: key1, CreatedAt: now.Add(-1 * time.Hour)})
		store.Put(ctx, domain.Snapshot{Key: key2, CreatedAt: now, TotalCount: 42})
		
		latest, err := store.GetLatest(ctx, tenant, "repo-1", domain.ViewKindStructure, "hash-1")
		if err != nil {
			t.Fatalf("failed to get latest snapshot: %v", err)
		}
		
		if latest.Key.HeadCommit != "commit-2" {
			t.Errorf("expected commit-2, got %s", latest.Key.HeadCommit)
		}
		if latest.TotalCount != 42 {
			t.Errorf("expected TotalCount 42, got %d", latest.TotalCount)
		}
	})
	
	t.Run("TenantIsolation", func(t *testing.T) {
		store := newStore()
		tenant1 := seedTenant()
		tenant2 := seedTenant()
		ctx := context.Background()
		
		key1 := domain.SnapshotKey{Tenant: tenant1, Binding: "repo", View: domain.ViewKindStructure, HeadCommit: "c", ParamsHash: "h"}
		key2 := domain.SnapshotKey{Tenant: tenant2, Binding: "repo", View: domain.ViewKindStructure, HeadCommit: "c", ParamsHash: "h"}
		
		store.Put(ctx, domain.Snapshot{Key: key1, TotalCount: 1})
		store.Put(ctx, domain.Snapshot{Key: key2, TotalCount: 2})
		
		snap1, _ := store.Get(ctx, key1)
		snap2, _ := store.Get(ctx, key2)
		
		if snap1.TotalCount != 1 || snap2.TotalCount != 2 {
			t.Errorf("tenant isolation failed")
		}
		
		store.DeleteByBinding(ctx, tenant1, "repo")
		
		_, err1 := store.Get(ctx, key1)
		var ae *apperrors.AppError
		if !errors.As(err1, &ae) || ae.Kind != apperrors.KindNotFound {
			t.Errorf("expected not found for tenant1, got %v", err1)
		}
		
		_, err2 := store.Get(ctx, key2)
		if err2 != nil {
			t.Errorf("expected tenant2 snapshot to survive, got %v", err2)
		}
	})
}
