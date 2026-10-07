package domain

import "errors"

type ReadonlyRoute string

const (
	RouteAgentEnforced ReadonlyRoute = "agent_enforced"
	RouteHonestyBound  ReadonlyRoute = "honesty_bound"
)

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
