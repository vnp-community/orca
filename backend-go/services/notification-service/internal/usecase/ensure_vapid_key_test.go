package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// memVapidStore mimics the unique-active-key-per-tenant constraint.
type memVapidStore struct {
	mu      sync.Mutex
	rows    map[string]domain.VapidKeyMetadata
	inserts int
}

func newMemVapidStore() *memVapidStore {
	return &memVapidStore{rows: map[string]domain.VapidKeyMetadata{}}
}

func (s *memVapidStore) GetPublicKey(_ context.Context, tenantID string) (domain.VapidKeyMetadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if k, ok := s.rows[tenantID]; ok {
		return k, nil
	}
	return domain.VapidKeyMetadata{}, domain.ErrNoActiveVapidKey
}

func (s *memVapidStore) InsertActiveIfAbsent(_ context.Context, key domain.VapidKeyMetadata) (domain.VapidKeyMetadata, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if k, ok := s.rows[key.TenantID]; ok {
		return k, false, nil
	}
	s.rows[key.TenantID] = key
	s.inserts++
	return key, true, nil
}

type fakeProvisioner struct {
	calls atomic.Int32
	delay time.Duration
	err   error
	pub   string
	// beforeReturn runs while the broker call is "in flight".
	beforeReturn func()
}

func (f *fakeProvisioner) EnsureVapidSigningKey(_ context.Context, _ string) (string, error) {
	f.calls.Add(1)
	time.Sleep(f.delay)
	if f.beforeReturn != nil {
		f.beforeReturn()
	}
	return f.pub, f.err
}

type outcomeCounter struct {
	mu sync.Mutex
	m  map[string]int
}

func (o *outcomeCounter) ObserveVapidProvision(outcome string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.m == nil {
		o.m = map[string]int{}
	}
	o.m[outcome]++
}

func (o *outcomeCounter) get(k string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.m[k]
}

func tctx(id string) context.Context { return tenant.WithTenantID(context.Background(), id) }

func TestGetVapidPublicKey_ConcurrentFirstUseProvisionsOnce(t *testing.T) {
	store := newMemVapidStore()
	broker := &fakeProvisioner{pub: "PUB", delay: 50 * time.Millisecond}
	obs := &outcomeCounter{}
	uc := NewGetVapidPublicKey(store).WithEnsurer(NewEnsureVapidKey(store, broker, obs))

	var wg sync.WaitGroup
	results := make([]string, 20)
	errs := make([]error, 20)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = uc.Execute(tctx("tenant-a"))
		}()
	}
	wg.Wait()
	for i := range results {
		if errs[i] != nil || results[i] != "PUB" {
			t.Fatalf("call %d: %q %v", i, results[i], errs[i])
		}
	}
	if got := broker.calls.Load(); got != 1 {
		t.Fatalf("broker calls = %d, want 1", got)
	}
	if store.inserts != 1 {
		t.Fatalf("inserts = %d, want 1", store.inserts)
	}
	if obs.get("created") != 1 {
		t.Fatalf("created outcomes = %d", obs.get("created"))
	}
	// Row now exists: later calls never reach the broker.
	if _, err := uc.Execute(tctx("tenant-a")); err != nil || broker.calls.Load() != 1 {
		t.Fatalf("repeat call hit broker: %v", err)
	}
}

func TestEnsureVapidKey_OtherReplicaInsertedFirst(t *testing.T) {
	store := newMemVapidStore()
	// Simulates replica B inserting while this replica waits on the broker.
	broker := &fakeProvisioner{pub: "PUB-A", beforeReturn: func() {
		_, _, _ = store.InsertActiveIfAbsent(context.Background(), domain.VapidKeyMetadata{TenantID: "tenant-a", PublicKey: "PUB-A", KeyID: "other"})
	}}
	obs := &outcomeCounter{}
	key, err := NewEnsureVapidKey(store, broker, obs).Execute(context.Background(), "tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	if key.KeyID != "other" || store.inserts != 1 {
		t.Fatalf("must converge on the existing row: %+v inserts=%d", key, store.inserts)
	}
	if obs.get("existing") != 1 || obs.get("created") != 0 {
		t.Fatalf("outcomes: %v", obs.m)
	}
}

func TestEnsureVapidKey_PreInsertedRowSkipsBroker(t *testing.T) {
	store := newMemVapidStore()
	store.rows["tenant-a"] = domain.VapidKeyMetadata{TenantID: "tenant-a", PublicKey: "PUB"}
	broker := &fakeProvisioner{pub: "X"}
	if _, err := NewEnsureVapidKey(store, broker, nil).Execute(context.Background(), "tenant-a"); err != nil {
		t.Fatal(err)
	}
	if broker.calls.Load() != 0 {
		t.Fatal("broker must not be called")
	}
}

func TestGetVapidPublicKey_ForbiddenIsNegativelyCached(t *testing.T) {
	store := newMemVapidStore()
	broker := &fakeProvisioner{err: fmt.Errorf("%w: policy", ErrVapidProvisionForbidden)}
	obs := &outcomeCounter{}
	ensure := NewEnsureVapidKey(store, broker, obs)
	now := time.Unix(1000, 0)
	ensure.now = func() time.Time { return now }
	uc := NewGetVapidPublicKey(store).WithEnsurer(ensure)

	for i := 0; i < 5; i++ {
		_, err := uc.Execute(tctx("tenant-a"))
		var ae *apperrors.AppError
		if !errors.As(err, &ae) || ae.Code != "NOTIFICATION_NO_VAPID_KEY" || ae.Kind != apperrors.KindNotFound {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if broker.calls.Load() != 1 {
		t.Fatalf("broker calls = %d, want 1 within the negative TTL", broker.calls.Load())
	}
	if obs.get("forbidden") != 1 {
		t.Fatalf("forbidden outcomes = %d", obs.get("forbidden"))
	}

	now = now.Add(31 * time.Second)
	_, _ = uc.Execute(tctx("tenant-a"))
	if broker.calls.Load() != 2 {
		t.Fatalf("broker calls after TTL = %d, want 2", broker.calls.Load())
	}
}

func TestGetVapidPublicKey_BrokerErrorKeepsFetchFailedCode(t *testing.T) {
	store := newMemVapidStore()
	broker := &fakeProvisioner{err: errors.New("broker down")}
	obs := &outcomeCounter{}
	uc := NewGetVapidPublicKey(store).WithEnsurer(NewEnsureVapidKey(store, broker, obs))
	_, err := uc.Execute(tctx("tenant-a"))
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "NOTIFICATION_VAPID_KEY_FETCH_FAILED" {
		t.Fatalf("got %v", err)
	}
	if obs.get("error") != 1 {
		t.Fatalf("outcomes: %v", obs.m)
	}
}

func TestEnsureVapidKey_WaiterContextCancelDoesNotHang(t *testing.T) {
	store := newMemVapidStore()
	release := make(chan struct{})
	broker := &fakeProvisioner{pub: "PUB", beforeReturn: func() { <-release }}
	ensure := NewEnsureVapidKey(store, broker, nil)
	go func() { _, _ = ensure.Execute(context.Background(), "tenant-a") }()
	for broker.calls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := ensure.Execute(ctx, "tenant-a"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v", err)
	}
	close(release)
}
