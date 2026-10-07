package usecase

import (
	"context"
)

const (
	DefaultSnapshotMaxBytes           int64 = 3 * 1024 * 1024   // 3 MiB
	DefaultMaxSnapshotBytesPerBinding int64 = 64 * 1024 * 1024  // 64 MiB
	DefaultMaxSnapshotBytesPerTenant  int64 = 512 * 1024 * 1024 // 512 MiB
	DefaultSnapshotKeepCommits              = 3
)

// SnapshotQuotaManager enforces binding and tenant cache size budgets.
type SnapshotQuotaManager struct {
	store              SnapshotStore
	maxSnapshotBytes   int64
	maxBytesPerBinding int64
	maxBytesPerTenant  int64
	keepCommits        int
}

// NewSnapshotQuotaManager creates a new SnapshotQuotaManager.
func NewSnapshotQuotaManager(
	store SnapshotStore,
	maxSnapshotBytes int64,
	maxBytesPerBinding int64,
	maxBytesPerTenant int64,
	keepCommits int,
) *SnapshotQuotaManager {
	if maxSnapshotBytes <= 0 {
		maxSnapshotBytes = DefaultSnapshotMaxBytes
	}
	if maxBytesPerBinding <= 0 {
		maxBytesPerBinding = DefaultMaxSnapshotBytesPerBinding
	}
	if maxBytesPerTenant <= 0 {
		maxBytesPerTenant = DefaultMaxSnapshotBytesPerTenant
	}
	if keepCommits <= 0 {
		keepCommits = DefaultSnapshotKeepCommits
	}
	return &SnapshotQuotaManager{
		store:              store,
		maxSnapshotBytes:   maxSnapshotBytes,
		maxBytesPerBinding: maxBytesPerBinding,
		maxBytesPerTenant:  maxBytesPerTenant,
		keepCommits:        keepCommits,
	}
}

// IsPayloadWithinLimit checks if the payload does not exceed maxSnapshotBytes.
func (q *SnapshotQuotaManager) IsPayloadWithinLimit(size int64) bool {
	return size <= q.maxSnapshotBytes
}

// CheckAndEvict evicts older commits if tenant capacity is exceeded.
func (q *SnapshotQuotaManager) CheckAndEvict(ctx context.Context, tenant, binding string) error {
	if q.store == nil {
		return nil
	}
	tenantBytes, err := q.store.TenantBytes(ctx, tenant)
	if err == nil && tenantBytes > q.maxBytesPerTenant {
		_ = q.store.EvictOldest(ctx, tenant, binding, q.keepCommits, q.maxBytesPerTenant)
	}
	return nil
}
