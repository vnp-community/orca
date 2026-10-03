package tools

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

const (
	kindTerminal = "terminal"
	kindAgent    = "agent"

	// Entries whose process already exited are kept for terminal_read; beyond
	// this many per session the oldest is dropped.
	maxExitedKept = 16
	maxClosedIDs  = 1024
)

// managedPty is one PTY (plain terminal or agent) an MCP session started.
type managedPty struct {
	kind      string
	ptyID     string
	sessionID string // agent session id (agents only)
	ring      *ptyOutputRing
	created   time.Time
	lastUse   atomic.Int64 // unix nanos of the last tool call touching it
	stopping  atomic.Bool
	idleStop  atomic.Bool
}

func (p *managedPty) touch(now time.Time) { p.lastUse.Store(now.UnixNano()) }

// ptyState is the PTY-owning part of a per-MCP-session ToolSession.
type ptyState struct {
	id, tenantID, userID, clientName string
	identity                         wscompat.Identity
	streams                          *wscompat.ToolStreamScope
	limiter                          *tokenBucket

	mu       sync.Mutex
	ptys     map[string]*managedPty // by ptyID
	agents   map[string]*managedPty // by agent session id
	pending  map[string]int         // reserved, not yet registered, by kind
	closed   bool
	closeOne sync.Once
	pumps    sync.WaitGroup
}

// ToolSessions is the per-MCP-session registry of ToolSessions (BE-MCP-SOL-009):
// each owns the PTY streams its tools started and stops them when the MCP
// session closes, expires or is kill-switched.
type ToolSessions struct {
	parent *ToolSession
	cfg    PtyToolsConfig
	disp   Dispatcher
	log    *slog.Logger
	now    func() time.Time
	guards func() Guards

	mu       sync.Mutex
	byID     map[string]*ToolSession
	closedID []string
	closedAt map[string]struct{}
	janitor  sync.Once
	stop     chan struct{}
	wg       sync.WaitGroup
	quota    quotaCache
}

func newToolSessions(parent *ToolSession, d Dispatcher, cfg PtyToolsConfig, log *slog.Logger, now func() time.Time, guards func() Guards) *ToolSessions {
	return &ToolSessions{parent: parent, cfg: cfg.withDefaults(), disp: d, log: log, now: now, guards: guards,
		byID: map[string]*ToolSession{}, closedAt: map[string]struct{}{}, stop: make(chan struct{})}
}

// get returns the live ToolSession of an MCP session, creating it on first
// use. A session bound to another (tenant, user), or already closed, is refused.
func (m *ToolSessions) get(ctx context.Context, p mcpserver.Principal, sessionID, clientName string) (*ToolSession, error) {
	if sessionID == "" {
		return nil, &ToolError{"MCP_SESSION_REQUIRED", "terminal and agent tools need an MCP session"}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, closed := m.closedAt[sessionID]; closed {
		return nil, &ToolError{"MCP_SESSION_CLOSED", "the MCP session was closed"}
	}
	if ts, ok := m.byID[sessionID]; ok {
		if ts.pty.tenantID != p.TenantID || ts.pty.userID != p.UserID {
			return nil, &ToolError{"MCP_NOT_FOUND", "not found or not permitted"}
		}
		return ts, nil
	}
	sctx, cancel := context.WithCancel(m.parent.ctx)
	ts := &ToolSession{ctx: sctx, cancel: cancel, pty: &ptyState{
		id: sessionID, tenantID: p.TenantID, userID: p.UserID, clientName: clientName,
		identity: wscompat.Identity{TenantID: p.TenantID, UserID: p.UserID, Role: p.Role},
		streams:  wscompat.NewToolStreamScope(), limiter: newTokenBucket(m.cfg.ReadRate, m.cfg.ReadBurst, m.now),
		ptys: map[string]*managedPty{}, agents: map[string]*managedPty{}, pending: map[string]int{},
	}}
	m.byID[sessionID] = ts
	m.janitor.Do(func() {
		m.wg.Add(1)
		go m.runJanitor()
	})
	if g := m.guards(); g.WatchKill != nil {
		m.watchKill(ctx, g, p, ts)
	}
	return ts, nil
}

// watchKill closes the session's PTYs when its principal is kill-switched even
// while no tool call is running.
func (m *ToolSessions) watchKill(callCtx context.Context, g Guards, p mcpserver.Principal, ts *ToolSession) {
	base, cancelBase := context.WithCancel(context.WithoutCancel(callCtx)) // keeps the session id values WatchKill reads
	stopBase := context.AfterFunc(ts.ctx, cancelBase)
	kctx, kcancel := g.WatchKill(base, p)
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer kcancel()
		defer stopBase()
		select {
		case <-kctx.Done():
			if ts.ctx.Err() == nil {
				m.closeSession(ts, "kill_switch")
			}
		case <-ts.ctx.Done():
		case <-m.stop:
		}
	}()
}

