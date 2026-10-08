package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

const capTenant = "tenant-1"
const capDevServer = "ds-1"

type refreshHarness struct {
	uc      *RefreshDevServerCapabilities
	agent   *capabilityFakeAgent
	store   *capabilityFakeStore
	outbox  *capabilityFakeOutbox
	clock   *capabilityFakeClock
	logs    *bytes.Buffer
	devSrvs *capabilityFakeDevServers
}

func newRefreshHarness(t *testing.T, minInterval time.Duration) *refreshHarness {
	t.Helper()
	ds, err := domain.NewDevServer(capDevServer, capTenant, "10.0.0.5", domain.ConnectionModeDirectWebSocket, "", nil)
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}
	h := &refreshHarness{
		agent:   newCapabilityFakeAgent(),
		store:   newCapabilityFakeStore(),
		outbox:  &capabilityFakeOutbox{},
		clock:   &capabilityFakeClock{now: time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)},
		logs:    &bytes.Buffer{},
		devSrvs: &capabilityFakeDevServers{byID: map[string]domain.DevServer{capDevServer: ds}},
	}
	h.agent.result = capabilityReport("1.22.3", 20000)
	logger := slog.New(slog.NewTextHandler(h.logs, nil))
	h.uc = NewRefreshDevServerCapabilities(h.devSrvs, h.agent, h.store, h.outbox, h.clock, minInterval).WithLogger(logger)
	return h
}

func (h *refreshHarness) run(t *testing.T, force bool) (domain.CapabilityProfile, error) {
	t.Helper()
	return h.uc.Execute(context.Background(), capTenant, capDevServer, force)
}

func TestRefresh_ProbeSuccess_UpsertsAndEmitsOnFirstProfile(t *testing.T) {
	h := newRefreshHarness(t, time.Minute)
	p, err := h.run(t, false)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if p.Source != domain.ProfileSourceProbe || p.Degraded() {
		t.Errorf("source=%q degraded=%v, want probe/false", p.Source, p.Degraded())
	}
	if p.AgentBuildVersion != "2.2.0" || p.ProtocolVersion != 2 {
		t.Errorf("version fields not taken from handshake: %+v", p)
	}
	if !p.HasFeature(domain.FeatureCapabilities) || !p.HasFeature(domain.FeatureAgentExecPrompt) {
		t.Errorf("features from handshake lost: %v", p.Features)
	}
	if len(p.Fingerprint) != 64 {
		t.Errorf("fingerprint %q is not sha256 hex", p.Fingerprint)
	}
	if h.store.upserts != 1 {
		t.Errorf("upserts=%d, want 1", h.store.upserts)
	}
	if h.outbox.count() != 1 || h.outbox.events[0].Subject != CapabilitiesChangedSubject || h.outbox.events[0].TenantID != capTenant {
		t.Fatalf("want one capabilities_changed event for the tenant, got %+v", h.outbox.events)
	}
	if got := h.agent.execMethods; len(got) != 1 || got[0] != "agent.capabilities" {
		t.Errorf("exec methods = %v", got)
	}
	if h.agent.execParams[0] != nil {
		t.Errorf("non-forced refresh must not send refresh=true, got %v", h.agent.execParams[0])
	}
}

func TestRefresh_FingerprintUnchanged_NoEvent(t *testing.T) {
	h := newRefreshHarness(t, time.Nanosecond)
	if _, err := h.run(t, false); err != nil {
		t.Fatalf("first: %v", err)
	}
	h.clock.advance(time.Hour)
	// Only volatile host fields differ between the two probes.
	h.agent.set(func(a *capabilityFakeAgent) { a.result = capabilityReport("1.22.3", 11111) })
	if _, err := h.run(t, false); err != nil {
		t.Fatalf("second: %v", err)
	}
	if h.agent.execCount.Load() != 2 {
		t.Fatalf("exec count=%d, want 2", h.agent.execCount.Load())
	}
	if h.outbox.count() != 1 {
		t.Errorf("events=%d, want 1 (no event when only volatile fields change)", h.outbox.count())
	}
}

