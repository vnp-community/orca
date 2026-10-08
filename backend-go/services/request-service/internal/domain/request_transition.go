package domain

func statusIn(s RequestStatus, set ...RequestStatus) bool {
	for _, x := range set {
		if s == x {
			return true
		}
	}
	return false
}

// inProgressStatuses are the seven statuses a request can be returned to the backlog from.
var inProgressStatuses = []RequestStatus{
	RequestStatusClassifying, RequestStatusAwaitingTypeConfirmation, RequestStatusAnalyzing,
	RequestStatusAwaitingAnalysisApproval, RequestStatusPlanning, RequestStatusAwaitingPlanApproval, RequestStatusExecuting,
}

// informationSourceStatuses may open a Clarification (CR-REQ-028 section 2.1).
var informationSourceStatuses = []RequestStatus{
	RequestStatusAwaitingTypeConfirmation, RequestStatusAnalyzing, RequestStatusAwaitingAnalysisApproval,
	RequestStatusPlanning, RequestStatusAwaitingPlanApproval, RequestStatusExecuting,
}

// resumeStatuses are the only places information_provided may land.
var resumeStatuses = []RequestStatus{RequestStatusAnalyzing, RequestStatusPlanning, RequestStatusExecuting}

// IsResumeStatus reports whether s is a legal resume target of a Clarification.
func IsResumeStatus(s RequestStatus) bool { return statusIn(s, resumeStatuses...) }

// NextStatusWithResume wraps NextStatus for information_provided, whose destination is the
// Clarification's resume status and so cannot be a pure function of (from, trigger).
// NextStatus keeps its signature and refuses that trigger.
func NextStatusWithResume(flow FlowDefinition, size RequestSize, from RequestStatus, trigger Trigger, resume RequestStatus) (RequestStatus, error) {
	if trigger != TriggerInformationProvided {
		return NextStatus(flow, size, from, trigger)
	}
	if !allowedFrom(trigger, from) {
		return "", ErrTransitionNotAllowed(from, trigger)
	}
	if !IsResumeStatus(resume) {
		return "", ErrResumeStatusInvalid(resume)
	}
	return resume, nil
}

// NextStatus is the whole transition table (CR-REQ-003 section 2.2); it is pure so the
// 12 statuses x 18 triggers matrix can be tested without a database.
// size is part of the signature so later CRs can branch on it without touching callers.
func NextStatus(flow FlowDefinition, size RequestSize, from RequestStatus, trigger Trigger) (RequestStatus, error) {
	_ = size
	deny := func() (RequestStatus, error) { return "", ErrTransitionNotAllowed(from, trigger) }
	if from.IsTerminal() {
		return deny()
	}
	if needsFlow(trigger) && flow.Type == "" {
		if allowedFrom(trigger, from) {
			return "", ErrTypeNotSet()
		}
		return deny()
	}
	if !allowedFrom(trigger, from) {
		return deny()
	}
	switch trigger {
	case TriggerStartClassification:
		return RequestStatusClassifying, nil
	case TriggerProposalReady:
		return RequestStatusAwaitingTypeConfirmation, nil
	case TriggerTypeConfirmed:
		if flow.AnalysisKind != AnalysisNone {
			return RequestStatusAnalyzing, nil
		}
		return RequestStatusPlanning, nil
	case TriggerAnalysisReady:
		switch {
		case flow.AnalysisGate != GateNone:
			return RequestStatusAwaitingAnalysisApproval, nil
		case flow.PlanKind == PlanSingleTask:
			return RequestStatusAwaitingPlanApproval, nil
		default:
			return RequestStatusPlanning, nil
		}
	case TriggerAnalysisApproved:
		if flow.CompletesAfterAnalysis {
			return RequestStatusCompleted, nil
		}
		return RequestStatusPlanning, nil
	case TriggerAnalysisRevision:
		return RequestStatusAnalyzing, nil
	case TriggerPlanReady:
		return RequestStatusAwaitingPlanApproval, nil
	case TriggerPlanApproved:
		return RequestStatusExecuting, nil
	case TriggerPlanRevision:
		return RequestStatusPlanning, nil
	case TriggerExecutionFinished:
		return RequestStatusCompleted, nil
	case TriggerAnalysisRejected, TriggerPlanRejected, TriggerReturnToBacklog:
		return RequestStatusRequestBacklog, nil
	case TriggerTypeChange:
		return RequestStatusAwaitingTypeConfirmation, nil
	case TriggerReopen:
		return RequestStatusClassifying, nil
	case TriggerCancel:
		return RequestStatusCancelled, nil
	case TriggerInformationRequired:
		return RequestStatusAwaitingInformation, nil
	}
	return deny()
}

// allowedFrom lists, per trigger, the statuses it may start from.
func allowedFrom(trigger Trigger, from RequestStatus) bool {
	switch trigger {
	case TriggerStartClassification:
		return from == RequestStatusNew
	case TriggerProposalReady:
		return from == RequestStatusClassifying
	case TriggerTypeConfirmed:
		return from == RequestStatusAwaitingTypeConfirmation
	case TriggerAnalysisReady:
		return from == RequestStatusAnalyzing
	case TriggerAnalysisApproved, TriggerAnalysisRejected, TriggerAnalysisRevision:
		return from == RequestStatusAwaitingAnalysisApproval
	case TriggerPlanReady:
		return from == RequestStatusPlanning
	case TriggerPlanApproved, TriggerPlanRejected, TriggerPlanRevision:
		return from == RequestStatusAwaitingPlanApproval
	case TriggerExecutionFinished:
		return from == RequestStatusExecuting
	case TriggerReturnToBacklog:
		return statusIn(from, inProgressStatuses...) || from == RequestStatusAwaitingInformation
	case TriggerTypeChange:
		return statusIn(from, inProgressStatuses[1:]...) || from == RequestStatusAwaitingInformation
	case TriggerReopen:
		return from == RequestStatusRequestBacklog
	case TriggerCancel:
		return !from.IsTerminal()
	case TriggerInformationRequired:
		return statusIn(from, informationSourceStatuses...)
	case TriggerInformationProvided:
		return from == RequestStatusAwaitingInformation
	}
	return false
}
