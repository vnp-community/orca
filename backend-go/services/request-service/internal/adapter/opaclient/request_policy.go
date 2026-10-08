package opaclient

import (
	"context"

	"github.com/stablyai/orca-go/common/policy"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const requestDecisionQuery = "data.orca.authz.request.allow"

// RequestPolicy evaluates authorization rules for request-service operations.
type RequestPolicy struct {
	evaluator *policy.Evaluator
}

// NewRequestPolicy creates a new RequestPolicy that loads Rego bundles from bundlePath.
func NewRequestPolicy(bundlePath string) *RequestPolicy {
	evaluator := policy.NewEvaluator(bundlePath)
	return &RequestPolicy{evaluator: evaluator}
}

// Warm evaluates the main decision query once to parse and cache the rules
// and verify the bundle is complete. Fails if the query doesn't exist.
func (p *RequestPolicy) Warm(ctx context.Context) error {
	return p.evaluator.Warm(ctx, requestDecisionQuery)
}

// RequestPolicyInput contains the parameters passed to the Rego policy.
type RequestPolicyInput struct {
	Action            string `json:"action"`
	RPC               string `json:"rpc"`
	CallerGlobalRole  string `json:"caller_global_role"`
	CallerProjectRole string `json:"caller_project_role"`
	IsReporter        bool   `json:"is_reporter"`
	ActorType         string `json:"actor_type"`
}

// Decision returns true if the policy allows the operation.
func (p *RequestPolicy) Decision(ctx context.Context, in RequestPolicyInput) (bool, error) {
	input := map[string]any{
		"action":              in.Action,
		"rpc":                 in.RPC,
		"caller_global_role":  in.CallerGlobalRole,
		"caller_project_role": in.CallerProjectRole,
		"is_reporter":         in.IsReporter,
		"actor_type":          in.ActorType,
	}
	return p.evaluator.Decision(ctx, requestDecisionQuery, input)
}

// Allow adapts domain.AccessInput to the Rego input; an evaluation error is never an allow.
func (p *RequestPolicy) Allow(ctx context.Context, in domain.AccessInput) (bool, error) {
	ok, err := p.Decision(ctx, RequestPolicyInput{
		Action: string(in.Action), RPC: in.RPC, CallerGlobalRole: in.CallerGlobalRole,
		CallerProjectRole: in.CallerProjectRole, IsReporter: in.IsReporter, ActorType: in.ActorType,
	})
	if err != nil {
		return false, err
	}
	return ok, nil
}
