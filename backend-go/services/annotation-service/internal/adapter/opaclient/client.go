// Package opaclient adapts common/policy.Evaluator to
// annotation-service's specific author-only edit/delete check — the only
// OPA query this service ever needs, so it's a thin, single-purpose
// wrapper rather than exposing the generic Evaluator.Decision signature
// straight into usecase/.
package opaclient

import (
	"context"

	"github.com/stablyai/orca-go/common/policy"
)

// decisionQuery is the fully-qualified Rego rule this service evaluates —
// see backend-go/policy/orca-authz/annotation.rego.
const decisionQuery = "data.orca.authz.annotation.allow"

// Client evaluates annotation-service's author-only edit/delete policy
// via the shared embedded-OPA evaluator (common/policy).
type Client struct {
	evaluator *policy.Evaluator
}

// New wraps evaluator for annotation-service's authorization check.
func New(evaluator *policy.Evaluator) *Client {
	return &Client{evaluator: evaluator}
}

// Decision reports whether actorID may edit/delete an annotation authored
// by authorID, given actorRole. Per annotation.rego, allow requires
// actorID == authorID OR actorRole == "admin". actorRole should be passed
// as "" when the caller's role isn't known — see this service's README
// "Known gaps": no role claim is propagated into annotation-service's
// request context yet, so the admin-override branch never fires until
// that lands upstream.
func (c *Client) Decision(ctx context.Context, actorID, authorID, actorRole string) (bool, error) {
	return c.evaluator.Decision(ctx, decisionQuery, map[string]any{
		"actor_id":   actorID,
		"author_id":  authorID,
		"actor_role": actorRole,
	})
}
