package domain

// StageForStatus lists the return stages valid for a request currently in status.
// phase is only valid when the type's flow splits this request into Phases.
func StageForStatus(status RequestStatus, flow FlowDefinition, size RequestSize) ([]ReturnStage, error) {
	switch status {
	case RequestStatusClassifying, RequestStatusAwaitingTypeConfirmation:
		return []ReturnStage{ReturnStageClassification}, nil
	case RequestStatusAnalyzing, RequestStatusAwaitingAnalysisApproval:
		return []ReturnStage{ReturnStageAnalysis}, nil
	case RequestStatusPlanning, RequestStatusAwaitingPlanApproval:
		return []ReturnStage{ReturnStagePlan}, nil
	case RequestStatusAwaitingInformation:
		// The Clarification knows where the request came from; the caller picks the stage and the table only bounds it.
		out := []ReturnStage{ReturnStageClassification, ReturnStageAnalysis, ReturnStagePlan}
		if flow.PhasesFor(size) {
			out = append(out, ReturnStagePhase)
		}
		return append(out, ReturnStageTask), nil
	case RequestStatusExecuting:
		if flow.PhasesFor(size) {
			return []ReturnStage{ReturnStagePhase, ReturnStageTask}, nil
		}
		return []ReturnStage{ReturnStageTask}, nil
	}
	return nil, ErrTransitionNotAllowed(status, TriggerReturnToBacklog)
}

// ContainsStage reports whether stage is one of stages.
func ContainsStage(stages []ReturnStage, stage ReturnStage) bool {
	for _, s := range stages {
		if s == stage {
			return true
		}
	}
	return false
}
