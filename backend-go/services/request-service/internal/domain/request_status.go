package domain

type RequestStatus string

const (
	RequestStatusNew                      RequestStatus = "new"
	RequestStatusClassifying              RequestStatus = "classifying"
	RequestStatusAwaitingTypeConfirmation RequestStatus = "awaiting_type_confirmation"
	RequestStatusAnalyzing                RequestStatus = "analyzing"
	RequestStatusAwaitingAnalysisApproval RequestStatus = "awaiting_analysis_approval"
	RequestStatusPlanning                 RequestStatus = "planning"
	RequestStatusAwaitingPlanApproval     RequestStatus = "awaiting_plan_approval"
	RequestStatusExecuting                RequestStatus = "executing"
	RequestStatusCompleted                RequestStatus = "completed"
	RequestStatusRequestBacklog           RequestStatus = "request_backlog"
	RequestStatusCancelled                RequestStatus = "cancelled"
	// RequestStatusAwaitingInformation parks a request while a Clarification is open (CR-REQ-028).
	RequestStatusAwaitingInformation RequestStatus = "awaiting_information"
)

func AllRequestStatuses() []RequestStatus {
	return []RequestStatus{
		RequestStatusNew,
		RequestStatusClassifying,
		RequestStatusAwaitingTypeConfirmation,
		RequestStatusAnalyzing,
		RequestStatusAwaitingAnalysisApproval,
		RequestStatusPlanning,
		RequestStatusAwaitingPlanApproval,
		RequestStatusExecuting,
		RequestStatusCompleted,
		RequestStatusRequestBacklog,
		RequestStatusCancelled,
		RequestStatusAwaitingInformation,
	}
}

func ParseRequestStatus(s string) (RequestStatus, error) {
	for _, rs := range AllRequestStatuses() {
		if string(rs) == s {
			return rs, nil
		}
	}
	return "", ErrRequestInvalidStatus(s)
}

func (s RequestStatus) IsTerminal() bool {
	return s == RequestStatusCompleted || s == RequestStatusCancelled
}

// IsHumanWaiting is true for the statuses where the next move belongs to a person.
func (s RequestStatus) IsHumanWaiting() bool {
	switch s {
	case RequestStatusAwaitingTypeConfirmation, RequestStatusAwaitingAnalysisApproval, RequestStatusAwaitingPlanApproval, RequestStatusAwaitingInformation:
		return true
	}
	return false
}
