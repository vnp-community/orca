package domain

import (
	"errors"
	"time"
)

// ApprovalStatus is an Approval's lifecycle state.
type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalRejected ApprovalStatus = "rejected"
)

var (
	// ErrApprovalEmptyTenant guards an approval with no owning tenant.
	ErrApprovalEmptyTenant = errors.New("domain: tenant_id is required")
	// ErrApprovalEmptyTemplate guards an approval with no target template.
	ErrApprovalEmptyTemplate = errors.New("domain: template_id is required")
	// ErrApprovalEmptyRequestedBy guards an approval with no requesting user.
	ErrApprovalEmptyRequestedBy = errors.New("domain: requested_by is required")
	// ErrApprovalNotPending is the invariant Resolve enforces: only a
	// pending approval can be approved/rejected — matches
	// workflow.approvals' idx_workflow_approvals_one_pending_per_template
	// unique index, which already guarantees at most one pending row per
	// template; this is the in-memory mirror of that same rule.
	ErrApprovalNotPending = errors.New("domain: approval is not pending")
	// ErrApprovalNotFound is the sentinel adapter/postgres returns (wrapped)
	// when a lookup finds no row — usecase maps it to apperrors.KindNotFound.
	ErrApprovalNotFound = errors.New("domain: approval not found")
	// ErrApprovalAlreadyPending is the sentinel adapter/postgres returns
	// (wrapped) when a CreateTx would violate
	// idx_workflow_approvals_one_pending_per_template (migrations/0008) —
	// usecase.PublishTemplate maps it to apperrors.KindAlreadyExists, a
	// clean typed conflict rather than a raw constraint-violation leak.
	ErrApprovalAlreadyPending = errors.New("domain: a pending approval already exists for this template")
)

// Approval is a lead-requires-admin-approval gate row — mirrors
// orchestration-service.md §5's decision_gates shape deliberately, per
// workflow.approvals' migration comment: "a row gating a state transition
// until a human resolves it." Backs WorkflowTemplate.Visibility's escalation
// to company/public (see usecase.PublishTemplate).
type Approval struct {
	ID          string
	TenantID    string
	TemplateID  string
	RequestedBy string
	Status      ApprovalStatus
	ResolvedBy  string
	ResolvedAt  *time.Time
	CreatedAt   time.Time
}

// NewApproval constructs a pending Approval, enforcing the invariants a
// gate row must satisfy to be meaningful: an owning tenant, a target
// template, and a requesting user.
func NewApproval(id, tenantID, templateID, requestedBy string) (Approval, error) {
	if tenantID == "" {
		return Approval{}, ErrApprovalEmptyTenant
	}
	if templateID == "" {
		return Approval{}, ErrApprovalEmptyTemplate
	}
	if requestedBy == "" {
		return Approval{}, ErrApprovalEmptyRequestedBy
	}
	return Approval{
		ID:          id,
		TenantID:    tenantID,
		TemplateID:  templateID,
		RequestedBy: requestedBy,
		Status:      ApprovalPending,
	}, nil
}

// Approve transitions a pending approval to approved, recording who
// resolved it and when. Returns ErrApprovalNotPending for any other
// current status — a decision already made must not be silently
// overwritten by a second call.
func (a *Approval) Approve(resolvedBy string, now time.Time) error {
	if a.Status != ApprovalPending {
		return ErrApprovalNotPending
	}
	a.Status = ApprovalApproved
	a.ResolvedBy = resolvedBy
	a.ResolvedAt = &now
	return nil
}

// Reject transitions a pending approval to rejected — see Approve's doc
// comment for the same not-pending guard.
func (a *Approval) Reject(resolvedBy string, now time.Time) error {
	if a.Status != ApprovalPending {
		return ErrApprovalNotPending
	}
	a.Status = ApprovalRejected
	a.ResolvedBy = resolvedBy
	a.ResolvedAt = &now
	return nil
}
