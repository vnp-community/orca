package usecase

import (
	"context"
	"log/slog"
	"time"
)

const (
	DefaultMaintenanceInterval = 10 * time.Minute
	DefaultExpiredBatchLimit   = 500
)

// SnapshotJanitor runs scheduled maintenance to clean expired snapshots.
type SnapshotJanitor struct {
	store      SnapshotStore
	interval   time.Duration
	batchLimit int
	logger     *slog.Logger
}

// NewSnapshotJanitor creates a new SnapshotJanitor instance.
func NewSnapshotJanitor(store SnapshotStore, interval time.Duration, batchLimit int, logger *slog.Logger) *SnapshotJanitor {
	if interval <= 0 {
		interval = DefaultMaintenanceInterval
	}
	if batchLimit <= 0 {
		batchLimit = DefaultExpiredBatchLimit
	}
	return &SnapshotJanitor{
		store:      store,
		interval:   interval,
		batchLimit: batchLimit,
		logger:     logger,
	}
}

// RunOnce performs a single pass of cleaning expired snapshots.
func (j *SnapshotJanitor) RunOnce(ctx context.Context) (int, error) {
	if j.store == nil {
		return 0, nil
	}
	return j.store.DeleteExpired(ctx, j.batchLimit)
}

// Start runs the periodic janitor loop until ctx is cancelled.
func (j *SnapshotJanitor) Start(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = j.RunOnce(ctx)
		}
	}
}