// CloseSession stops everything the MCP session (row id) created. Idempotent;
// unknown ids are remembered so a late tool call cannot start new PTYs.
func (m *ToolSessions) CloseSession(sessionID, reason string) {
	m.mu.Lock()
	ts := m.byID[sessionID]
	m.rememberClosedLocked(sessionID)
	m.mu.Unlock()
	if ts != nil {
		m.closeSession(ts, reason)
	}
}

func (m *ToolSessions) rememberClosedLocked(id string) {
	if _, ok := m.closedAt[id]; ok {
		return
	}
	m.closedAt[id] = struct{}{}
	m.closedID = append(m.closedID, id)
	if len(m.closedID) > maxClosedIDs {
		delete(m.closedAt, m.closedID[0])
		m.closedID = m.closedID[1:]
	}
}

func (m *ToolSessions) closeSession(ts *ToolSession, reason string) {
	st := ts.pty
	st.closeOne.Do(func() {
		st.mu.Lock()
		st.closed = true
		all := make([]*managedPty, 0, len(st.ptys))
		for _, p := range st.ptys {
			all = append(all, p)
		}
		st.mu.Unlock()
		m.mu.Lock()
		m.rememberClosedLocked(st.id)
		delete(m.byID, st.id)
		m.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), m.cfg.CloseTimeout)
		defer cancel()
		var wg sync.WaitGroup
		for _, p := range all {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = m.stopPty(ctx, ts, p, closeReasonSessionClosed)
			}()
		}
		wg.Wait()
		st.streams.Close()
		ts.cancel()
		done := make(chan struct{})
		go func() { st.pumps.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			m.log.Warn("mcp pty pumps did not end after session close", slog.String("session", st.id))
		}
		m.log.Info("mcp session ptys stopped", slog.String("session", st.id), slog.String("reason", reason), slog.Int("count", len(all)))
	})
}

// Close reasons sent on terminal.close; infra-fleet records them on the
// orca.infrafleet.terminal.closed event (only "idle" notifies the owner).
const (
	closeReasonUser          = "user"
	closeReasonIdle          = "idle"
	closeReasonSessionClosed = "session_closed"
)

// stopPty ends one PTY. Agents get a polite agent.stop and, after the grace
// period agent.kill; terminals are closed (KillTerminalSession) with reason.
// Idempotent and best effort; the returned error is the terminal.close failure.
func (m *ToolSessions) stopPty(ctx context.Context, ts *ToolSession, p *managedPty, reason string) error {
	if !p.stopping.CompareAndSwap(false, true) {
		return nil
	}
	var closeErr error
	sctx := ts.pty.streams.Context(ctx)
	id := ts.pty.identity
	if p.kind == kindAgent {
		_, err := m.disp.Dispatch(sctx, id, "agent.stop", mustArgs(map[string]any{"sessionId": p.sessionID}))
		if err != nil {
			m.log.Warn("mcp agent.stop failed", slog.Any("error", err))
		}
		if !waitExited(ctx, p.ring, m.cfg.AgentGrace) {
			if _, err := m.disp.Dispatch(sctx, id, "agent.kill", mustArgs(map[string]any{"sessionId": p.sessionID, "signal": "SIGKILL"})); err != nil {
				m.log.Warn("mcp agent.kill failed", slog.Any("error", err))
			}
		}
	} else if _, err := m.disp.Dispatch(sctx, id, "terminal.close", mustArgs(map[string]any{"terminal": p.ptyID, "reason": reason})); err != nil {
		closeErr = err
		m.log.Warn("mcp terminal.close failed", slog.Any("error", err))
	}
	ts.pty.streams.Detach(p.ptyID)
	ts.pty.forget(p)
	return closeErr
}

