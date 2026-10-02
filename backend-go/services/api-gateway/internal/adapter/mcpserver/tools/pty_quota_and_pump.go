package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

// quotaCache remembers, per tenant, how many MCP-created terminals are open
// according to infra-fleet (terminal.list filtered on origin.type == "mcp"), so
// per-user/tenant caps hold across replicas and replica restarts.
type quotaCache struct {
	mu       sync.Mutex
	byTenant map[string]quotaEntry
}

type quotaEntry struct {
	at      time.Time
	perUser map[string]int
	total   int
}

func (m *ToolSessions) durableCounts(ctx context.Context, ts *ToolSession) (user, tenant int) {
	st := ts.pty
	m.quota.mu.Lock()
	if e, ok := m.quota.byTenant[st.tenantID]; ok && m.now().Sub(e.at) < m.cfg.QuotaCacheTTL {
		m.quota.mu.Unlock()
		return e.perUser[st.userID], e.total
	}
	m.quota.mu.Unlock()
	res, err := m.disp.Dispatch(ctx, st.identity, "terminal.list", mustArgs(map[string]any{}))
	if err != nil {
		m.log.Debug("mcp terminal quota: durable count unavailable, using local counts", slog.Any("error", err))
		return 0, 0
	}
	b, _ := json.Marshal(res)
	var rows []struct {
		Origin *struct {
			Type   string `json:"type"`
			UserID string `json:"userId"`
		} `json:"origin"`
	}
	if json.Unmarshal(b, &rows) != nil {
		return 0, 0
	}
	e := quotaEntry{at: m.now(), perUser: map[string]int{}}
	for _, r := range rows {
		if r.Origin != nil && r.Origin.Type == "mcp" {
			e.total++
			e.perUser[r.Origin.UserID]++
		}
	}
	m.quota.mu.Lock()
	if m.quota.byTenant == nil {
		m.quota.byTenant = map[string]quotaEntry{}
	}
	m.quota.byTenant[st.tenantID] = e
	m.quota.mu.Unlock()
	return e.perUser[st.userID], e.total
}

func (p *managedPty) live() bool {
	exited, _, detached, _ := p.ring.State()
	return !exited && !detached && !p.stopping.Load()
}

// reserve admits one more PTY of kind or fails with QUOTA_EXCEEDED. The returned
// release must be called once the PTY is registered (or creation failed).
func (m *ToolSessions) reserve(ctx context.Context, ts *ToolSession, kind string) (release func(), err error) {
	st := ts.pty
	var durUser, durTenant int
	if kind == kindTerminal {
		durUser, durTenant = m.durableCounts(ctx, ts)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	localUser, localTenant, sessionN := 0, 0, 0
	for _, other := range m.byID {
		o := other.pty
		if o.tenantID != st.tenantID {
			continue
		}
		o.mu.Lock()
		n := o.pending[kindTerminal]
		for _, p := range o.ptys {
			if p.kind == kindTerminal && p.live() {
				n++
			}
		}
		if other == ts {
			sessionN = n
			if kind == kindAgent {
				sessionN = o.pending[kindAgent]
				for _, p := range o.ptys {
					if p.kind == kindAgent && p.live() {
						sessionN++
					}
				}
			}
		}
		localTenant += n
		if o.userID == st.userID {
			localUser += n
		}
		o.mu.Unlock()
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.closed {
		return nil, &ToolError{"MCP_SESSION_CLOSED", "the MCP session was closed"}
	}
	limit := m.cfg.MaxTerminalsPerSession
	if kind == kindAgent {
		limit = m.cfg.MaxAgentsPerSession
	}
	if sessionN >= limit {
		return nil, &ToolError{"QUOTA_EXCEEDED", fmt.Sprintf("at most %d %s(s) per MCP session; stop one first", limit, kind)}
	}
	if kind == kindTerminal {
		if max(localUser, durUser) >= m.cfg.MaxTerminalsPerUser {
			return nil, &ToolError{"QUOTA_EXCEEDED", fmt.Sprintf("at most %d MCP terminals per user; stop one first", m.cfg.MaxTerminalsPerUser)}
		}
		if max(localTenant, durTenant) >= m.cfg.MaxTerminalsPerTenant {
			return nil, &ToolError{"QUOTA_EXCEEDED", fmt.Sprintf("at most %d MCP terminals per tenant; try again later", m.cfg.MaxTerminalsPerTenant)}
		}
	}
	st.pending[kind]++
	var once sync.Once
	return func() {
		once.Do(func() {
			st.mu.Lock()
			st.pending[kind]--
			st.mu.Unlock()
		})
	}, nil
}

// register records a freshly started PTY and starts the goroutine that feeds
// its ring from the AttachPty push events. The pump never blocks on a slow
// reader (ring writes are O(1) and overwrite), so drainAttachPtyOutput cannot
// stall on its unbuffered channel.
func (m *ToolSessions) register(ts *ToolSession, pt *managedPty, events <-chan wscompat.PushEvent) error {
	st := ts.pty
	pt.touch(m.now())
	st.mu.Lock()
	if st.closed {
		st.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), m.cfg.CloseTimeout)
		defer cancel()
		m.stopPty(ctx, ts, pt)
		return &ToolError{"MCP_SESSION_CLOSED", "the MCP session was closed"}
	}
	st.ptys[pt.ptyID] = pt
	if pt.sessionID != "" {
		st.agents[pt.sessionID] = pt
	}
	st.evictExitedLocked()
	st.pumps.Add(1)
	st.mu.Unlock()
	go func() {
		defer st.pumps.Done()
		for ev := range events {
			if len(ev.Args) == 0 {
				continue
			}
			f, _ := ev.Args[0].(map[string]any)
			switch ev.Channel {
			case "terminal.output":
				if b, ok := f["data"].([]byte); ok {
					pt.ring.Append(b, m.now())
				}
			case "terminal.exited":
				code, _ := f["exitCode"].(int32)
				pt.ring.MarkExited(code, m.now())
			}
		}
		if exited, _, _, _ := pt.ring.State(); !exited {
			pt.ring.MarkDetached()
		}
	}()
	return nil
}

func (st *ptyState) evictExitedLocked() {
	var exited []*managedPty
	for _, p := range st.ptys {
		if e, _, d, _ := p.ring.State(); e || d {
			exited = append(exited, p)
		}
	}
	for len(exited) > maxExitedKept {
		oldest := 0
		for i, p := range exited {
			if p.created.Before(exited[oldest].created) {
				oldest = i
			}
		}
		delete(st.ptys, exited[oldest].ptyID)
		if exited[oldest].sessionID != "" {
			delete(st.agents, exited[oldest].sessionID)
		}
		exited = append(exited[:oldest], exited[oldest+1:]...)
	}
}

func (st *ptyState) find(ptyID string) (*managedPty, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	p, ok := st.ptys[ptyID]
	return p, ok
}

func (st *ptyState) findAgent(sessionID string) (*managedPty, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	p, ok := st.agents[sessionID]
	return p, ok
}
