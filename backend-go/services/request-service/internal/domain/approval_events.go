package domain

import "time"

const (
	SubjectApprovalRequested = "orca.request.approval.requested"
	SubjectApprovalDecided   = "orca.request.approval.decided"
)

// ApprovalRequestedPayload is the wire shape of orca.request.approval.requested.
// It never carries the decision comment or subject content: notifications are stored.
// reporter_id and self_approval_allowed let the relay-side enrichment pick recipients without a DB read.
type ApprovalRequestedPayload struct {
	ApprovalID          string     `json:"approval_id"`
	RequestID           string     `json:"request_id"`
	RequestNumber       int64      `json:"request_number,omitempty"`
	SubjectType         string     `json:"subject_type"`
	SubjectID           string     `json:"subject_id"`
	Stage               string     `json:"stage"`
	RequestedBy         string     `json:"requested_by"`
	DueAt               *time.Time `json:"due_at,omitempty"`
	ReporterID          string     `json:"reporter_id,omitempty"`
	SelfApprovalAllowed bool       `json:"self_approval_allowed"`
	Reason              string     `json:"reason,omitempty"`
}

// ApprovalDecidedPayload is the wire shape of orca.request.approval.decided; decision is approved|rejected|cancelled|expired.
type ApprovalDecidedPayload struct {
	ApprovalID    string `json:"approval_id"`
	RequestID     string `json:"request_id"`
	RequestNumber int64  `json:"request_number,omitempty"`
	SubjectType   string `json:"subject_type"`
	SubjectID     string `json:"subject_id"`
	Decision      string `json:"decision"`
	DecidedBy     string `json:"decided_by,omitempty"`
	RequestedBy   string `json:"requested_by,omitempty"`
	ReporterID    string `json:"reporter_id,omitempty"`
	// Stage and WaitSeconds feed audit and the approval wait histogram (CR-REQ-024); additive, omitted when unknown.
	Stage       string  `json:"stage,omitempty"`
	WaitSeconds float64 `json:"wait_seconds,omitempty"`
}

func NewApprovalRequestedPayload(a Approval, r Request, reason string) ApprovalRequestedPayload {
	return ApprovalRequestedPayload{
		ApprovalID: a.ID, RequestID: a.RequestID, RequestNumber: r.Number, SubjectType: string(a.SubjectType), SubjectID: a.SubjectID,
		Stage: a.Stage, RequestedBy: a.RequestedBy, DueAt: a.DueAt, ReporterID: r.ReporterID, SelfApprovalAllowed: a.SelfApprovalAllowed, Reason: reason,
	}
}

func NewApprovalDecidedPayload(a Approval, r Request, decision string) ApprovalDecidedPayload {
	p := ApprovalDecidedPayload{
		ApprovalID: a.ID, RequestID: a.RequestID, RequestNumber: r.Number, SubjectType: string(a.SubjectType), SubjectID: a.SubjectID,
		Decision: decision, RequestedBy: a.RequestedBy, ReporterID: r.ReporterID, Stage: a.Stage,
	}
	if a.DecidedAt != nil && !a.CreatedAt.IsZero() && a.DecidedAt.After(a.CreatedAt) {
		p.WaitSeconds = a.DecidedAt.Sub(a.CreatedAt).Seconds()
	}
	if a.DecidedBy != nil {
		p.DecidedBy = *a.DecidedBy
	}
	return p
}