func TestRefresh_FingerprintChanged_OneEvent(t *testing.T) {
	h := newRefreshHarness(t, time.Nanosecond)
	if _, err := h.run(t, false); err != nil {
		t.Fatalf("first: %v", err)
	}
	h.clock.advance(time.Hour)
	h.agent.set(func(a *capabilityFakeAgent) { a.result = capabilityReport("1.23.0", 20000) })
	if _, err := h.run(t, false); err != nil {
		t.Fatalf("second: %v", err)
	}
	if h.outbox.count() != 2 {
		t.Errorf("events=%d, want 2 (first profile + one change)", h.outbox.count())
	}
}

func TestRefresh_MethodNotFound_BuildsHandshakeOnly(t *testing.T) {
	h := newRefreshHarness(t, time.Minute)
	h.agent.err = domain.ErrAgentMethodNotFound
	h.agent.result = nil
	h.agent.info.Features = nil
	h.agent.info.ProtocolVersion = 0
	h.agent.info.BuildVersion = ""
	p, err := h.run(t, false)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if p.Source != domain.ProfileSourceHandshakeOnly || !p.Degraded() {
		t.Errorf("source=%q degraded=%v, want handshake_only/true", p.Source, p.Degraded())
	}
	if len(p.Features) != 0 {
		t.Errorf("features=%v, want none", p.Features)
	}
	if p.ProtocolVersion != 1 {
		t.Errorf("protocol=%d, want 1 for an agent that reports none", p.ProtocolVersion)
	}
	var body map[string]any
	if err := json.Unmarshal(p.ProfileJSON, &body); err != nil {
		t.Fatalf("profile json: %v", err)
	}
	host := body["host"].(map[string]any)
	if host["platform"] != "linux" || host["arch"] != "x64" || host["nodeVersion"] != "v22.3.0" {
		t.Errorf("host from handshake wrong: %v", host)
	}
	if body["schemaVersion"] != float64(1) {
		t.Errorf("schemaVersion = %v", body["schemaVersion"])
	}
	if h.outbox.count() != 1 {
		t.Errorf("events=%d, want 1", h.outbox.count())
	}
}

func TestRefresh_AgentTimeout_KeepsOldProfile(t *testing.T) {
	h := newRefreshHarness(t, time.Nanosecond)
	first, err := h.run(t, false)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	h.clock.advance(time.Hour)
	h.agent.set(func(a *capabilityFakeAgent) { a.err = context.DeadlineExceeded; a.result = nil })
	got, err := h.run(t, true)
	if err != nil {
		t.Fatalf("a transient failure with a stored profile must not error: %v", err)
	}
	if got.Fingerprint != first.Fingerprint || got.Source != domain.ProfileSourceProbe {
		t.Errorf("old profile not kept: %+v", got)
	}
	if h.store.upserts != 1 || h.outbox.count() != 1 {
		t.Errorf("upserts=%d events=%d, want 1/1 (no downgrade, no event)", h.store.upserts, h.outbox.count())
	}
}

func TestRefresh_AgentTimeout_NoStoredProfile_ReturnsError(t *testing.T) {
	h := newRefreshHarness(t, time.Minute)
	h.agent.err = context.DeadlineExceeded
	_, err := h.run(t, false)
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("want AppError, got %v", err)
	}
	if h.store.upserts != 0 {
		t.Errorf("a failed first probe must not store anything")
	}
}

func TestRefresh_NotConnected_ReturnsStored(t *testing.T) {
	h := newRefreshHarness(t, time.Nanosecond)
	first, err := h.run(t, false)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	h.agent.set(func(a *capabilityFakeAgent) { a.connected = false })
	h.clock.advance(time.Hour)
	got, err := h.run(t, true)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.Fingerprint != first.Fingerprint {
		t.Errorf("stored profile not returned")
	}
	if h.agent.execCount.Load() != 1 {
		t.Errorf("exec count=%d, a disconnected agent must never be probed (it would dial)", h.agent.execCount.Load())
	}
}

