package mcpserver

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemorySessionStore is the in-process SessionStore: the default when no
// durable store is wired (single replica; sessions die with the process, like
// the SDK's own) and the shared "database" of the two-replica unit tests.
// It implements the same idle-TTL semantics as the mcp-service reaper.
type MemorySessionStore struct {
	mu   sync.Mutex
	ttl  time.Duration
	now  SessionClock
	rows map[string]*memSession // key: hex(secret hash)
	byID map[string]*memSession

	streams map[string]memStreamRow // stream id -> owner
	nextStr int
}

type memStreamRow struct{ tenant, user string }

type memSession struct {
	rec      SessionRecord
	lastSeen time.Time
	calls    int64
}

func NewMemorySessionStore(idleTTL time.Duration, now SessionClock) *MemorySessionStore {
	if now == nil {
		now = time.Now
	}
	return &MemorySessionStore{ttl: idleTTL, now: now, rows: map[string]*memSession{}, byID: map[string]*memSession{}, streams: map[string]memStreamRow{}}
}

func (m *MemorySessionStore) expireLocked(s *memSession) {
	if s.rec.State != SessionClosed && m.ttl > 0 && m.now().Sub(s.lastSeen) > m.ttl {
		s.rec.State = SessionClosed
	}
}

func (m *MemorySessionStore) Create(_ context.Context, p Principal, hash []byte, ns NewSession) (SessionRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := &memSession{lastSeen: m.now(), rec: SessionRecord{
		ID: uuid.NewString(), TenantID: p.TenantID, UserID: p.UserID, ClientID: p.ClientID, ClientName: ns.ClientName,
		ClientVersion: ns.ClientVersion, GrantID: p.GrantID, TokenID: p.TokenID, ProtocolVersion: ns.ProtocolVersion,
		Capabilities: ns.Capabilities, LogLevel: "warning", State: SessionInitializing,
	}}
	m.rows[hex.EncodeToString(hash)] = s
	m.byID[s.rec.ID] = s
	return s.rec, nil
}

func (m *MemorySessionStore) Lookup(_ context.Context, _ Principal, hash []byte) (SessionRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.rows[hex.EncodeToString(hash)]
	if !ok {
		return SessionRecord{}, ErrSessionNotFound
	}
	m.expireLocked(s)
	return s.rec, nil
}

func (m *MemorySessionStore) Touch(_ context.Context, _ Principal, id string, t SessionTouch) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.byID[id]
	if !ok {
		return "", ErrSessionNotFound
	}
	m.expireLocked(s)
	if s.rec.State == SessionClosed {
		return SessionClosed, nil
	}
	s.lastSeen = m.now()
	s.calls += t.ToolCallsDelta
	if t.Ready && s.rec.State == SessionInitializing {
		s.rec.State = SessionReady
	}
	if t.LogLevel != "" {
		s.rec.LogLevel = t.LogLevel
	}
	return s.rec.State, nil
}

func (m *MemorySessionStore) Close(_ context.Context, _ Principal, id, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.byID[id]
	if !ok {
		return ErrSessionNotFound
	}
	s.rec.State = SessionClosed
	return nil
}

var _ SessionStore = (*MemorySessionStore)(nil)

// OpenStream/HeartbeatStream/CloseStream make the memory store a StreamRegistry
// with cluster-wide (store-wide) caps, like mcp-service's session_streams.
func (m *MemorySessionStore) OpenStream(_ context.Context, p Principal, id string, maxUser, maxTenant int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.byID[id]
	if !ok || s.rec.UserID != p.UserID || s.rec.TenantID != p.TenantID {
		return "", ErrSessionNotFound
	}
	var nu, nt int
	for _, r := range m.streams {
		if r.tenant == p.TenantID {
			nt++
			if r.user == p.UserID {
				nu++
			}
		}
	}
	if (maxUser > 0 && nu >= maxUser) || (maxTenant > 0 && nt >= maxTenant) {
		return "", ErrStreamLimit
	}
	m.nextStr++
	sid := fmt.Sprintf("stream-%d", m.nextStr)
	m.streams[sid] = memStreamRow{tenant: p.TenantID, user: p.UserID}
	return sid, nil
}

func (m *MemorySessionStore) HeartbeatStream(context.Context, Principal, string) error { return nil }

func (m *MemorySessionStore) CloseStream(_ context.Context, _ Principal, streamID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.streams, streamID)
	return nil
}

var _ StreamRegistry = (*MemorySessionStore)(nil)
