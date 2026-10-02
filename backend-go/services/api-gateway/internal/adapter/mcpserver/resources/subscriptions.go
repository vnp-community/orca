package resources

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
)

// TaskEventSource reports task changes. Watch blocks until ctx ends and calls
// fn for every event of any tenant; the manager filters by tenant.
type TaskEventSource interface {
	Watch(ctx context.Context, fn func(tenantID, taskID string)) error
}

type subscription struct {
	p         mcpserver.Principal
	sessionID string
	ref       Ref
}

// subscriptions is a per-replica manager: MCP sessions live in this replica's
// memory, and every replica receives every event (JetStream ephemeral
// consumers), so no cross-replica coordination is needed. Notifications carry
// the URI only.
type subscriptions struct {
	src      TaskEventSource
	cfg      Config
	allowed  func(context.Context, *subscription) bool
	log      *slog.Logger
	mu       sync.Mutex
	notifier mcpserver.ResourceNotifier
	// byTask: tenant+"/"+taskID -> sessionID+"\x00"+rawURI -> subscription
	byTask    map[string]map[string]*subscription
	bySession map[string]map[string]struct{} // sessionID -> rawURIs
	timers    map[string]*time.Timer         // tenant+"/"+taskID+"\x00"+rawURI
	stopSrc   context.CancelFunc
	srcDone   chan struct{}
	closed    bool
}

func newSubscriptions(src TaskEventSource, cfg Config, allowed func(context.Context, *subscription) bool, log *slog.Logger) *subscriptions {
	return &subscriptions{src: src, cfg: cfg, allowed: allowed, log: log,
		byTask: map[string]map[string]*subscription{}, bySession: map[string]map[string]struct{}{}, timers: map[string]*time.Timer{}}
}

func (m *subscriptions) bind(n mcpserver.ResourceNotifier) {
	m.mu.Lock()
	m.notifier = n
	m.mu.Unlock()
}

func subKey(sessionID, uri string) string { return sessionID + "\x00" + uri }
func taskKey(tenant, task string) string  { return tenant + "/" + task }

func (m *subscriptions) add(s *subscription) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return mcpserver.ErrResourceNotFound
	}
	uris := m.bySession[s.sessionID]
	if _, dup := uris[s.ref.RawURI]; !dup && len(uris) >= m.cfg.MaxSubsPerSession {
		return mcpserver.ErrSubscriptionLimit
	}
	if uris == nil {
		uris = map[string]struct{}{}
		m.bySession[s.sessionID] = uris
	}
	uris[s.ref.RawURI] = struct{}{}
	tk := taskKey(s.p.TenantID, s.ref.ID)
	if m.byTask[tk] == nil {
		m.byTask[tk] = map[string]*subscription{}
	}
	m.byTask[tk][subKey(s.sessionID, s.ref.RawURI)] = s
	m.startSourceLocked()
	return nil
}

func (m *subscriptions) remove(sessionID, uri string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeLocked(sessionID, uri)
}

func (m *subscriptions) removeLocked(sessionID, uri string) {
	if uris := m.bySession[sessionID]; uris != nil {
		delete(uris, uri)
		if len(uris) == 0 {
			delete(m.bySession, sessionID)
		}
	}
	for tk, subs := range m.byTask {
		if _, ok := subs[subKey(sessionID, uri)]; !ok {
			continue
		}
		delete(subs, subKey(sessionID, uri))
		if len(subs) == 0 {
			delete(m.byTask, tk)
			for k, t := range m.timers { // no one left on this task: drop pending timers
				if len(k) > len(tk) && k[:len(tk)+1] == tk+"\x00" {
					t.Stop()
					delete(m.timers, k)
				}
			}
		}
	}
	if len(m.byTask) == 0 {
		m.stopSourceLocked()
	}
}

func (m *subscriptions) endSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for uri := range m.bySession[sessionID] {
		m.removeLocked(sessionID, uri)
	}
}

func (m *subscriptions) close() {
	m.mu.Lock()
	m.closed = true
	for _, t := range m.timers {
		t.Stop()
	}
	m.timers = map[string]*time.Timer{}
	m.byTask, m.bySession = map[string]map[string]*subscription{}, map[string]map[string]struct{}{}
	done := m.stopSourceLocked()
	m.mu.Unlock()
	if done != nil {
		<-done
	}
}

func (m *subscriptions) startSourceLocked() {
	if m.stopSrc != nil || m.src == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	m.stopSrc, m.srcDone = cancel, done
	go func() {
		defer close(done)
		if err := m.src.Watch(ctx, m.onTaskEvent); err != nil && ctx.Err() == nil {
			m.log.Warn("mcp resources: task event source ended", slog.Any("error", err))
		}
	}()
}

// stopSourceLocked cancels the source and returns its done channel.
func (m *subscriptions) stopSourceLocked() <-chan struct{} {
	if m.stopSrc == nil {
		return nil
	}
	m.stopSrc()
	done := m.srcDone
	m.stopSrc, m.srcDone = nil, nil
	return done
}

func (m *subscriptions) onTaskEvent(tenantID, taskID string) {
	tk := taskKey(tenantID, taskID)
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.byTask[tk] {
		tkey := tk + "\x00" + s.ref.RawURI
		if _, pending := m.timers[tkey]; pending {
			continue // already scheduled: coalesce
		}
		uri := s.ref.RawURI
		m.timers[tkey] = time.AfterFunc(m.cfg.Debounce, func() { m.fire(tk, tkey, uri) })
	}
}

// fire re-checks every subscriber of uri (token expiry, scope, tenant policy),
// closes the sessions that lost access (the SDK can only broadcast per URI),
// and sends one resources/updated for the rest.
func (m *subscriptions) fire(tk, tkey, uri string) {
	m.mu.Lock()
	delete(m.timers, tkey)
	var subs []*subscription
	for _, s := range m.byTask[tk] {
		if s.ref.RawURI == uri {
			subs = append(subs, s)
		}
	}
	n := m.notifier
	m.mu.Unlock()
	if n == nil || len(subs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	notify := false
	for _, s := range subs {
		if m.allowed(ctx, s) {
			notify = true
			continue
		}
		m.remove(s.sessionID, uri)
		n.CloseSession(s.sessionID)
	}
	if notify {
		n.ResourceUpdated(uri)
	}
}
