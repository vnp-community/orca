package domain

// Trigger names the event that moves a Request between statuses; TransitionRequest is the only writer of status.
type Trigger string

const (
	TriggerStartClassification Trigger = "start_classification"
	TriggerProposalReady       Trigger = "proposal_ready"
	TriggerTypeConfirmed       Trigger = "type_confirmed"
	TriggerAnalysisReady       Trigger = "analysis_ready"
	TriggerAnalysisApproved    Trigger = "analysis_approved"
	TriggerAnalysisRejected    Trigger = "analysis_rejected"
	TriggerAnalysisRevision    Trigger = "analysis_revision"
	TriggerPlanReady           Trigger = "plan_ready"
	TriggerPlanApproved        Trigger = "plan_approved"
	TriggerPlanRejected        Trigger = "plan_rejected"
	TriggerPlanRevision        Trigger = "plan_revision"
	TriggerExecutionFinished   Trigger = "execution_finished"
	TriggerReturnToBacklog     Trigger = "return_to_backlog"
	TriggerTypeChange          Trigger = "type_change"
	TriggerReopen              Trigger = "reopen"
	TriggerCancel              Trigger = "cancel"
	// TriggerInformationRequired and TriggerInformationProvided open and close a Clarification (CR-REQ-028).
	TriggerInformationRequired Trigger = "information_required"
	TriggerInformationProvided Trigger = "information_provided"
)

func AllTriggers() []Trigger {
	return []Trigger{
		TriggerStartClassification, TriggerProposalReady, TriggerTypeConfirmed, TriggerAnalysisReady, TriggerAnalysisApproved,
		TriggerAnalysisRejected, TriggerAnalysisRevision, TriggerPlanReady, TriggerPlanApproved, TriggerPlanRejected,
		TriggerPlanRevision, TriggerExecutionFinished, TriggerReturnToBacklog, TriggerTypeChange, TriggerReopen, TriggerCancel,
		TriggerInformationRequired, TriggerInformationProvided,
	}
}

// RequiresReason is true for triggers that end in request_backlog or cancelled.
func RequiresReason(t Trigger) bool {
	switch t {
	case TriggerAnalysisRejected, TriggerPlanRejected, TriggerReturnToBacklog, TriggerCancel:
		return true
	}
	return false
}

// TargetsBacklog is true for triggers that land in request_backlog (they carry stage and category).
func TargetsBacklog(t Trigger) bool {
	switch t {
	case TriggerAnalysisRejected, TriggerPlanRejected, TriggerReturnToBacklog:
		return true
	}
	return false
}

// needsFlow is true when the destination depends on the type's FlowDefinition.
func needsFlow(t Trigger) bool {
	switch t {
	case TriggerTypeConfirmed, TriggerAnalysisReady, TriggerAnalysisApproved:
		return true
	}
	return false
}
