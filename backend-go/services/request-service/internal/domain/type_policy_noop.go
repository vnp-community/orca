package domain

import "context"

// noopPolicy keeps bug, task, docs, change_request, spike and question on the standard path.
type noopPolicy struct{}

func (noopPolicy) PlanPreconditions(context.Context, Request, PlanProposal) error { return nil }
func (noopPolicy) PreExecutionGate(context.Context, Request, TaskRef) (*GateRequirement, error) {
	return nil, nil
}
func (noopPolicy) CompletionChecks(Request, []RequestCheck) ([]CheckVerdict, error) { return nil, nil }
func (noopPolicy) OnCompleted(Request) ([]FollowUp, error)                          { return nil, nil }
