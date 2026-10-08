package usecase

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

const (
	tenA  = "aaaaaaaa-0000-4000-8000-000000000001"
	adm   = "11111111-0000-4000-8000-000000000001"
	usr   = "22222222-0000-4000-8000-000000000002"
	usr2  = "33333333-0000-4000-8000-000000000003"
	team1 = "44444444-0000-4000-8000-000000000004"
)

func ctxAs(tenantID, userID, role string) context.Context {
	ctx := tenant.WithTenantID(context.Background(), tenantID)
	ctx = tenant.WithUserID(ctx, userID)
	return tenant.WithRole(ctx, role)
}

type extClock struct{ t time.Time }

func (c extClock) Now() time.Time { return c.t }

// memRepo is an in-memory ExternalServerRepository that enforces tenant
// scoping and the review digest check like the SQL implementation.
type memRepo struct {
	mu      sync.Mutex
	servers map[string]domain.ExternalServer
	events  []domain.OutboxRecord
	probes  []ProbeRecord
	// lastTools mirrors last_probe_tools so approval snapshots ApprovedTools like SQL.
	lastTools map[string][]domain.ToolInfo
}

func newMemRepo() *memRepo { return &memRepo{servers: map[string]domain.ExternalServer{}} }

func (m *memRepo) lock() func() { m.mu.Lock(); return m.mu.Unlock }

func (m *memRepo) GetExternalServer(_ context.Context, tenantID, id string) (domain.ExternalServer, error) {
	defer m.lock()()
	s, ok := m.servers[id]
	if !ok || s.TenantID != tenantID {
		return domain.ExternalServer{}, domain.ErrNotFound()
	}
	return s, nil
}

func (m *memRepo) ListExternalServers(_ context.Context, tenantID string, f ExternalServerFilter) ([]domain.ExternalServer, error) {
	defer m.lock()()
	var out []domain.ExternalServer
	for _, s := range m.servers {
		if s.TenantID != tenantID || (f.Scope != "" && s.Scope != f.Scope) {
			continue
		}
		if f.OwnerUserID != "" && !(s.Scope == domain.ScopeUser && s.ScopeID == f.OwnerUserID) {
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *memRepo) CreateExternalServer(_ context.Context, s domain.ExternalServer, ev []domain.OutboxRecord) error {
	defer m.lock()()
	for _, o := range m.servers {
		if o.TenantID == s.TenantID && o.Scope == s.Scope && o.ScopeID == s.ScopeID && o.Name == s.Name {
			return domain.ErrNameConflict()
		}
	}
	m.servers[s.ID] = s
	m.events = append(m.events, ev...)
	return nil
}

func (m *memRepo) UpdateExternalServer(_ context.Context, s domain.ExternalServer, ev []domain.OutboxRecord) error {
	defer m.lock()()
	cur := m.servers[s.ID]
	if cur.Version != s.Version {
		return domain.ErrServerInvalid("modified concurrently")
	}
	s.Version++
	m.servers[s.ID] = s
	m.events = append(m.events, ev...)
	return nil
}

func (m *memRepo) SetSecretRef(_ context.Context, _, serverID string, ref domain.SecretRef, ev []domain.OutboxRecord) error {
	defer m.lock()()
	s := m.servers[serverID]
	set := func(refs []domain.SecretRef) {
		for i := range refs {
			if refs[i].Kind == ref.Kind && refs[i].Name == ref.Name {
				refs[i] = ref
			}
		}
	}
	set(s.EnvRefs)
	set(s.HeaderRefs)
	m.servers[serverID] = s
	m.events = append(m.events, ev...)
	return nil
}

func (m *memRepo) RecordProbe(_ context.Context, _, serverID string, r ProbeRecord, ev []domain.OutboxRecord) error {
	defer m.lock()()
	s := m.servers[serverID]
	s.LastProbeDigest, s.LastProbeAt = r.Digest, &r.At
	if m.lastTools == nil {
		m.lastTools = map[string][]domain.ToolInfo{}
	}
	m.lastTools[serverID] = r.Tools
	m.servers[serverID] = s
	m.probes = append(m.probes, r)
	m.events = append(m.events, ev...)
	return nil
}

func (m *memRepo) ApplyReview(_ context.Context, r ReviewRecord, ev []domain.OutboxRecord) (domain.ExternalServer, error) {
	defer m.lock()()
	s, ok := m.servers[r.ServerID]
	if !ok {
		return domain.ExternalServer{}, domain.ErrNotFound()
	}
	if r.Approve {
		if s.LastProbeDigest == "" || s.LastProbeDigest != r.ExpectedDigest {
			return domain.ExternalServer{}, domain.ErrDigestMismatch()
		}
		s.Status, s.ApprovedDigest = domain.StatusApproved, s.LastProbeDigest
		s.ApprovedTools = m.lastTools[r.ServerID]
	} else {
		s.Status = domain.StatusDisabled
	}
	s.ReviewedBy = r.ReviewerID
	m.servers[r.ServerID] = s
	m.events = append(m.events, ev...)
	return s, nil
}

func (m *memRepo) RecordHealth(_ context.Context, _, serverID string, h domain.Health, ev []domain.OutboxRecord) error {
	defer m.lock()()
	s := m.servers[serverID]
	s.Health = &h
	m.servers[serverID] = s
	m.events = append(m.events, ev...)
	return nil
}

func (m *memRepo) DeleteExternalServer(_ context.Context, _, id string, ev []domain.OutboxRecord) error {
	defer m.lock()()
	delete(m.servers, id)
	m.events = append(m.events, ev...)
	return nil
}

func (m *memRepo) ListServersByName(_ context.Context, tenantID string, names []string) ([]domain.ExternalServer, error) {
	defer m.lock()()
	var out []domain.ExternalServer
	for _, s := range m.servers {
		for _, n := range names {
			if s.TenantID == tenantID && s.Name == n {
				out = append(out, s)
			}
		}
	}
	return out, nil
}

func (m *memRepo) ClaimHealthChecks(context.Context, time.Time, int) ([]ServerKey, error) {
	return nil, nil
}

// EnqueueOutbox lets memRepo double as the OutboxWriter.
func (m *memRepo) EnqueueOutbox(_ context.Context, _ string, rec domain.OutboxRecord) error {
	defer m.lock()()
	m.events = append(m.events, rec)
	return nil
}

func (m *memRepo) put(s domain.ExternalServer) { m.servers[s.ID] = s }

type memBroker struct {
	mu      sync.Mutex
	secrets map[string]string
	putErr  error
}

func newMemBroker() *memBroker { return &memBroker{secrets: map[string]string{}} }

func (b *memBroker) Put(_ context.Context, _, owner string, v domain.SecretValue) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.putErr != nil {
		return b.putErr
	}
	b.secrets[owner] = string(v.Reveal())
	return nil
}

func (b *memBroker) Get(_ context.Context, _, owner string) (domain.SecretValue, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	v, ok := b.secrets[owner]
	if !ok {
		return domain.SecretValue{}, errors.New("not found")
	}
	return domain.NewSecretValue(v), nil
}

func (b *memBroker) Delete(_ context.Context, _, owner string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.secrets, owner)
	return nil
}

type fakeProber struct {
	tools []domain.ToolInfo
	err   error
	seen  []ProbeTarget
}

func (f *fakeProber) ListTools(_ context.Context, t ProbeTarget) (ProbeResult, error) {
	f.seen = append(f.seen, t)
	return ProbeResult{Tools: f.tools}, f.err
}
