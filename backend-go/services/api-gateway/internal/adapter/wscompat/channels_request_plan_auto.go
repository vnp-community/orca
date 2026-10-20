package wscompat

import (
	"context"
	"encoding/json"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// requestFeaturePlanAuto is published by request.flowStatus so a screen shows the automatic Plan states (writing, failed
// with Retry, could not start) only when this gateway serves request.retryPlan and autoPlan (CR-REQ-044). A screen that
// does not find it keeps the manual Generate button.
const requestFeaturePlanAuto = "request.planAuto"

// PlanAutoView is the autoPlan block of request.planProposal and the answer of request.retryPlan.
type PlanAutoView struct {
	Enabled bool `json:"enabled"`
	// State is pending, running, committing, done, gave_up, skipped or superseded; empty when no job applies.
	State         string `json:"state"`
	Origin        string `json:"origin"`
	Attempt       int32  `json:"attempt"`
	MaxAttempts   int32  `json:"maxAttempts"`
	ReasonCode    string `json:"reasonCode"`
	NextAttemptAt string `json:"nextAttemptAt"`
}

func planAutoViewOf(a *requestv1.PlanAutoState) PlanAutoView {
	return PlanAutoView{
		Enabled: a.GetEnabled(), State: a.GetState(), Origin: a.GetOrigin(), Attempt: a.GetAttempt(), MaxAttempts: a.GetMaxAttempts(),
		ReasonCode: a.GetReasonCode(), NextAttemptAt: a.GetNextAttemptAt(),
	}
}

// addPlanAutoView adds autoPlan only when request-service sent the block, so an older service leaves the key out and a
// screen reads that as "not supported".
func addPlanAutoView(out map[string]any, resp *requestv1.GetPlanProposalResponse) {
	if resp.GetAutoPlan() != nil {
		out["autoPlan"] = planAutoViewOf(resp.GetAutoPlan())
	}
}

// registerPlanAuto adds request.retryPlan, the way back after an automatic Plan failed. It has no MCP tool on purpose:
// an agent starts a Plan through request_generatePlan (see excluded_channels.yaml).
func (c requestChannels) registerPlanAuto(r *Registry) {
	c.handle(r, "request.retryPlan", requestChannelOpts{timeout: requestRPCTimeout}, c.retryPlan)
}

func (c requestChannels) retryPlan(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
	in, err := decodeRequestArgs[requestIDArgs](args)
	if err != nil {
		return nil, err
	}
	resp, err := c.req.RetryPlanGeneration(ctx, &requestv1.RetryPlanGenerationRequest{RequestId: in.ID})
	if err != nil {
		return nil, err
	}
	return struct {
		AutoPlan PlanAutoView `json:"autoPlan"`
	}{planAutoViewOf(resp.GetAutoPlan())}, nil
}
