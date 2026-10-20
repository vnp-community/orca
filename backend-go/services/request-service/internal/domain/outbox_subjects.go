package domain

const (
	SubjectRequestCreated       = "orca.request.request.created"
	SubjectRequestStatusChanged = "orca.request.request.status_changed"
	SubjectRequestClassified    = "orca.request.request.classified"
	SubjectRequestTypeConfirmed = "orca.request.request.type_confirmed"
	SubjectRequestTypeChanged   = "orca.request.request.type_changed"
	// SubjectRequestBaseRefChanged carries ids and the old and new ref names, never the Request title or body (CR-REQ-039).
	SubjectRequestBaseRefChanged = "orca.request.request.base_ref_changed"
	SubjectRequestReturned       = "orca.request.request.returned"
	SubjectRequestCompleted      = "orca.request.request.completed"
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

// SubjectPlanGenerated is emitted when a Plan has been written to task-service and sent for approval.
const SubjectPlanGenerated = "orca.request.plan.generated"

// Outcome Review subjects. Payloads carry ids and enums only, never review text, so a subscriber cannot read content it has no right to.
const (
	SubjectReviewGenerated           = "orca.request.review.generated"
	SubjectReviewArtifactRegenerated = "orca.request.review.artifact_regenerated"
	SubjectReviewDecided             = "orca.request.review.decided"
	// SubjectReviewProgress is a run starting or ending; it feeds the live progress line and nothing else.
	SubjectReviewProgress = "orca.request.review.progress"
)
