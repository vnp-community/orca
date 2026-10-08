package domain

// Audit actions of CR-REQ-035 and CR-REQ-024 2.8.
const (
	AuditRequestReadDenied   = "request.read.denied"
	AuditRequestAccessDenied = "request.access.denied"
	AuditRequestExport       = "request.export"
	AuditRequestErase        = "request.erase"
	AuditRetentionRun        = "request.retention.run"
	AuditAIBudgetSet         = "ai.budget.set"
	AuditAIEgressSet         = "ai.egress.set"
	AuditApprovalApprove     = "approval.approve"
)

// DurableAuditActions must survive an auth-service outage: they are written to the audit outbox in the
// same transaction as the change. approval.approve is durable only for the pre_deploy gate (see IsDurableAudit).
var DurableAuditActions = map[string]bool{
	AuditRequestExport: true, AuditRequestErase: true, AuditAIEgressSet: true, AuditRetentionRun: true,
}

// IsDurableAudit tells whether an action needs the outbox; subjectType matters only for approvals.
func IsDurableAudit(action, subjectType string) bool {
	if action == AuditApprovalApprove {
		return subjectType == "pre_deploy"
	}
	return DurableAuditActions[action]
}
