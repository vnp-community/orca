// Package opaclient adapts common/policy.Evaluator to auth-service's
// requireAdminActor check — the only OPA query this service needs, so it's
// a thin, single-purpose wrapper rather than exposing the generic
// Evaluator.Decision signature straight into usecase/. Mirrors
// task-service/annotation-service's own internal/adapter/opaclient shape.
package opaclient

import (
	"context"

	"github.com/stablyai/orca-go/common/policy"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// decisionQuery is the fully-qualified Rego rule this service evaluates —
// see backend-go/policy/orca-authz/admin.rego.
const decisionQuery = "data.orca.authz.admin.allow"

// Client evaluates auth-service's admin-actor policy via the shared
// embedded-OPA evaluator (common/policy).
type Client struct {
	evaluator *policy.Evaluator
}

// New wraps evaluator for auth-service's authorization check. evaluator is
// constructed once in cmd/server/main.go, pointed at config.Config's
// OPABundlePath, and shared across every requireAdminActor call.
func New(evaluator *policy.Evaluator) *Client {
	return &Client{evaluator: evaluator}
}

// Decision reports whether actor is authorized as admin, per admin.rego's
// {"actor": {"id", "role"}} input contract — role is sent as its literal
// string form (domain.RoleAdmin == "admin"), matching what the policy
// compares against.
func (c *Client) Decision(ctx context.Context, actor domain.User) (bool, error) {
	return c.evaluator.Decision(ctx, decisionQuery, map[string]any{
		"actor": map[string]any{
			"id":   actor.ID,
			"role": string(actor.Role),
		},
	})
}
