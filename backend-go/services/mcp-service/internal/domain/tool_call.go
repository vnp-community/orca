package domain

import "time"

// Journal decisions (McpAuditEntry.decision).
const (
	CallAllow    = "allow"
	CallDeny     = "deny"
	CallApproved = "approved"
	CallDenied   = "denied"
	CallExpired  = "expired"

	CallStateStarted = "started"
	CallStateDone    = "done"

	ResultOK    = "ok"
	ResultError = "error"
)

// Reason codes written to the journal / audit metadata and used for neutral
// agent-facing messages.
const (
	ReasonRateLimited     = "rate_limited"
	ReasonApprovalFlood   = "approval_flood"
	ReasonLoopBlocked     = "loop_blocked"
	ReasonLoopSlowDown    = "loop_slow_down"
	ReasonEgressSecret    = "egress_secret_blocked"
	ReasonArgsTooLarge    = "args_too_large_for_review"
	ReasonInvalidArgs     = "invalid_arguments"
	ReasonKillSwitch      = "kill_switch"
	ReasonInterrupted     = "interrupted"
	ReasonKilled          = "killed"
	ReasonApprovalExpired = "approval_expired"
)

// RiskClass groups risks for rate limiting.
func RiskClass(r string) string {
	switch Risk(r) {
	case RiskRead:
		return "read"
	case RiskWriteReversible:
		return "write"
	default:
		return "exec" // exec, destructive, admin and anything unknown
	}
}

// ToolCall is one journal row (mcp.tool_calls): exactly one audit event is
// emitted when it reaches its final state.
type ToolCall struct {
	ID, TenantID, UserID, ClientID, ClientName, SessionID string
	RootSessionID                                         string
	ToolName, Channel, Risk, RiskClass, ParamsHash        string
	ArgsSummary, Decision, ReasonCode, ApprovalID         string
	State, Result                                         string
	ReadUntrusted                                         bool
	DurationMs                                            int64
	StartedAt                                             time.Time
	FinishedAt                                            *time.Time
	ApprovedBy                                            string
	SuppressedCount                                       int
	// TraceID is the W3C trace id of the request that finalized the call; it is
	// audit metadata only (not stored on the journal row).
	TraceID string
}
