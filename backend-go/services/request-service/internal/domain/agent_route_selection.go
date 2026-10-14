package domain

import "errors"

type ReadonlyRoute string

const (
	RouteAgentEnforced ReadonlyRoute = "agent_enforced"
	RouteHonestyBound  ReadonlyRoute = "honesty_bound"
)

// FeatureKnowledgeHub mirrors infra-fleet's domain.FeatureKnowledgeHub; the agent declares it after the code exists.
const FeatureKnowledgeHub = "agent.execPrompt.knowledgeHub"

var ErrReadonlyUnsupported = errors.New("readonly unsupported by agent")

func SelectReadonlyRoute(c DevServerCapability, requireEnforced bool) (ReadonlyRoute, error) {
	if c.HasFeature("agent.execPrompt.readonly") {
		return RouteAgentEnforced, nil
	}

	if requireEnforced {
		return "", ErrReadonlyUnsupported
	}

	// For old agent: if version < 5.0.0, we just assume it's honesty bound
	// But agent old version is not explicitly checked, since we rely on capabilities.
	return RouteHonestyBound, nil
}

// SupportsKnowledgeHub reports whether the agent can reach a project's tdd-agent knowledge graph (agent.execPrompt.knowledgeHub).
// Callers must check it before setting AgentPromptInput.KnowledgeRoot: an older agent ignores the unknown key and would
// run without the knowledge graph, hiding the loss.
func SupportsKnowledgeHub(c DevServerCapability) bool {
	return c.HasFeature(FeatureKnowledgeHub)
}
