package usecase

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/domain"
)

type mockViewReaderSpy struct {
	calls int32
	delay time.Duration
	res   ViewResult
	err   error
}

func (m *mockViewReaderSpy) Get(ctx context.Context, target AgentTarget, view domain.ViewKind, params any) (ViewResult, error) {
	atomic.AddInt32(&m.calls, 1)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return ViewResult{}, ctx.Err()
		}
	}
	return m.res, m.err
}

type fakeStore struct {
	mu        sync.RWMutex
	snapshots map[domain.SnapshotKey]domain.Snapshot
}

func newFakeStore() *fakeStore {
	return &fakeStore{snapshots: make(map[domain.SnapshotKey]domain.Snapshot)}
}

func (f *fakeStore) Get(ctx context.Context, key domain.SnapshotKey) (domain.Snapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	s, ok := f.snapshots[key]
	if !ok {
		return domain.Snapshot{}, apperrors.New(apperrors.KindNotFound, "CODEINTEL_NOT_FOUND", "not found", nil)
	}
	return s, nil
}

func (f *fakeStore) GetLatest(ctx context.Context, tenant, binding string, view domain.ViewKind, paramsHash string) (domain.Snapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var latest domain.Snapshot
	var found bool
	for k, s := range f.snapshots {
		if k.Tenant == tenant && k.Binding == binding && k.View == view && k.ParamsHash == paramsHash {
			if !found || s.CreatedAt.After(latest.CreatedAt) {
				latest = s
				found = true
			}
		}
	}
	if !found {
		return domain.Snapshot{}, apperrors.New(apperrors.KindNotFound, "CODEINTEL_NOT_FOUND", "not found", nil)
	}
	return latest, nil
}

func (f *fakeStore) Put(ctx context.Context, s domain.Snapshot) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snapshots[s.Key] = s
	return nil
}

func (f *fakeStore) DeleteByBinding(ctx context.Context, tenant, binding string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for k := range f.snapshots {
		if k.Tenant == tenant && k.Binding == binding {
			delete(f.snapshots, k)
		}
	}
	return nil
}

func (f *fakeStore) DeleteExpired(ctx context.Context, limit int) (int, error) {
	return 0, nil
}

func (f *fakeStore) TotalBytes(ctx context.Context) (int64, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return int64(len(f.snapshots) * 1024), nil
}

func (f *fakeStore) TenantBytes(ctx context.Context, tenant string) (int64, error) {
	return 0, nil
}

func (f *fakeStore) EvictOldest(ctx context.Context, tenant, binding string, keepCommits int, targetBytes int64) error {
	return nil
}

func TestCachedViewReader_HitAndMiss(t *testing.T) {
	store := newFakeStore()
	probeGW := newMockGateway()
	probeGW.statusRes = RawCodeIntelResult{HeadCommit: "head-1"}
	probe := NewHeadProbe(probeGW, 15*time.Second)

	underlying := &mockViewReaderSpy{
		res: ViewResult{
			Meta: domain.ResultMeta{Commit: "head-1", TotalCount: 42},
			Data: map[string]string{"foo": "bar"},
		},
	}

	reader := NewCachedViewReader(underlying, store, probe, nil, 5*time.Second, 1*time.Second)
	target := AgentTarget{TenantID: "tenant-1", WorkspaceRoot: "/work/repo-1"}

	// 1. First call -> Cache MISS -> calls underlying
	res1, err := reader.Get(context.Background(), target, domain.ViewKindStructure, map[string]any{"depth": 1})
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	if res1.Meta.Cached {
		t.Errorf("expected first call to be cache miss")
	}
	if atomic.LoadInt32(&underlying.calls) != 1 {
		t.Errorf("expected 1 underlying call, got %d", underlying.calls)
	}

	// 2. Second call -> Cache HIT -> does NOT call underlying
	res2, err := reader.Get(context.Background(), target, domain.ViewKindStructure, map[string]any{"depth": 1})
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if !res2.Meta.Cached {
		t.Errorf("expected second call to be cached")
	}
	if atomic.LoadInt32(&underlying.calls) != 1 {
		t.Errorf("expected underlying calls to stay 1, got %d", underlying.calls)
	}
}

