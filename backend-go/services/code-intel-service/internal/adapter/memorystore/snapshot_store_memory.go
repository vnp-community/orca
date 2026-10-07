package memorystore

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/domain"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/usecase"
)

type memorySnapshotStore struct {
	mu        sync.RWMutex
	snapshots map[domain.SnapshotKey]domain.Snapshot
}

// NewMemorySnapshotStore creates a new in-memory SnapshotStore for testing.
func NewMemorySnapshotStore() usecase.SnapshotStore {
	return &memorySnapshotStore{
		snapshots: make(map[domain.SnapshotKey]domain.Snapshot),
	}
}

func (s *memorySnapshotStore) Get(ctx context.Context, key domain.SnapshotKey) (domain.Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snap, ok := s.snapshots[key]
	if !ok {
		return domain.Snapshot{}, apperrors.New(apperrors.KindNotFound, "SNAPSHOT_NOT_FOUND", "snapshot not found", nil)
	}
	return snap, nil
}

func (s *memorySnapshotStore) GetLatest(ctx context.Context, tenant, binding string, view domain.ViewKind, paramsHash string) (domain.Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var latest domain.Snapshot
	var found bool
	for key, snap := range s.snapshots {
		if key.Tenant == tenant && key.Binding == binding && key.View == view && key.ParamsHash == paramsHash {
			if !found || snap.CreatedAt.After(latest.CreatedAt) {
				latest = snap
				found = true
			}
		}
	}

	if !found {
		return domain.Snapshot{}, apperrors.New(apperrors.KindNotFound, "SNAPSHOT_NOT_FOUND", "no snapshots found", nil)
	}
	return latest, nil
}

func (s *memorySnapshotStore) Put(ctx context.Context, snapshot domain.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[snapshot.Key] = snapshot
	return nil
}

func (s *memorySnapshotStore) DeleteByBinding(ctx context.Context, tenant, binding string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key := range s.snapshots {
		if key.Tenant == tenant && key.Binding == binding {
			delete(s.snapshots, key)
		}
	}
	return nil
}

func (s *memorySnapshotStore) DeleteExpired(ctx context.Context, limit int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	deleted := 0
	for key, snap := range s.snapshots {
		if limit > 0 && deleted >= limit {
			break
		}
		if snap.ExpiresAt.Before(now) {
			delete(s.snapshots, key)
			deleted++
		}
	}
	return deleted, nil
}

func (s *memorySnapshotStore) TotalBytes(ctx context.Context) (int64, error) {
	return 0, nil // Mock implementation
}

func (s *memorySnapshotStore) TenantBytes(ctx context.Context, tenant string) (int64, error) {
	return 0, nil // Mock implementation
}

func (s *memorySnapshotStore) EvictOldest(ctx context.Context, tenant, binding string, keepCommits int, targetBytes int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Group by view and paramsHash
	groups := make(map[string][]domain.SnapshotKey)
	for key := range s.snapshots {
		if key.Tenant == tenant && key.Binding == binding {
			groupID := key.View.CacheName() + ":" + key.ParamsHash
			groups[groupID] = append(groups[groupID], key)
		}
	}

	for _, keys := range groups {
		if len(keys) <= keepCommits {
			continue
		}
		// Sort keys by CreatedAt descending
		sort.Slice(keys, func(i, j int) bool {
			return s.snapshots[keys[i]].CreatedAt.After(s.snapshots[keys[j]].CreatedAt)
		})
		
		// Remove items beyond keepCommits
		for i := keepCommits; i < len(keys); i++ {
			delete(s.snapshots, keys[i])
		}
	}
	
	return nil
}
