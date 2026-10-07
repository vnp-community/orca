package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/code-intel-service/internal/domain"
)

type quotaMockStore struct {
	fakeStore
	evictedTenant  string
	evictedBinding string
	tenantBytesVal int64
}

func (q *quotaMockStore) TenantBytes(ctx context.Context, tenant string) (int64, error) {
	return q.tenantBytesVal, nil
}

func (q *quotaMockStore) EvictOldest(ctx context.Context, tenant, binding string, keepCommits int, targetBytes int64) error {
	q.evictedTenant = tenant
	q.evictedBinding = binding
	return nil
}

func TestSnapshotQuota_PayloadLimit(t *testing.T) {
	quota := NewSnapshotQuotaManager(nil, 3*1024*1024, 64*1024*1024, 512*1024*1024, 3)

	// Within 3 MiB
	if !quota.IsPayloadWithinLimit(2 * 1024 * 1024) {
		t.Errorf("expected 2 MiB to be within limit")
	}

	// Exceeds 3 MiB (e.g. 3.5 MiB)
	if quota.IsPayloadWithinLimit(int64(3.5 * 1024 * 1024)) {
		t.Errorf("expected 3.5 MiB to exceed limit")
	}
}

func TestSnapshotQuota_EvictOnExceededTenantBytes(t *testing.T) {
	store := &quotaMockStore{
		fakeStore:      *newFakeStore(),
		tenantBytesVal: 600 * 1024 * 1024, // Exceeds 512 MiB limit
	}

	quota := NewSnapshotQuotaManager(store, 3*1024*1024, 64*1024*1024, 512*1024*1024, 3)

	err := quota.CheckAndEvict(context.Background(), "tenant-1", "repo-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store.evictedTenant != "tenant-1" || store.evictedBinding != "repo-1" {
		t.Errorf("expected eviction called for tenant-1 repo-1, got %s %s", store.evictedTenant, store.evictedBinding)
	}
}

func TestCachedViewReader_OverSizePayloadNotCached(t *testing.T) {
	store := newFakeStore()
	probeGW := newMockGateway()
	probeGW.statusRes = RawCodeIntelResult{HeadCommit: "head-1"}
	probe := NewHeadProbe(probeGW, 15*time.Second)

	// Make large payload: 4 MiB string
	largePayload := make([]byte, 4*1024*1024)
	underlying := &mockViewReaderSpy{
		res: ViewResult{
			Meta: domain.ResultMeta{Commit: "head-1", TotalCount: 1},
			Data: string(largePayload),
		},
	}

	// 3 MiB max payload
	quota := NewSnapshotQuotaManager(store, 3*1024*1024, 64*1024*1024, 512*1024*1024, 3)
	reader := NewCachedViewReader(underlying, store, probe, quota, 5*time.Second, 1*time.Second)
	target := AgentTarget{TenantID: "tenant-1", WorkspaceRoot: "/work/repo-1"}

	res, err := reader.Get(context.Background(), target, domain.ViewKindStructure, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Data == nil {
		t.Errorf("expected caller to still receive data even if payload is not cached")
	}

	// Check DB store: should NOT be stored
	if len(store.snapshots) != 0 {
		t.Errorf("expected 0 snapshots in store due to payload size exceeding max, got %d", len(store.snapshots))
	}
}
