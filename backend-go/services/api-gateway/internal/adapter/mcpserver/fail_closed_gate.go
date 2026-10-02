package mcpserver

import (
	"context"
	"encoding/json"
	"time"
)

// FailClosedGate is the PolicyGate used when no governance adapter is wired:
// only risk=read tools are allowed, everything else is denied and approvals
// never resolve. It also implements ToolPolicyView so non-read tools are
// hidden from tools/list instead of listed-then-denied.
type FailClosedGate struct{}

func (FailClosedGate) Decide(_ context.Context, _ Principal, meta ToolMeta, _ json.RawMessage) (GateDecision, error) {
	if meta.Risk == RiskRead {
		return GateDecision{Outcome: OutcomeAllow, Reasons: []string{"no policy gate wired: read-only default"}}, nil
	}
	return GateDecision{Outcome: OutcomeDeny, Reasons: []string{"no policy gate wired: only read tools are allowed"},
		Message: "This tool is disabled until a tool policy is configured."}, nil
}

func (FailClosedGate) AwaitApproval(context.Context, Principal, string) (bool, error) {
	return false, nil
}

func (FailClosedGate) Complete(context.Context, Principal, ToolMeta, string, time.Duration) {}

func (FailClosedGate) EffectiveDecision(_ context.Context, _ string, meta ToolMeta) (EffectiveDecision, error) {
	if meta.Risk == RiskRead {
		return EffectiveDecision{Decision: OutcomeAllow, Source: "default"}, nil
	}
	return EffectiveDecision{Decision: OutcomeDeny, Source: "default"}, nil
}
