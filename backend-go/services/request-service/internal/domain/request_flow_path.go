package domain

// PathStep is one hop of the standard path.
type PathStep struct {
	Trigger Trigger
	To      RequestStatus
}

// standardTrigger is the happy-path trigger at each status.
var standardTrigger = map[RequestStatus]Trigger{
	RequestStatusNew:                      TriggerStartClassification,
	RequestStatusClassifying:              TriggerProposalReady,
	RequestStatusAwaitingTypeConfirmation: TriggerTypeConfirmed,
	RequestStatusAnalyzing:                TriggerAnalysisReady,
	RequestStatusAwaitingAnalysisApproval: TriggerAnalysisApproved,
	RequestStatusPlanning:                 TriggerPlanReady,
	RequestStatusAwaitingPlanApproval:     TriggerPlanApproved,
	RequestStatusExecuting:                TriggerExecutionFinished,
}

// HappyPathSteps simulates the standard trigger sequence through NextStatus so the
// path never drifts from the transition table. It excludes backlog, type change and cancel.
func HappyPathSteps(flow FlowDefinition, size RequestSize) ([]PathStep, error) {
	var steps []PathStep
	cur := RequestStatusNew
	for i := 0; cur != RequestStatusCompleted; i++ {
		trig, ok := standardTrigger[cur]
		if !ok || i > len(AllRequestStatuses()) {
			return nil, ErrTransitionNotAllowed(cur, "")
		}
		next, err := NextStatus(flow, size, cur, trig)
		if err != nil {
			return nil, err
		}
		steps = append(steps, PathStep{Trigger: trig, To: next})
		cur = next
	}
	return steps, nil
}

// HappyPath lists the statuses from new to completed.
func HappyPath(flow FlowDefinition, size RequestSize) ([]RequestStatus, error) {
	steps, err := HappyPathSteps(flow, size)
	if err != nil {
		return nil, err
	}
	out := []RequestStatus{RequestStatusNew}
	for _, s := range steps {
		out = append(out, s.To)
	}
	return out, nil
}