func TestCachedViewReader_IfNoneMatch_NotModified(t *testing.T) {
	store := newFakeStore()
	probeGW := newMockGateway()
	probeGW.statusRes = RawCodeIntelResult{HeadCommit: "head-1"}
	probe := NewHeadProbe(probeGW, 15*time.Second)

	underlying := &mockViewReaderSpy{
		res: ViewResult{
			Meta: domain.ResultMeta{Commit: "head-1", TotalCount: 10},
			Data: map[string]string{"k": "v"},
		},
	}

	reader := NewCachedViewReader(underlying, store, probe, nil, 5*time.Second, 1*time.Second)
	target := AgentTarget{TenantID: "tenant-1", WorkspaceRoot: "/work/repo-1"}

	res1, err := reader.Get(context.Background(), target, domain.ViewKindStructure, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	etag := res1.Meta.ETag
	if etag == "" {
		t.Fatalf("expected non-empty ETag")
	}

	// Pass matching IfNoneMatch
	res2, err := reader.Get(context.Background(), target, domain.ViewKindStructure, map[string]any{"if_none_match": etag})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res2.Meta.NotModified {
		t.Errorf("expected NotModified true")
	}
	if res2.Data != nil {
		t.Errorf("expected nil data for NotModified response")
	}
}

func TestCachedViewReader_SingleflightConcurrency(t *testing.T) {
	store := newFakeStore()
	probeGW := newMockGateway()
	probeGW.statusRes = RawCodeIntelResult{HeadCommit: "head-1"}
	probe := NewHeadProbe(probeGW, 15*time.Second)

	underlying := &mockViewReaderSpy{
		delay: 30 * time.Millisecond,
		res: ViewResult{
			Meta: domain.ResultMeta{Commit: "head-1", TotalCount: 100},
			Data: "graph-payload",
		},
	}

	reader := NewCachedViewReader(underlying, store, probe, nil, 5*time.Second, 2*time.Second)
	target := AgentTarget{TenantID: "tenant-1", WorkspaceRoot: "/work/repo-1"}

	var wg sync.WaitGroup
	concurrent := 50
	wg.Add(concurrent)

	for i := 0; i < concurrent; i++ {
		go func() {
			defer wg.Done()
			res, err := reader.Get(context.Background(), target, domain.ViewKindStructure, map[string]any{"con": true})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if res.Data == nil {
				t.Errorf("expected data")
			}
		}()
	}

	wg.Wait()

	calls := atomic.LoadInt32(&underlying.calls)
	if calls != 1 {
		t.Fatalf("expected exactly 1 call across 50 concurrent requests, got %d", calls)
	}
}

func TestCachedViewReader_GracefulDegradation_Offline(t *testing.T) {
	store := newFakeStore()
	probeGW := newMockGateway()
	probeGW.statusRes = RawCodeIntelResult{HeadCommit: "head-old"}
	probe := NewHeadProbe(probeGW, 15*time.Second)

	underlying := &mockViewReaderSpy{
		res: ViewResult{
			Meta: domain.ResultMeta{Commit: "head-old", TotalCount: 5},
			Data: "cached-data",
		},
	}

	reader := NewCachedViewReader(underlying, store, probe, nil, 5*time.Second, 1*time.Second)
	target := AgentTarget{TenantID: "tenant-1", WorkspaceRoot: "/work/repo-1"}

	// Populate snapshot
	_, err := reader.Get(context.Background(), target, domain.ViewKindStructure, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Now agent goes offline
	probeGW.statusErr = apperrors.New(apperrors.KindUnavailable, "CODEINTEL_DEV_SERVER_OFFLINE", "dev server disconnected", nil)
	probe.Invalidate("tenant-1", "/work/repo-1")
	underlying.err = apperrors.New(apperrors.KindUnavailable, "CODEINTEL_DEV_SERVER_OFFLINE", "dev server disconnected", nil)

	res, err := reader.Get(context.Background(), target, domain.ViewKindStructure, map[string]any{})
	if err != nil {
		t.Fatalf("expected graceful degradation with stale snapshot, got error: %v", err)
	}
	if !res.Meta.Stale || !res.Meta.Cached {
		t.Errorf("expected stale cached result, got stale=%v, cached=%v", res.Meta.Stale, res.Meta.Cached)
	}
	if res.Data != "cached-data" {
		t.Errorf("expected 'cached-data', got %v", res.Data)
	}
}

func TestCachedViewReader_BusinessErrorNotMasked(t *testing.T) {
	store := newFakeStore()
	probeGW := newMockGateway()
	probeGW.statusRes = RawCodeIntelResult{HeadCommit: "head-1"}
	probe := NewHeadProbe(probeGW, 15*time.Second)

	underlying := &mockViewReaderSpy{
		err: apperrors.New(apperrors.KindNotFound, "CODEINTEL_INDEX_MISSING", "index not found", nil),
	}

	reader := NewCachedViewReader(underlying, store, probe, nil, 5*time.Second, 1*time.Second)
	target := AgentTarget{TenantID: "tenant-1", WorkspaceRoot: "/work/repo-1"}

	_, err := reader.Get(context.Background(), target, domain.ViewKindStructure, map[string]any{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "CODEINTEL_INDEX_MISSING" {
		t.Errorf("expected CODEINTEL_INDEX_MISSING, got %v", err)
	}
}

func TestCachedViewReader_StatusAndSymbolBypassDB(t *testing.T) {
	store := newFakeStore()
	probeGW := newMockGateway()
	probe := NewHeadProbe(probeGW, 15*time.Second)

	underlying := &mockViewReaderSpy{
		res: ViewResult{Meta: domain.ResultMeta{TotalCount: 1}, Data: "status-data"},
	}

	reader := NewCachedViewReader(underlying, store, probe, nil, 5*time.Second, 1*time.Second)
	target := AgentTarget{TenantID: "tenant-1", WorkspaceRoot: "/work/repo-1"}

	// Status call
	_, _ = reader.Get(context.Background(), target, domain.ViewKindStatus, nil)

	// Symbol call
	underlying.res = ViewResult{Meta: domain.ResultMeta{TotalCount: 1}, Data: "symbol-data"}
	_, _ = reader.Get(context.Background(), target, domain.ViewKindSymbol, map[string]any{"symbol": "Foo"})

	// Check DB has no snapshots
	if len(store.snapshots) != 0 {
		t.Errorf("expected 0 snapshots in DB store, got %d", len(store.snapshots))
	}
}
