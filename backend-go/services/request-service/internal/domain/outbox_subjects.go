package domain

const (
	SubjectRequestCreated       = "orca.request.request.created"
	SubjectRequestStatusChanged = "orca.request.request.status_changed"
	SubjectRequestClassified    = "orca.request.request.classified"
	SubjectRequestTypeConfirmed = "orca.request.request.type_confirmed"
	SubjectRequestTypeChanged   = "orca.request.request.type_changed"
	SubjectRequestReturned      = "orca.request.request.returned"
	SubjectRequestCompleted     = "orca.request.request.completed"
)

// CR-REQ-027 / CR-REQ-028 subjects. Payloads never carry Request content, questions or answers.
const (
	SubjectRequestRevised         = "orca.request.request.revised"
	SubjectClarificationRequested = "orca.request.clarification.requested"
	SubjectClarificationAnswered  = "orca.request.clarification.answered"
	SubjectClarificationExpired   = "orca.request.clarification.expired"
	SubjectClarificationCancelled = "orca.request.clarification.cancelled"
	SubjectDecisionRecorded       = "orca.request.decision.recorded"
	SubjectDecisionConfirmed      = "orca.request.decision.confirmed"
)
