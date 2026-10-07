package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/code-intel-service/internal/domain"
)

// SnapshotStore defines the interface for storing and retrieving code intel view snapshots.
type SnapshotStore interface {
	// Get retrieves a specific snapshot by its full key (including HeadCommit).
	Get(ctx context.Context, key domain.SnapshotKey) (domain.Snapshot, error)

	// GetLatest retrieves the most recent snapshot for a given view and params, regardless of head commit.
	GetLatest(ctx context.Context, tenant, binding string, view domain.ViewKind, paramsHash string) (domain.Snapshot, error)

	// Put stores or updates a snapshot (upsert based on key).
	Put(ctx context.Context, snapshot domain.Snapshot) error

	// DeleteByBinding removes all snapshots for a specific repository binding.
	DeleteByBinding(ctx context.Context, tenant, binding string) error

	// DeleteExpired removes snapshots that have passed their ExpiresAt. Returns number of deleted rows.
	DeleteExpired(ctx context.Context, limit int) (int, error)

	// TotalBytes returns the estimated total bytes used by all snapshots.
	TotalBytes(ctx context.Context) (int64, error)

	// TenantBytes returns the estimated total bytes used by a specific tenant's snapshots.
	TenantBytes(ctx context.Context, tenant string) (int64, error)

	// EvictOldest removes older snapshots to stay within limits, while keeping at least 'keepCommits'
	// and never deleting the absolute latest snapshot for a view.
	EvictOldest(ctx context.Context, tenant, binding string, keepCommits int, targetBytes int64) error
}
