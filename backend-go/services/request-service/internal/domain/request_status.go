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