func (st *ptyState) forget(p *managedPty) {
	st.mu.Lock()
	defer st.mu.Unlock()
	delete(st.ptys, p.ptyID)
	if p.sessionID != "" {
		delete(st.agents, p.sessionID)
	}
}

func waitExited(ctx context.Context, r *ptyOutputRing, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for {
		if exited, _, detached, _ := r.State(); exited || detached {
			return true
		}
		left := time.Until(deadline)
		if left <= 0 {
			return false
		}
		r.Wait(ctx.Done(), r.Tail(), min(left, 100*time.Millisecond))
		if ctx.Err() != nil {
			return false
		}
	}
}

func mustArgs(v any) []json.RawMessage {
	b, _ := json.Marshal(v)
	return []json.RawMessage{b}
}

// runJanitor stops PTYs idle for longer than IdleTimeout.
func (m *ToolSessions) runJanitor() {
	defer m.wg.Done()
	t := time.NewTicker(m.cfg.JanitorEvery)
	defer t.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-t.C:
			m.sweepIdle()
		}
	}
}

func (m *ToolSessions) sweepIdle() {
	m.mu.Lock()
	sessions := make([]*ToolSession, 0, len(m.byID))
	for _, ts := range m.byID {
		sessions = append(sessions, ts)
	}
	m.mu.Unlock()
	cutoff := m.now().Add(-m.cfg.IdleTimeout).UnixNano()
	for _, ts := range sessions {
		ts.pty.mu.Lock()
		var idle []*managedPty
		for _, p := range ts.pty.ptys {
			if p.lastUse.Load() < cutoff && p.ring.LastIO().UnixNano() < cutoff && !p.stopping.Load() {
				idle = append(idle, p)
			}
		}
		ts.pty.mu.Unlock()
		for _, p := range idle {
			p.idleStop.Store(true)
			ctx, cancel := context.WithTimeout(context.Background(), m.cfg.CloseTimeout)
			err := m.stopPty(ctx, ts, p, closeReasonIdle)
			cancel()
			if m.cfg.OnIdleStopped != nil {
				m.cfg.OnIdleStopped(ts.pty.tenantID, ts.pty.userID, ts.pty.id, p.ptyID, ts.pty.clientName, err)
			}
		}
	}
}

// Close stops the janitor and every session (process shutdown).
func (m *ToolSessions) Close() {
	select {
	case <-m.stop:
		return
	default:
		close(m.stop)
	}
	m.mu.Lock()
	all := make([]*ToolSession, 0, len(m.byID))
	for _, ts := range m.byID {
		all = append(all, ts)
	}
	m.mu.Unlock()
	for _, ts := range all {
		m.closeSession(ts, "shutdown")
	}
	m.wg.Wait()
}

// Active reports how many MCP sessions currently own PTYs state (tests/metrics).
func (m *ToolSessions) Active() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.byID)
}

// ---- rate limit ------------------------------------------------------------

type tokenBucket struct {
	mu     sync.Mutex
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
	now    func() time.Time
}

func newTokenBucket(rate float64, burst int, now func() time.Time) *tokenBucket {
	return &tokenBucket{rate: rate, burst: float64(burst), tokens: float64(burst), last: now(), now: now}
}

// allow takes one token; otherwise it returns how long until one is free.
func (b *tokenBucket) allow() (bool, time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	t := b.now()
	b.tokens = min(b.burst, b.tokens+t.Sub(b.last).Seconds()*b.rate)
	b.last = t
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	return false, time.Duration((1 - b.tokens) / b.rate * float64(time.Second))
}
