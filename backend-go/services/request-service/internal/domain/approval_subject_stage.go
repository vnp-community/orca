package domain

// ApprovalStatusesFor lists the Request statuses in which an approval of this subject may be open.
// Approval.Stage stores the status it was opened in, so deciding after the Request moved on is detectable.
func ApprovalStatusesFor(st SubjectType) []RequestStatus {
	switch st {
	case SubjectRequestType:
		return []RequestStatus{RequestStatusAwaitingTypeConfirmation}
	case SubjectSolution, SubjectFindings, SubjectAnswer:
		return []RequestStatus{RequestStatusAwaitingAnalysisApproval}
	case SubjectPlan, SubjectTaskList:
		return []RequestStatus{RequestStatusAwaitingPlanApproval}
	case SubjectPreDeploy:
		return []RequestStatus{RequestStatusAwaitingPlanApproval, RequestStatusExecuting}
	case SubjectPhase:
		return []RequestStatus{RequestStatusExecuting}
	}
	return nil
}

// ApprovalAllowedInStatus reports whether subject st may be approved while the Request is in status.
func ApprovalAllowedInStatus(st SubjectType, status RequestStatus) bool {
	for _, s := range ApprovalStatusesFor(st) {
		if s == status {
			return true
		}
	}
	return false
}

// ApprovalSubjectAllowedByFlow checks the subject against the Request type's gates (README v6 section 3.4).
// A Request without a confirmed type only has the request_type gate.
func ApprovalSubjectAllowedByFlow(flow FlowDefinition, size RequestSize, hasType bool, st SubjectType) bool {
	if st == SubjectRequestType {
		return true
	}
	if !hasType {
		return false
	}
	switch st {
	case SubjectSolution:
		return flow.AnalysisGate == GateSolution
	case SubjectFindings:
		return flow.AnalysisGate == GateFindings
	case SubjectAnswer:
		return flow.AnalysisGate == GateAnswer
	case SubjectPlan:
		return flow.StartGate == GatePlan
	case SubjectTaskList:
		return flow.StartGate == GateTaskList
	case SubjectPreDeploy:
		return flow.StartGate == GatePreDeploy || flow.HasExecutionGate(GatePreDeploy)
	case SubjectPhase:
		return flow.HasExecutionGate(GatePhase) || flow.PhasesFor(size)
	}
	return false
}

// ApprovalReturnStage is the returned_from_stage used when an approval of this subject expires or is rejected.
// Inside executing the stage is "phase" only when the flow splits the Request into phases (see StageForStatus), else "task".
func ApprovalReturnStage(st SubjectType, status RequestStatus, req Request) ReturnStage {
	switch st {
	case SubjectRequestType:
		return ReturnStageClassification
	case SubjectSolution, SubjectFindings, SubjectAnswer:
		return ReturnStageAnalysis
	case SubjectPlan, SubjectTaskList:
		return ReturnStagePlan
	}
	if status != RequestStatusExecuting {
		return ReturnStagePlan // pre_deploy as the start gate
	}
	if flow, err := FlowFor(req.Type); err == nil && flow.PhasesFor(req.Size) {
		return ReturnStagePhase
	}
	return ReturnStageTask
}

// ApprovalSubjectLabel is the short English noun used in notification titles; the UI localizes its own copy.
func ApprovalSubjectLabel(st SubjectType) string {
	switch st {
	case SubjectRequestType:
		return "Request type"
	case SubjectSolution:
		return "Solution"
	case SubjectFindings:
		return "Findings"
	case SubjectAnswer:
		return "Answer"
	case SubjectPlan:
		return "Plan"
	case SubjectPhase:
		return "Phase"
	case SubjectTaskList:
		return "Task list"
	case SubjectPreDeploy:
		return "Pre-deploy"
	}
	return string(st)
}
