package resources

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
)

// fakeSource lets a test push task events and observe the Watch lifecycle.
type fakeSource struct {
	mu       sync.Mutex
	fn       func(tenantID, taskID string)
	started  atomic.Int32
	running  atomic.Int32
	attached chan struct{}
}

func newFakeSource() *fakeSource { return &fakeSource{attached: make(chan struct{}, 8)} }

func (s *fakeSource) Watch(ctx context.Context, fn func(tenantID, taskID string)) error {
	s.started.Add(1)
	s.running.Add(1)
	defer s.running.Add(-1)
	s.mu.Lock()
	s.fn = fn
	s.mu.Unlock()
	s.attached <- struct{}{}
	<-ctx.Done()
	return nil
}

func (s *fakeSource) emit(tenant, task string) {
	s.mu.Lock()
	fn := s.fn
	s.mu.Unlock()
	fn(tenant, task)
}

type fakeNotifier struct {
	mu      sync.Mutex
	updated []string
	closed  []string
}

func (n *fakeNotifier) ResourceUpdated(uri string) {
	n.mu.Lock()
	n.updated = append(n.updated, uri)
	n.mu.Unlock()
}
func (n *fakeNotifier) CloseSession(id string) {
	n.mu.Lock()
	n.closed = append(n.closed, id)
	n.mu.Unlock()
}
func (n *fakeNotifier) snapshot() (u, c []string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.updated...), append([]string(nil), n.closed...)
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func subProvider(t *testing.T, gate mcpserver.PolicyGate, cfg Config) (*Provider, *fakeSource, *fakeNotifier) {
	t.Helper()
	src, n := newFakeSource(), &fakeNotifier{}
	if cfg.Debounce == 0 {
		cfg.Debounce = 20 * time.Millisecond
	}
	p := NewProvider(scripted(), gate, src, cfg, quietLog())
	p.Bind(n)
	t.Cleanup(p.Close)
	return p, src, n
}

func TestSubscribe_LifecycleAndNotification(t *testing.T) {
	p, src, n := subProvider(t, &mcpservertest.FakeGate{}, Config{})
	ctx := context.Background()
	taskURI := "orca://task/" + idA
	if !p.Subscribable() {
		t.Fatal("source configured: subscribe must be offered")
	}
	if err := p.Subscribe(ctx, alice, "s1", taskURI); err != nil {
		t.Fatal(err)
	}
	<-src.attached

	// Another tenant's event for the same task id is ignored.
	src.emit("t2", idA)
	// Matching event: a burst is coalesced into one URI-only notification.
	for i := 0; i < 5; i++ {
		src.emit("t1", idA)
	}
	eventually(t, "notification", func() bool { u, _ := n.snapshot(); return len(u) >= 1 })
	time.Sleep(80 * time.Millisecond)
	if u, _ := n.snapshot(); len(u) != 1 || u[0] != taskURI {
		t.Fatalf("updated = %v, want exactly [%s]", u, taskURI)
	}
	// Unrelated task: nothing.
	src.emit("t1", idB)
	time.Sleep(60 * time.Millisecond)
	if u, _ := n.snapshot(); len(u) != 1 {
		t.Fatalf("unrelated event notified: %v", u)
	}

	// Unsubscribe stops notifications and releases the source.
	p.Unsubscribe("s1", taskURI)
	eventually(t, "source stopped", func() bool { return src.running.Load() == 0 })
	src.emit("t1", idA)
	time.Sleep(60 * time.Millisecond)
	if u, _ := n.snapshot(); len(u) != 1 {
		t.Fatalf("notified after unsubscribe: %v", u)
	}
}

func TestSubscribe_AccessAndSupport(t *testing.T) {
	ctx := context.Background()
	p, _, _ := subProvider(t, &mcpservertest.FakeGate{}, Config{})
	// Subscribing authorizes like a read: missing resource, malformed URI and
	// denied policy are all the same not-found.
	d := scripted()
	d.errs["task.get"] = errors.New("boom")
	missing := NewProvider(d, &mcpservertest.FakeGate{}, newFakeSource(), Config{}, quietLog())
	if err := missing.Subscribe(ctx, alice, "s", "orca://task/"+idA); err == nil || errors.Is(err, mcpserver.ErrSubscriptionUnsupported) {
		t.Fatalf("unreadable task must not be subscribable: %v", err)
	}
	if err := p.Subscribe(ctx, alice, "s", "orca://task/not-a-uuid"); !errors.Is(err, mcpserver.ErrResourceNotFound) {
		t.Fatalf("%v", err)
	}
	deny := NewProvider(scripted(), &mcpservertest.FakeGate{Default: mcpserver.GateDecision{Outcome: "deny"}}, newFakeSource(), Config{}, quietLog())
	if err := deny.Subscribe(ctx, alice, "s", "orca://task/"+idA); !errors.Is(err, mcpserver.ErrResourceNotFound) {
		t.Fatalf("%v", err)
	}
	// Readable but not subscribable (v1: task only).
	for _, u := range []string{"orca://projects", "orca://project/" + idA, "orca://worktree/" + idA + "/status", "orca://review/github/o%2Fr/1"} {
		if err := p.Subscribe(ctx, alice, "s", u); !errors.Is(err, mcpserver.ErrSubscriptionUnsupported) {
			t.Errorf("%s: %v", u, err)
		}
	}
	// No event source: capability is not offered at all.
	none := NewProvider(scripted(), nil, nil, Config{}, quietLog())
	if none.Subscribable() {
		t.Fatal("must not declare subscribe without an event source")
	}
}

