package wscompat

import (
	"context"
	"encoding/json"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// DecisionView is what the CLI and the UI read of a recorded decision (CR-REQ-028).
type DecisionView struct {
	ID                string `json:"id"`
	DisplayID         string `json:"displayId"`
	RequestID         string `json:"requestId"`
	SubjectKind       string `json:"subjectKind"`
	SubjectID         string `json:"subjectId"`
	ChosenOptionID    string `json:"chosenOptionId,omitempty"`
	ChosenOptionTitle string `json:"chosenOptionTitle,omitempty"`
	RiskLevel         string `json:"riskLevel,omitempty"`
	ConfirmedBy       string `json:"confirmedBy,omitempty"`
	Status            string `json:"status"`
	Version           int64  `json:"version"`
}

func decisionViewOf(d *requestv1.Decision) DecisionView {
	v := DecisionView{ID: d.GetId(), DisplayID: d.GetDisplayId(), RequestID: d.GetRequestId(), SubjectKind: d.GetSubjectKind(), SubjectID: d.GetSubjectId(),
		ChosenOptionID: d.GetChosenOptionId(), RiskLevel: d.GetRiskLevel(), ConfirmedBy: d.GetConfirmedBy(),
		Status: enumLower(d.GetStatus().String(), "DECISION_STATUS_"), Version: d.GetVersion()}
	var opts []struct {
		ID    string `json:"id"`
		Label string `json:"label"` // a decision stores each option's title under "label"
	}
	if json.Unmarshal([]byte(d.GetOptionsJson()), &opts) == nil {
		for _, o := range opts {
			if o.ID == v.ChosenOptionID {
				v.ChosenOptionTitle = o.Label
			}
		}
	}
	return v
}

// registerDecisions exposes the decision log to a person: reading it, and confirming a high risk choice by typing the chosen
// option's title. The RPC says "human only": it is a person's act, so it has no MCP tool and no channel an agent session is
// given by default; a person (or someone the person authorised) drives it from the CLI or the UI.
func (c requestChannels) registerDecisions(r *Registry) {
	opts := requestChannelOpts{timeout: requestRPCTimeout}
	c.handle(r, "decision.list", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			RequestID string `json:"requestId"`
		}](args)
		if err != nil {
			return nil, err
		}
		if in.RequestID == "" {
			return nil, invalidRequestArg("requestId is required")
		}
		resp, err := c.req.ListDecisions(ctx, &requestv1.ListDecisionsRequest{RequestId: in.RequestID, PageSize: 50})
		if err != nil {
			return nil, err
		}
		out := make([]DecisionView, 0, len(resp.GetDecisions()))
		for _, d := range resp.GetDecisions() {
			out = append(out, decisionViewOf(d))
		}
		return struct {
			Decisions []DecisionView `json:"decisions"`
		}{out}, nil
	})
	c.handle(r, "decision.confirm", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			DecisionID       string `json:"decisionId"`
			ConfirmationText string `json:"confirmationText"`
			ExpectedVersion  int64  `json:"expectedVersion"`
		}](args)
		if err != nil {
			return nil, err
		}
		if in.DecisionID == "" || in.ConfirmationText == "" {
			return nil, invalidRequestArg("decisionId and confirmationText are required")
		}
		resp, err := c.req.ConfirmDecision(ctx, &requestv1.ConfirmDecisionRequest{
			DecisionId: in.DecisionID, ConfirmationText: in.ConfirmationText, ExpectedVersion: in.ExpectedVersion,
		})
		if err != nil {
			return nil, err
		}
		return struct {
			Decision DecisionView `json:"decision"`
		}{decisionViewOf(resp.GetDecision())}, nil
	})
}
