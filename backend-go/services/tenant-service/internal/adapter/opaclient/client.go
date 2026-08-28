// Package opaclient adapts common/policy.Evaluator to tenant-service's
// caller-role authorization check. Mirrors project-service/auth-service/
// annotation-service/task-service's own internal/adapter/opaclient shape.
package opaclient

import (
	"context"

	"github.com/stablyai/orca-go/common/policy"
)

// decisionQuery is the fully-qualified Rego rule this service evaluates —
// see backend-go/policy/orca-authz/tenant.rego.
const decisionQuery = "data.orca.authz.tenant.allow"

// Client evaluates tenant-service's role-based authorization policy via the
// shared embedded-OPA evaluator (common/policy).
type Client struct {
	evaluator *policy.Evaluator
}

// New wraps evaluator for tenant-service's authorization check. evaluator is
// constructed once in cmd/server/main.go, pointed at config.Config's
// OPABundlePath, and shared across every decide call.
func New(evaluator *policy.Evaluator) *Client {
	return &Client{evaluator: evaluator}
}

func (c *Client) Decision(ctx context.Context, callerRole, action string, sameDepartment bool) (bool, error) {
	return c.evaluator.Decision(ctx, decisionQuery, map[string]any{
		"caller_role":     callerRole,
		"action":          action,
		"same_department": sameDepartment,
	})
}
