package domain

import (
	"regexp"
	"strings"
	"time"
)

// Audit actions written through orca.mcp.audit.appended.
const (
	AuditActionToolCall       = "mcp.tool_call"
	AuditActionPolicyUpsert   = "mcp.policy.upsert"
	AuditActionPolicyDelete   = "mcp.policy.delete"
	AuditActionSettingsUpdate = "mcp.settings.update"
	AuditActionKillSwitchSet  = "mcp.killswitch.set"
	AuditActionApprovalDecide = "mcp.approval.decide"

	ActorAgent = "agent"
	ActorUser  = "user"

	MaxArgsSummary = 512
)

var whitespaceRun = regexp.MustCompile(`\s+`)

// SummarizeArgs makes the one-line, redacted, length-capped args summary kept
// in the journal and audit. Raw values are never stored.
func SummarizeArgs(preview string, r Redactor) string {
	s := whitespaceRun.ReplaceAllString(preview, " ")
	if r != nil {
		s, _ = r.Redact(s)
	}
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) > MaxArgsSummary {
		s = string(runes[:MaxArgsSummary])
	}
	return s
}

// AuditOutcome maps a journal decision to auth-service's closed enum; the
// detail stays in metadata.decision.
func AuditOutcome(decision string) string {
	if decision == CallAllow || decision == CallApproved {
		return "allowed"
	}
	return "denied"
}

// NewToolCallAuditEvent builds the single audit event of a finished call.
// The audit id is the call id so redelivery stays idempotent downstream.
func NewToolCallAuditEvent(eventID string, c ToolCall, at time.Time, suppressed int) (OutboxRecord, error) {
	meta := map[string]any{
		"call_id": c.ID, "client_id": c.ClientID, "client_name": c.ClientName, "mcp_session_id": c.SessionID,
		"risk": c.Risk, "decision": c.Decision, "reason_code": c.ReasonCode, "approval_id": c.ApprovalID,
		"approver": c.ApprovedBy, "args_summary": c.ArgsSummary, "args_hash": c.ParamsHash,
		"duration_ms": c.DurationMs,
	}
	if c.Result != "" {
		meta["result"] = c.Result
	}
	if suppressed > 0 {
		meta["suppressed_count"] = suppressed
	}
	if c.TraceID != "" { // lets an audit row be joined to the MCP request trace
		meta["trace_id"] = c.TraceID
	}
	return NewOutboxEvent(eventID, SubjectAuditAppended, c.TenantID, at, map[string]any{
		"audit_id": c.ID, "actor_id": c.UserID, "action": AuditActionToolCall, "actor_type": ActorAgent,
		"target_type": "mcp_tool", "target_id": c.ToolName, "outcome": AuditOutcome(c.Decision), "metadata": meta,
	})
}

// NewAdminAuditEvent builds an audit event for a human action (policy,
// settings, kill switch, approval decision). meta must already be free of secrets.
func NewAdminAuditEvent(eventID, auditID, tenantID, actorID, action, targetType, targetID, outcome string, at time.Time, meta map[string]any) (OutboxRecord, error) {
	if meta == nil {
		meta = map[string]any{}
	}
	return NewOutboxEvent(eventID, SubjectAuditAppended, tenantID, at, map[string]any{
		"audit_id": auditID, "actor_id": actorID, "action": action, "actor_type": ActorUser,
		"target_type": targetType, "target_id": targetID, "outcome": outcome, "metadata": meta,
	})
}
