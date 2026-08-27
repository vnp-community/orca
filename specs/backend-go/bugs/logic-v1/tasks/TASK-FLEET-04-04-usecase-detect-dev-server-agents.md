# TASK-FLEET-04-04: `usecase.DetectDevServerAgents` (Step 3)

**From Solution:** SOL-FLEET-04
**Priority:** P1
**Service:** `infra-fleet-service` (usecase)
**File:** `backend-go/services/infra-fleet-service/internal/usecase/detect_dev_server_agents.go` (new)
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BL-FLEET-04 Step 3 ("SSH exec `which claude codex gemini openai`") is
closed without any raw SSH exec, using the already-real
`preflight.detectAgents` JSON-RPC method (confirmed in
`agent/src/relay/preflight-handler.ts`) — a caller-supplied PATH-lookup
primitive that never executes the detected command, reachable uniformly
across all three connection modes.

## Changes to make

```go
// internal/usecase/detect_dev_server_agents.go
package usecase

var defaultAgentProbes = []AgentProbe{
    {ID: "claude", Cmd: "claude"}, {ID: "codex", Cmd: "codex"},
    {ID: "gemini", Cmd: "gemini"}, {ID: "openai", Cmd: "openai"},
} // BL-FLEET-04 Step 3's exact four

type AgentProbe struct {
    ID, Cmd                                string
    RequiredCommands, UnsupportedRuntimes []string
}

type DetectedAgents struct {
    Agents   []string
    Platform string
}

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
        return DetectedAgents{}, apperrors.New(apperrors.KindUnavailable, "INFRA_DETECT_AGENTS_FAILED", "failed to detect installed AI agents", err)
    }
    return decodeDetectedAgents(result), nil
}

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
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestDetectDevServerAgents -v
```

Expected: fake `DevServerAgentClient.Exec` asserts called with
`"preflight.detectAgents"` and the exact 4-probe `commands` list; decodes a
fixture `{agents:["claude"],platform:"linux"}` response correctly; an `Exec`
error surfaces as `INFRA_DETECT_AGENTS_FAILED`, not a fabricated empty list.
