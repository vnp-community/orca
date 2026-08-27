// Package opaclient implements usecase.OPAClient by wrapping the shared,
// embedded common/policy.Evaluator against
// backend-go/policy/orca-authz/task_grant.rego's data.orca.authz.task.allow
// rule — task-service.md §9's "OPA's job is the generic 'does this level
// authorize this action' decision" half of the grant-resolution split.
// Mirrors annotation-service's internal/adapter/opaclient shape: a thin,
// single-purpose wrapper around the generic Evaluator rather than exposing
// Evaluator.Decision straight into usecase/.
package opaclient

import (
	"context"

	"github.com/stablyai/orca-go/common/policy"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// decisionQuery is the fully-qualified Rego rule this service evaluates.
const decisionQuery = "data.orca.authz.task.allow"

// grantLevelToRegoString maps domain.GrantLevel to the lowercase strings
// task_grant.rego's level_actions table keys on. domain.GrantLevelUnspecified
// (the BFS walk's not-found sentinel — see grant_resolution.go) has no
// entry: ResolvePermission.Execute never calls Decision with it (it returns
// its own PermissionDenied before reaching this call), but the fallback
// below still sends "unspecified" — a level_actions miss, always denied —
// so this adapter fails closed rather than panicking if that invariant is
// ever violated.
var grantLevelToRegoString = map[domain.GrantLevel]string{
	domain.GrantLevelOwner:   "owner",
	domain.GrantLevelAdmin:   "admin",
	domain.GrantLevelUser:    "user",
	domain.GrantLevelTeam:    "team",
	domain.GrantLevelCompany: "company",
}

// Client evaluates task-service's grant-level/action authorization policy
// via the shared embedded-OPA evaluator (common/policy).
type Client struct {
	evaluator *policy.Evaluator
}

// New wraps evaluator for task-service's authorization check. evaluator is
// constructed once in cmd/server/main.go, pointed at config.Config's
// OPABundlePath, and shared across every ResolvePermission call.
func New(evaluator *policy.Evaluator) *Client {
	return &Client{evaluator: evaluator}
}

// Decision reports whether level authorizes action for tenantID, per
// task_grant.rego's input contract
// ({"level": ..., "action": ..., "tenant_id": ...}).
func (c *Client) Decision(ctx context.Context, level domain.GrantLevel, action, tenantID string) (bool, error) {
	levelStr, ok := grantLevelToRegoString[level]
	if !ok {
		levelStr = "unspecified"
	}
	return c.evaluator.Decision(ctx, decisionQuery, map[string]any{
		"level":     levelStr,
		"action":    action,
		"tenant_id": tenantID,
	})
}
