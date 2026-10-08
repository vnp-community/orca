package domain

import (
	"context"
	"fmt"
)

// hotfixPolicy: the pre_deploy gate already sits in StartGate, so execution itself is ungated.
type hotfixPolicy struct{}

func (hotfixPolicy) PlanPreconditions(_ context.Context, _ Request, p PlanProposal) error {
	if len(p.Phases) != 0 || len(p.Tasks) != 1 {
		return ErrHotfixPlanShape("a hotfix plan is exactly one task and no phases")
	}
	return nil
}

func (hotfixPolicy) PreExecutionGate(context.Context, Request, TaskRef) (*GateRequirement, error) {
	return nil, nil
}

func (hotfixPolicy) CompletionChecks(Request, []RequestCheck) ([]CheckVerdict, error) {
	return nil, nil
}

// OnCompleted asks for a root-cause bug and a review task; ClientRequestID keeps repeats from duplicating them.
func (hotfixPolicy) OnCompleted(req Request) ([]FollowUp, error) {
	body := fmt.Sprintf("Follow-up of hotfix #%d (%s): %s", req.Number, req.ID, req.Title)
	return []FollowUp{
		{
			TypeHint: RequestTypeBug, Title: "Nguyên nhân gốc và test hồi quy cho " + req.Title, Body: body,
			LinkReason: LinkReasonFollowupHotfix, ClientRequestID: "hotfix-followup:" + req.ID + ":bug",
		},
		{
			TypeHint: RequestTypeTask, Title: "Review sau hotfix: " + req.Title, Body: body,
			LinkReason: LinkReasonFollowupHotfix, ClientRequestID: "hotfix-followup:" + req.ID + ":task",
		},
	}, nil
}
