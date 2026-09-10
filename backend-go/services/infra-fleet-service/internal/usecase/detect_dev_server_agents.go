package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// AgentProbe mirrors agent/src/relay/preflight-handler.ts's
// AgentDetectionCommand shape verbatim (json tags matter here — this
// struct crosses the wire as preflight.detectAgents's params.commands).
type AgentProbe struct {
	ID                  string   `json:"id"`
	Cmd                 string   `json:"cmd"`
	RequiredCommands    []string `json:"requiredCommands,omitempty"`
	UnsupportedRuntimes []string `json:"unsupportedRuntimes,omitempty"`
}

// defaultAgentProbes is BL-FLEET-04 Step 3's exact four
// ("SSH exec `which claude codex gemini openai`") — closed here without
// any raw SSH exec, via the already-real preflight.detectAgents JSON-RPC
// method (confirmed in agent/src/relay/preflight-handler.ts), a
// caller-supplied PATH-lookup primitive that never executes the detected
// command, reachable uniformly across all three connection modes.
var defaultAgentProbes = []AgentProbe{
	{ID: "claude", Cmd: "claude"},
	{ID: "codex", Cmd: "codex"},
	{ID: "gemini", Cmd: "gemini"},
	{ID: "openai", Cmd: "openai"},
}

// DetectedAgents is preflight.detectAgents's decoded result.
type DetectedAgents struct {
	Agents   []string
	Platform string
}

// DetectDevServerAgents implements BL-FLEET-04 Step 3.
type DetectDevServerAgents struct {
	devServers DevServerRepository
	agent      DevServerAgentClient
}

func NewDetectDevServerAgents(devServers DevServerRepository, agent DevServerAgentClient) *DetectDevServerAgents {
	return &DetectDevServerAgents{devServers: devServers, agent: agent}
}

func (uc *DetectDevServerAgents) Execute(ctx context.Context, tenantID, devServerID string) (DetectedAgents, error) {
	ds, err := uc.devServers.Get(ctx, tenantID, devServerID)
	if err != nil {
		return DetectedAgents{}, err
	}
	result, err := uc.agent.Exec(ctx, ds, "preflight.detectAgents", map[string]any{
		"commands": defaultAgentProbes,
	})
	if err != nil {
		return DetectedAgents{}, apperrors.New(apperrors.KindInternal, "INFRA_DETECT_AGENTS_FAILED", "failed to detect installed AI agents", err)
	}
	return decodeDetectedAgents(result), nil
}

// decodeDetectedAgents is pure parsing of preflight.detectAgents's
// generic map[string]any result — malformed/missing fields degrade to
// zero values, never panic.
func decodeDetectedAgents(result map[string]any) DetectedAgents {
	var out DetectedAgents
	if agents, ok := result["agents"].([]any); ok {
		for _, a := range agents {
			if s, ok := a.(string); ok {
				out.Agents = append(out.Agents, s)
			}
		}
	}
	if p, ok := result["platform"].(string); ok {
		out.Platform = p
	}
	return out
}
