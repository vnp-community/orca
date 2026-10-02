package domain

import (
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

// Approval statuses (CONTRACT McpApproval.status). "approved" stays after
// consumption; ConsumedAt is an internal flag, not a status.
const (
	ApprovalPending   = "pending"
	ApprovalApproved  = "approved"
	ApprovalDenied    = "denied"
	ApprovalExpired   = "expired"
	ApprovalCancelled = "cancelled"

	ViaWeb         = "web"
	ViaMobile      = "mobile"
	ViaElicitation = "elicitation"
)

const (
	CodeApprovalExpired        = "MCP_APPROVAL_EXPIRED"
	CodeApprovalAlreadyDecided = "MCP_APPROVAL_ALREADY_DECIDED"
	CodeApprovalHashMismatch   = "MCP_APPROVAL_HASH_MISMATCH"
	CodeKillSwitchActive       = "MCP_KILL_SWITCH_ACTIVE"
)

func ErrApprovalExpired() error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeApprovalExpired, "approval has expired", nil)
}
func ErrApprovalAlreadyDecided() error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeApprovalAlreadyDecided, "approval was already decided", nil)
}
func ErrApprovalHashMismatch() error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeApprovalHashMismatch, "approval parameters changed; reload the request", nil)
}
func ErrKillSwitchActive() error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeKillSwitchActive, "MCP kill switch is active", nil)
}

// ElicitationEligibleRisks: an elicitation answer travels through the MCP
// client itself, so a manipulated client can fake "accept". It may therefore
// only decide the lowest-risk approvals; everything else needs the owner's
// own session (mcp.approval.decide).
var ElicitationEligibleRisks = map[Risk]bool{RiskWriteReversible: true}

type Approval struct {
	ID, TenantID, UserID, ClientID, ClientName string
	SessionID, CallID                          string
	ToolName, ToolTitle, Channel, Risk         string
	ParamsHash, ArgsPreview                    string
	ArgsRedacted                               bool
	Status                                     string
	CreatedAt, ExpiresAt                       time.Time
	DecidedAt, ConsumedAt                      *time.Time
	DecidedVia, DecisionNote                   string
	Reasons                                    []string
}

// EffectiveStatus reports "expired" for a pending approval past its deadline,
// so readers never depend on the expiry worker's timing.
func (a Approval) EffectiveStatus(now time.Time) string {
	if a.Status == ApprovalPending && !now.Before(a.ExpiresAt) {
		return ApprovalExpired
	}
	return a.Status
}

// CanTransition encodes the state machine; terminal states never change.
func CanTransition(from, to string) bool {
	switch from {
	case ApprovalPending:
		return to == ApprovalApproved || to == ApprovalDenied || to == ApprovalExpired || to == ApprovalCancelled
	case ApprovalApproved:
		return to == ApprovalExpired || to == ApprovalCancelled
	}
	return false
}

// DecideDiagnosis picks the error for a decision that matched no pending row,
// in the spec's fixed order: not found/not owner, expired, already decided,
// hash mismatch. found=false covers "missing", "other user" and "other tenant".
func DecideDiagnosis(a Approval, found bool, now time.Time) error {
	switch {
	case !found:
		return ErrNotFound()
	case a.EffectiveStatus(now) == ApprovalExpired:
		return ErrApprovalExpired()
	case a.Status != ApprovalPending:
		return ErrApprovalAlreadyDecided()
	default:
		return ErrApprovalHashMismatch()
	}
}

// ApprovalDeepLink is the CONTRACT section 4 / D5 deep link.
func ApprovalDeepLink(id string) string {
	return "/?section=mcp&tab=approvals&approval=" + id
}

// NewApprovalRequestedEvent builds the notification-bound event. The body
// carries only client name, tool title and risk: never arguments, because
// notifications are stored and may traverse third-party push services.
func NewApprovalRequestedEvent(eventID string, a Approval, at time.Time) (OutboxRecord, error) {
	return NewOutboxEvent(eventID, SubjectApprovalRequested, a.TenantID, at, map[string]any{
		"user_id": a.UserID, "title": "Approval needed",
		"body":      a.ClientName + " wants to use " + a.ToolTitle + " (" + a.Risk + ")",
		"deep_link": ApprovalDeepLink(a.ID), "approval_id": a.ID, "expires_at": a.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

// NewApprovalResolvedEvent carries ids only; consumers re-read the row.
func NewApprovalResolvedEvent(eventID string, a Approval, at time.Time) (OutboxRecord, error) {
	return NewOutboxEvent(eventID, SubjectApprovalResolved, a.TenantID, at, map[string]any{
		"user_id": a.UserID, "approval_id": a.ID, "status": a.Status,
	})
}
