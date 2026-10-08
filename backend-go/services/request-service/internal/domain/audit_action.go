package domain

// Audit actions recorded in auth-service's audit log (CR-REQ-024 section 2.8).
// request.jira.sync is written by issue-status-sync, not here.
const (
	ActionRequestCreate      = "request.create"
	ActionRequestTypeConfirm = "request.type.confirm"
	ActionRequestTypeChange  = "request.type.change"
	ActionRequestReturn      = "request.return"
	ActionRequestReopen      = "request.reopen"
	ActionRequestCancel      = "request.cancel"
	ActionSolutionChoose     = "solution.choose"
	ActionApprovalApprove    = "approval.approve"
	ActionApprovalReject     = "approval.reject"
	ActionApprovalCancel     = "approval.cancel"
	ActionApprovalExpire     = "approval.expire"
	ActionRequestFlowSet     = "request.flow.set"
)

// AuditActorKind matches the user|agent|system CHECK on auth.audit_log.actor_type.
type AuditActorKind string

const (
	AuditActorUser   AuditActorKind = "user"
	AuditActorAgent  AuditActorKind = "agent"
	AuditActorSystem AuditActorKind = "system"
)

const (
	AuditOutcomeAllowed = "allowed"
	AuditOutcomeDenied  = "denied"
)