func TestSubscribe_QuotaAndEndSessionNoLeak(t *testing.T) {
	before := runtime.NumGoroutine()
	p, src, n := subProvider(t, &mcpservertest.FakeGate{}, Config{MaxSubsPerSession: 2})
	ctx := context.Background()
	ids := []string{idA, idB, "33333333-3333-4333-8333-333333333333"}
	for i, id := range ids {
		err := p.Subscribe(ctx, alice, "s1", "orca://task/"+id)
		if i < 2 && err != nil {
			t.Fatal(err)
		}
		if i == 2 && !errors.Is(err, mcpserver.ErrSubscriptionLimit) {
			t.Fatalf("quota: %v", err)
		}
	}
	// Re-subscribing to an existing URI is idempotent, not counted twice.
	if err := p.Subscribe(ctx, alice, "s1", "orca://task/"+idA); err != nil {
		t.Fatalf("idempotent: %v", err)
	}
	<-src.attached
	// A second session shares the one event source.
	if err := p.Subscribe(ctx, alice, "s2", "orca://task/"+idA); err != nil {
		t.Fatal(err)
	}
	if src.started.Load() != 1 {
		t.Fatalf("source started %d times, want 1 shared", src.started.Load())
	}
	src.emit("t1", idA)

	// Session end (client DELETE / timeout / drop) cleans everything up.
	p.EndSession("s1")
	p.EndSession("s2")
	p.EndSession("never-existed")
	eventually(t, "source stopped", func() bool { return src.running.Load() == 0 })
	time.Sleep(60 * time.Millisecond) // a pending debounce timer must have been cancelled
	p.subs.mu.Lock()
	leftover := len(p.subs.byTask) + len(p.subs.bySession) + len(p.subs.timers)
	p.subs.mu.Unlock()
	if leftover != 0 {
		t.Fatalf("manager state leaked: %d entries", leftover)
	}
	_ = n
	assertNoGoroutineGrowth(t, before)
}

func TestFire_RevokedSubscriberIsClosedAndSkipped(t *testing.T) {
	gate := mcpservertest.FakeViewGate{FakeGate: &mcpservertest.FakeGate{}}
	var denied atomic.Bool
	gate.Effective = func(m mcpserver.ToolMeta) mcpserver.EffectiveDecision {
		if denied.Load() {
			return mcpserver.EffectiveDecision{Decision: mcpserver.OutcomeDeny, Source: "tenant_policy"}
		}
		return mcpserver.EffectiveDecision{Decision: mcpserver.OutcomeAllow}
	}
	p, src, n := subProvider(t, gate, Config{})
	uri := "orca://task/" + idA
	bob := alice
	bob.UserID = "bob"
	expiring := alice
	expiring.UserID = "carol"
	expiring.ExpiresAt = time.Now().Add(40 * time.Millisecond)
	for sess, pr := range map[string]mcpserver.Principal{"s-bob": bob, "s-carol": expiring} {
		if err := p.Subscribe(context.Background(), pr, sess, uri); err != nil {
			t.Fatal(err)
		}
	}
	<-src.attached
	time.Sleep(60 * time.Millisecond) // carol's token expires
	src.emit("t1", idA)
	eventually(t, "notification", func() bool { u, _ := n.snapshot(); return len(u) == 1 })
	if _, c := n.snapshot(); len(c) != 1 || c[0] != "s-carol" {
		t.Fatalf("only the expired token's session may be closed: %v", c)
	}
	// Policy flips to deny: the remaining session is closed and nothing is sent.
	denied.Store(true)
	src.emit("t1", idA)
	eventually(t, "close", func() bool { _, c := n.snapshot(); return len(c) == 2 })
	time.Sleep(50 * time.Millisecond)
	if u, c := n.snapshot(); len(u) != 1 || !strings.Contains(strings.Join(c, ","), "s-bob") {
		t.Fatalf("updated=%v closed=%v", u, c)
	}
}

func assertNoGoroutineGrowth(t *testing.T, before int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= before+1 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	buf := make([]byte, 1<<16)
	t.Fatalf("goroutines grew from %d to %d:\n%s", before, runtime.NumGoroutine(), buf[:runtime.Stack(buf, true)])
}
