package usecase

import (
	"context"
	"log/slog"
)

// SnapshotInvalidator invalidates snapshots and in-memory caches upon repository index changes or resync.
type SnapshotInvalidator struct {
	store  SnapshotStore
	probe  *HeadProbe
	reader *CachedViewReader
	logger *slog.Logger
}

// NewSnapshotInvalidator creates a new SnapshotInvalidator instance.
func NewSnapshotInvalidator(store SnapshotStore, probe *HeadProbe, reader *CachedViewReader, logger *slog.Logger) *SnapshotInvalidator {
	return &SnapshotInvalidator{
		store:  store,
		probe:  probe,
		reader: reader,
		logger: logger,
	}
}

// InvalidateBinding deletes snapshots from the store, invalidates head probe, and clears symbol cache.
// It is idempotent: calling multiple times will not return an error even if no rows exist.
func (inv *SnapshotInvalidator) InvalidateBinding(ctx context.Context, tenant, binding, reason string) error {
	if inv.probe != nil {
		inv.probe.Invalidate(tenant, binding)
	}
	if inv.reader != nil {
		inv.reader.InvalidateLRU()
	}
	if inv.store != nil && tenant != "" && binding != "" {
		if err := inv.store.DeleteByBinding(ctx, tenant, binding); err != nil {
			return err
		}
	}
	return nil
}

// InvalidateProbe clears in-memory probe and symbol cache only, preserving database snapshots.
func (inv *SnapshotInvalidator) InvalidateProbe(tenant, binding string) {
	if inv.probe != nil {
		inv.probe.Invalidate(tenant, binding)
	}
	if inv.reader != nil {
		inv.reader.InvalidateLRU()
	}
}
