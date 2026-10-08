package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// capabilityFakeAgent embeds the shared fake for the unused interface surface
// and overrides only what the capability use cases touch.
type capabilityFakeAgent struct {
	*fakeDevServerAgentClient

	mu          sync.Mutex
	connected   bool
	info        HandshakeInfo
	infoOK      bool
	result      map[string]any
	err         error
	delay       time.Duration
	execCount   atomic.Int32
	execParams  []map[string]any
	execMethods []string
}

func newCapabilityFakeAgent() *capabilityFakeAgent {
	return &capabilityFakeAgent{
		fakeDevServerAgentClient: &fakeDevServerAgentClient{},
		connected:                true,
		infoOK:                   true,
		info: HandshakeInfo{
			Platform: "linux", Arch: "x64", NodeVersion: "v22.3.0",
			Capabilities: []string{"pty", "git"},
			Features:     []string{"agent.execPrompt", "agent.capabilities"},

			ProtocolVersion: 2, BuildVersion: "2.2.0",
		},
	}
}

func (a *capabilityFakeAgent) Exec(_ context.Context, _ domain.DevServer, method string, params map[string]any) (map[string]any, error) {
	a.execCount.Add(1)
	if a.delay > 0 {
		time.Sleep(a.delay)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.execMethods = append(a.execMethods, method)
	a.execParams = append(a.execParams, params)
	return a.result, a.err
}

func (a *capabilityFakeAgent) IsConnected(string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.connected
}

func (a *capabilityFakeAgent) LastHandshakeInfo(string) (HandshakeInfo, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.info, a.infoOK
}

func (a *capabilityFakeAgent) set(f func(a *capabilityFakeAgent)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	f(a)
}

type capabilityFakeStore struct {
	mu       sync.Mutex
	profiles map[string]domain.CapabilityProfile
	upserts  int
	getErr   error
	// failGetAfter, when > 0, makes every Get after that many calls fail.
	failGetAfter int
	getCalls     int
}

func newCapabilityFakeStore() *capabilityFakeStore {
	return &capabilityFakeStore{profiles: map[string]domain.CapabilityProfile{}}
}

func (s *capabilityFakeStore) Get(_ context.Context, tenantID, devServerID string) (domain.CapabilityProfile, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getCalls++
	if s.getErr != nil || (s.failGetAfter > 0 && s.getCalls > s.failGetAfter) {
		return domain.CapabilityProfile{}, false, errors.New("store unavailable")
	}
	p, ok := s.profiles[tenantID+":"+devServerID]
	return p, ok, nil
}

func (s *capabilityFakeStore) Upsert(_ context.Context, p domain.CapabilityProfile) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := p.TenantID + ":" + p.DevServerID
	old, existed := s.profiles[key]
	s.profiles[key] = p
	s.upserts++
	return old.Fingerprint, existed, nil
}

type capabilityFakeEvent struct {
	Subject  string
	TenantID string
	Payload  []byte
}

type capabilityFakeOutbox struct {
	mu     sync.Mutex
	events []capabilityFakeEvent
	err    error
}

func (o *capabilityFakeOutbox) EnqueueOutboxEvent(_ context.Context, _, tenantID, subject string, _ time.Time, _ int, payload []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.err != nil {
		return o.err
	}
	o.events = append(o.events, capabilityFakeEvent{Subject: subject, TenantID: tenantID, Payload: payload})
	return nil
}

func (o *capabilityFakeOutbox) count() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.events)
}

type capabilityFakeDevServers struct {
	DevServerRepository
	byID map[string]domain.DevServer
}

func (r *capabilityFakeDevServers) Get(_ context.Context, tenantID, id string) (domain.DevServer, error) {
	ds, ok := r.byID[id]
	if !ok || ds.TenantID != tenantID {
		return domain.DevServer{}, domain.ErrNotFound
	}
	return ds, nil
}

type capabilityFakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *capabilityFakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *capabilityFakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// capabilityReport builds an agent.capabilities result with the volatile
// host fields the fingerprint must ignore.
func capabilityReport(goVersion string, memFreeMb float64) map[string]any {
	raw := `{
		"schemaVersion": 1,
		"probedAt": "2026-10-06T10:00:00.000Z",
		"partial": false,
		"agent": {"buildVersion": "2.2.0", "protocolVersion": 2},
		"host": {"platform": "linux", "arch": "x64", "nodeVersion": "v22.3.0", "cpuCount": 8, "memTotalMb": 32000, "memFreeMb": 0, "diskFreeMb": 1, "loadAvg1": 0.4},
		"tools": [{"id": "go", "installed": true, "version": "` + goVersion + `"}],
		"claude": {"installed": true, "auth": "logged_in"},
		"env": [{"name": "ANTHROPIC_API_KEY", "present": true}]
	}`
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		panic(err)
	}
	m["host"].(map[string]any)["memFreeMb"] = memFreeMb
	return m
}
