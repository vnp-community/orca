package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/auditclient"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

// memTx runs fn directly; commit/rollback semantics are asserted where it matters through rollbackTx.
type memTx struct{}

func (memTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// rollbackTx buffers audit entries in a pending slice and drops them when fn fails, like a real transaction.
type rollbackTx struct{ out *memAuditOutbox }

func (r rollbackTx) InTx(ctx context.Context, fn func(context.Context) error) error {
	if r.out.depth > 0 { // a nested InTx joins the outer transaction, like the real repositories
		r.out.depth++
		defer func() { r.out.depth-- }()
		return fn(ctx)
	}
	r.out.begin()
	r.out.depth = 1
	defer func() { r.out.depth = 0 }()
	if err := fn(ctx); err != nil {
		r.out.rollback()
		return err
	}
	r.out.commit()
	return nil
}

type memAuditOutbox struct {
	mu      sync.Mutex
	rows    []AuditOutboxRecord
	pending []AuditOutboxRecord
	failEnq error
	depth   int
}

func (o *memAuditOutbox) begin() { o.pending = nil }
func (o *memAuditOutbox) commit() {
	o.rows = append(o.rows, o.pending...)
	o.pending = nil
}
func (o *memAuditOutbox) rollback() { o.pending = nil }

func (o *memAuditOutbox) Enqueue(_ context.Context, rec AuditOutboxRecord) error {
	if o.failEnq != nil {
		return o.failEnq
	}
	o.pending = append(o.pending, rec)
	return nil
}
func (o *memAuditOutbox) ProcessDue(context.Context, time.Time, int, func(int) time.Duration, func(context.Context, AuditOutboxRecord) error) (int, int, error) {
	return 0, 0, nil
}
func (o *memAuditOutbox) PurgeDelivered(context.Context, time.Time, int) (int, error) { return 0, nil }
func (o *memAuditOutbox) CountPending(context.Context) (int, error)                   { return len(o.rows), nil }

type memSink struct {
	mu      sync.Mutex
	entries []auditclient.Entry
}

func (s *memSink) AppendDetailed(_ context.Context, e auditclient.Entry) {
	s.mu.Lock()
	s.entries = append(s.entries, e)
	s.mu.Unlock()
}

func adminCtx(tenantID string) context.Context {
	ctx := tenant.WithTenantID(context.Background(), tenantID)
	ctx = tenant.WithUserID(ctx, "a0000000-0000-4000-8000-000000000001")
	return tenant.WithRole(ctx, "admin")
}

type fakeRetentionStore struct {
	mu        sync.Mutex
	tenants   []string
	settings  map[string]domain.RetentionSettings
	expired   map[string][]string // tenant -> request ids older than the cutoff and finished
	erased    map[string]bool
	reporters map[string]string // request id -> reporter
	byTenant  map[string]string // request id -> tenant
	lastCut   map[string]time.Time
	claimed   map[string]bool
}

func newFakeRetention() *fakeRetentionStore {
	return &fakeRetentionStore{settings: map[string]domain.RetentionSettings{}, expired: map[string][]string{}, erased: map[string]bool{},
		reporters: map[string]string{}, byTenant: map[string]string{}, lastCut: map[string]time.Time{}, claimed: map[string]bool{}}
}

func (f *fakeRetentionStore) ListTenants(context.Context) ([]string, error) { return f.tenants, nil }

func (f *fakeRetentionStore) Settings(ctx context.Context) (domain.RetentionSettings, error) {
	t, _ := tenant.TenantID(ctx)
	if s, ok := f.settings[t]; ok {
		return s, nil
	}
	return domain.DefaultRetention, nil
}

func (f *fakeRetentionStore) AnonymizeExpired(ctx context.Context, cutoff time.Time, limit int, ps func(string) string, at time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, _ := tenant.TenantID(ctx)
	f.lastCut[t] = cutoff
	n := 0
	for _, id := range f.expired[t] {
		if n == limit {
			break
		}
		if f.erased[id] || f.claimed[id] {
			continue
		}
		f.erased[id] = true
		f.reporters[id] = ps(f.reporters[id])
		n++
	}
	return n, nil
}

func (f *fakeRetentionStore) Anonymize(ctx context.Context, id string, ps func(string) string, at time.Time, by string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.erased[id] {
		return false, nil
	}
	f.erased[id] = true
	f.reporters[id] = ps(f.reporters[id])
	return true, nil
}
