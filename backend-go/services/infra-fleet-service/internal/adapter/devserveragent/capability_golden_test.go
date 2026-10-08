package devserveragent

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func loadCapabilityGolden(t *testing.T) map[string]map[string]any {
	t.Helper()
	b, err := os.ReadFile("testdata/agent_capabilities_v1.golden.json")
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	var all map[string]map[string]any
	if err := json.Unmarshal(b, &all); err != nil {
		t.Fatalf("decoding golden: %v", err)
	}
	return all
}

// The golden is the agent's own fixture; these assertions pin the field names
// infra-fleet-service itself reads or generates a lookalike of
// (buildHandshakeOnlyProfile mirrors agent/host).
func TestAgentCapabilitiesGolden_FieldNames(t *testing.T) {
	full := loadCapabilityGolden(t)["full"]
	if full["schemaVersion"] != float64(1) {
		t.Errorf("schemaVersion = %v", full["schemaVersion"])
	}
	agent := full["agent"].(map[string]any)
	if agent["buildVersion"] != "2.2.0" || agent["protocolVersion"] != float64(2) {
		t.Errorf("agent = %v", agent)
	}
	host := full["host"].(map[string]any)
	for _, k := range []string{"platform", "arch", "nodeVersion", "cpuCount", "memTotalMb", "memFreeMb", "diskFreeMb", "loadAvg1"} {
		if _, ok := host[k]; !ok {
			t.Errorf("host.%s missing from golden", k)
		}
	}
	if claude := full["claude"].(map[string]any); claude["auth"] != "logged_in" {
		t.Errorf("claude = %v", claude)
	}
	if _, ok := full["env"].([]any); !ok {
		t.Errorf("env must be an array of {name,present}")
	}
}

func TestAgentCapabilitiesGolden_FingerprintIgnoresVolatileHostFields(t *testing.T) {
	full := loadCapabilityGolden(t)["full"]
	b1, _ := json.Marshal(full)
	fp1, err := domain.ComputeFingerprint(b1)
	if err != nil {
		t.Fatalf("ComputeFingerprint: %v", err)
	}

	host := full["host"].(map[string]any)
	host["memFreeMb"] = float64(1)
	host["diskFreeMb"] = float64(2)
	host["loadAvg1"] = float64(9.9)
	full["probedAt"] = "2030-01-01T00:00:00.000Z"
	b2, _ := json.Marshal(full)
	fp2, _ := domain.ComputeFingerprint(b2)
	if fp1 != fp2 {
		t.Errorf("volatile fields changed the fingerprint")
	}

	full["claude"].(map[string]any)["version"] = "9.9.9"
	b3, _ := json.Marshal(full)
	if fp3, _ := domain.ComputeFingerprint(b3); fp3 == fp1 {
		t.Errorf("a claude version change must change the fingerprint")
	}
}

// The agent's handshake sends exactly these keys (agent-session-handshake.ts);
// a rename on either side breaks feature detection silently otherwise.
func TestHandshakeInfo_DecodesAgentHandshakeShape(t *testing.T) {
	const wire = `{"agentVersion":"5.0.0","platform":"linux","arch":"x64","nodeVersion":"v22.3.0",
		"capabilities":["pty"],"tools":["pty.create"],"devServerId":"ignored",
		"protocolVersion":2,"buildVersion":"2.2.0",
		"features":["agent.execPrompt","agent.execPrompt.readonly","agent.capabilities"]}`
	var info HandshakeInfo
	if err := json.Unmarshal([]byte(wire), &info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if info.ProtocolVersion != 2 || info.BuildVersion != "2.2.0" || len(info.Features) != 3 || info.Features[2] != domain.FeatureCapabilities {
		t.Errorf("decoded %+v", info)
	}
	for _, f := range []string{domain.FeatureAgentExecPrompt, domain.FeatureReadonly, domain.FeatureWorkspaceKind,
		domain.FeatureChanges, domain.FeatureResultBlock, domain.FeatureCapabilities, domain.FeatureAIComplete, domain.FeatureAICompleteUsage} {
		if got := domain.SanitizeAgentFeatures([]string{f}); len(got) != 1 {
			t.Errorf("feature %q rejected by SanitizeAgentFeatures", f)
		}
	}
}
