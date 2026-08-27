// Package opaclient adapts common/policy.Evaluator to project-service's
// project-role/global-admin authorization check — the only OPA query this
// service needs, so it's a thin, single-purpose wrapper rather than
// exposing the generic Evaluator.Decision signature straight into usecase/.
// Mirrors auth-service/annotation-service/task-service's own
// internal/adapter/opaclient shape.
package opaclient

import (
	"context"

	"github.com/stablyai/orca-go/common/policy"
)

// decisionQuery is the fully-qualified Rego rule this service evaluates —
// see backend-go/policy/orca-authz/project.rego.
const decisionQuery = "data.orca.authz.project.allow"

// Client evaluates project-service's project-role/global-admin
// authorization policy via the shared embedded-OPA evaluator
// (common/policy).
type Client struct {
	evaluator *policy.Evaluator
}

// New wraps evaluator for project-service's authorization check. evaluator
// is constructed once in cmd/server/main.go, pointed at config.Config's
// OPABundlePath, and shared across every requireProjectAccess call.
func New(evaluator *policy.Evaluator) *Client {
	return &Client{evaluator: evaluator}
}

// Decision reports whether callerProjectRole/callerGlobalRole authorizes
// action, per project.rego's {"caller_project_role", "caller_global_role",
// "action"} input contract.
func (c *Client) Decision(ctx context.Context, callerProjectRole, callerGlobalRole, action string) (bool, error) {
	return c.evaluator.Decision(ctx, decisionQuery, map[string]any{
		"caller_project_role": callerProjectRole,
		"caller_global_role":  callerGlobalRole,
		"action":              action,
	})
}
