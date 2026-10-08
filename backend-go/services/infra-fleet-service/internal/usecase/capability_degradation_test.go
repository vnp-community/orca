package usecase

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// loadAgentCapabilityGolden reads the agent's golden report (a verbatim copy
// of agent/src/relay/__fixtures__/agent-capabilities-golden.json, kept in
// sync by scripts/check-agent-capability-golden.sh).
func loadAgentCapabilityGolden(t *testing.T, variant string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "adapter", "devserveragent", "testdata", "agent_capabilities_v1.golden.json"))
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	var all map[string]map[string]any
	if err := json.Unmarshal(b, &all); err != nil {
		t.Fatalf("decoding golden: %v", err)
	}
	if all[variant] == nil {
		t.Fatalf("golden variant %q missing", variant)
	}
	return all[variant]
}

func TestDegradation_NewAgent_ProbeStoresFeatures(t *testing.T) {
	h := newRefreshHarness(t, time.Minute)
	h.agent.result = loadAgentCapabilityGolden(t, "full")
	h.agent.info.Features = []string{
		domain.FeatureCapabilities, domain.FeatureAgentExecPrompt, domain.FeatureReadonly,
	}
	p, err := h.run(t, false)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if p.Source != domain.ProfileSourceProbe || p.Degraded() {
		t.Errorf("source=%q degraded=%v", p.Source, p.Degraded())
	}
	if !p.HasFeature(domain.FeatureReadonly) {
		t.Errorf("features=%v, want agent.execPrompt.readonly", p.Features)
	}
	var report struct {
		SchemaVersion int `json:"schemaVersion"`
		Agent         struct {
			BuildVersion    string `json:"buildVersion"`
			ProtocolVersion int    `json:"protocolVersion"`
		} `json:"agent"`
		Tools []struct {
			ID        string `json:"id"`
			Installed bool   `json:"installed"`
		} `json:"tools"`
		Claude struct {
			Auth string `json:"auth"`
		} `json:"claude"`
		Env []struct {
			Name    string `json:"name"`
			Present bool   `json:"present"`
		} `json:"env"`
	}
	if err := json.Unmarshal(p.ProfileJSON, &report); err != nil {
		t.Fatalf("stored profile is not valid report JSON: %v", err)
	}
	if report.SchemaVersion != 1 || report.Agent.BuildVersion != "2.2.0" || report.Agent.ProtocolVersion != 2 ||
		report.Claude.Auth != "logged_in" || len(report.Tools) != 6 || report.Tools[0].ID != "go" || !report.Tools[0].Installed ||
		len(report.Env) != 3 || report.Env[0].Name != "ANTHROPIC_API_KEY" || !report.Env[0].Present {
		t.Errorf("golden report did not round-trip: %+v", report)
	}
}

func TestDegradation_OldAgent_MethodNotFound(t *testing.T) {
	h := newRefreshHarness(t, time.Minute)
	h.agent.err = domain.ErrAgentMethodNotFound
	h.agent.info = HandshakeInfo{Platform: "darwin", Arch: "arm64", NodeVersion: "v20.1.0", Capabilities: []string{"pty"}}
	p, err := h.run(t, false)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if p.Source != domain.ProfileSourceHandshakeOnly || !p.Degraded() || len(p.Features) != 0 {
		t.Fatalf("profile = %+v", p)
	}
	var body struct {
		Host struct {
			Platform string `json:"platform"`
		} `json:"host"`
	}
	_ = json.Unmarshal(p.ProfileJSON, &body)
	if body.Host.Platform != "darwin" {
		t.Errorf("platform from handshake = %q", body.Host.Platform)
	}
}

func TestDegradation_OldAgentThenUpgrade_FingerprintChangesOnce(t *testing.T) {
	h := newRefreshHarness(t, time.Nanosecond)
	h.agent.err = domain.ErrAgentMethodNotFound
	old, err := h.run(t, false)
	if err != nil {
		t.Fatalf("old agent: %v", err)
	}
	h.clock.advance(time.Hour)
	if _, err := h.run(t, false); err != nil { // same old agent again: no event
		t.Fatalf("old agent again: %v", err)
	}
	if h.outbox.count() != 1 {
		t.Fatalf("events=%d, want 1 after two identical handshake_only refreshes", h.outbox.count())
	}

	h.agent.set(func(a *capabilityFakeAgent) {
		a.err = nil
		a.result = loadAgentCapabilityGolden(t, "full")
	})
	h.clock.advance(time.Hour)
	upgraded, err := h.run(t, false)
	if err != nil {
		t.Fatalf("upgraded agent: %v", err)
	}
	if upgraded.Source != domain.ProfileSourceProbe || upgraded.Fingerprint == old.Fingerprint {
		t.Errorf("upgrade did not change the profile: %+v", upgraded)
	}
	if h.outbox.count() != 2 {
		t.Errorf("events=%d, want exactly 2 (one per transition)", h.outbox.count())
	}
	h.clock.advance(time.Hour)
	if _, err := h.run(t, false); err != nil {
		t.Fatalf("steady state: %v", err)
	}
	if h.outbox.count() != 2 {
		t.Errorf("events=%d, steady state must not emit", h.outbox.count())
	}
}

func TestDegradation_DesktopBundleTreatedAsOld(t *testing.T) {
	h := newRefreshHarness(t, time.Minute)
	h.agent.err = domain.ErrAgentMethodNotFound
	h.agent.info = HandshakeInfo{Platform: "win32", Arch: "x64", NodeVersion: "v22.3.0", AgentVersion: "5.0.0"} // no features/protocol/build
	p, err := h.run(t, false)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !p.Degraded() || p.ProtocolVersion != 1 || p.AgentBuildVersion != "" || len(p.Features) != 0 {
		t.Errorf("desktop bundle must read as a protocol-1 agent without features: %+v", p)
	}
}

func TestDegradation_PartialReportStillProbe(t *testing.T) {
	h := newRefreshHarness(t, time.Minute)
	h.agent.result = loadAgentCapabilityGolden(t, "partial")
	p, err := h.run(t, false)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if p.Source != domain.ProfileSourceProbe {
		t.Errorf("a partial report is still a probe result, got %q", p.Source)
	}
}