func TestRefresh_NotConnected_NoProfile_FailedPrecondition(t *testing.T) {
	h := newRefreshHarness(t, time.Minute)
	h.agent.connected = false
	_, err := h.run(t, false)
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindFailedPrecondition {
		t.Fatalf("want FailedPrecondition, got %v", err)
	}
}

func TestRefresh_ConcurrentCallsCoalesce(t *testing.T) {
	h := newRefreshHarness(t, time.Minute)
	h.agent.delay = 150 * time.Millisecond

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := h.uc.Execute(context.Background(), capTenant, capDevServer, true)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("Execute: %v", err)
		}
	}
	if n := h.agent.execCount.Load(); n != 1 {
		t.Errorf("exec count=%d, want exactly 1 for 8 concurrent calls", n)
	}
	if h.outbox.count() != 1 {
		t.Errorf("events=%d, want 1", h.outbox.count())
	}
}

func TestRefresh_MinIntervalSkips(t *testing.T) {
	h := newRefreshHarness(t, 5*time.Minute)
	if _, err := h.run(t, false); err != nil {
		t.Fatalf("first: %v", err)
	}
	h.clock.advance(time.Minute)
	if _, err := h.run(t, false); err != nil {
		t.Fatalf("second: %v", err)
	}
	if n := h.agent.execCount.Load(); n != 1 {
		t.Errorf("exec count=%d, want 1 inside the min interval", n)
	}
	h.clock.advance(10 * time.Minute)
	if _, err := h.run(t, false); err != nil {
		t.Fatalf("third: %v", err)
	}
	if n := h.agent.execCount.Load(); n != 2 {
		t.Errorf("exec count=%d, want 2 after the interval elapsed", n)
	}
}

func TestRefresh_ForceBypassesMinInterval(t *testing.T) {
	h := newRefreshHarness(t, time.Hour)
	if _, err := h.run(t, false); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := h.run(t, true); err != nil {
		t.Fatalf("forced: %v", err)
	}
	if n := h.agent.execCount.Load(); n != 2 {
		t.Errorf("exec count=%d, want 2", n)
	}
	if p := h.agent.execParams[1]; p == nil || p["refresh"] != true {
		t.Errorf("forced refresh must ask the agent to bypass its cache, got %v", p)
	}
}

func TestRefresh_UnknownDevServer_NotFound(t *testing.T) {
	h := newRefreshHarness(t, time.Minute)
	_, err := h.uc.Execute(context.Background(), "other-tenant", capDevServer, false)
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindNotFound {
		t.Fatalf("want NotFound for a foreign tenant, got %v", err)
	}
}

func TestRefresh_EventPayloadCarriesNoProfileOrSecrets(t *testing.T) {
	h := newRefreshHarness(t, time.Minute)
	h.outbox.err = errors.New("outbox down")
	if _, err := h.run(t, false); err != nil {
		t.Fatalf("an outbox failure must not fail the refresh: %v", err)
	}
	h.outbox.err = nil

	h2 := newRefreshHarness(t, time.Minute)
	if _, err := h2.run(t, false); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(h2.outbox.events[0].Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	for _, k := range []string{"dev_server_id", "fingerprint", "source", "features"} {
		if _, ok := payload[k]; !ok {
			t.Errorf("payload missing %q", k)
		}
	}
	if len(payload) != 4 {
		t.Errorf("payload has unexpected keys: %v", payload)
	}
	for _, sentinel := range []string{"ANTHROPIC_API_KEY", "memFreeMb", "claude"} {
		if bytes.Contains(h2.outbox.events[0].Payload, []byte(sentinel)) || bytes.Contains(h2.logs.Bytes(), []byte(sentinel)) {
			t.Errorf("profile content %q leaked into event or logs", sentinel)
		}
	}
}
