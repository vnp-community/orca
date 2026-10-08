package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RequestTypeApprovalRecorder is the real ApprovalRecorder for classification: a proposal opens the request_type
// approval, a direct confirmation (ConfirmRequestType RPC) settles it so the inbox does not keep a stale row.
type RequestTypeApprovalRecorder struct {
	Open   *OpenApproval
	Repo   ApprovalRepository
	Tx     TxRunner
	Locker RequestLocker
	Outbox OutboxWriter
}

var _ ApprovalRecorder = (*RequestTypeApprovalRecorder)(nil)

func (r *RequestTypeApprovalRecorder) RequestTypeApproval(ctx context.Context, requestID string) error {
	_, err := r.Open.Execute(ctx, OpenApprovalInput{RequestID: requestID, SubjectType: domain.SubjectRequestType, RequestedBy: systemActor})
	return err
}

// Approve closes the pending request_type approval as approved by actorID without running the subject handler:
// the caller is the confirmation itself. After an approval-driven confirm nothing is pending and this is a no-op.
func (r *RequestTypeApprovalRecorder) Approve(ctx context.Context, requestID, actorID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return err
	}
	return r.Tx.InTx(ctx, func(ctx context.Context) error {
		req, err := r.Locker.LockRequest(ctx, requestID)
		if err != nil {
			return err
		}
		pending, err := r.Repo.FindPendingBySubject(ctx, tenantID, domain.SubjectRequestType, requestID)
		if err != nil || pending == nil {
			return err
		}
		a, err := r.Repo.GetForUpdate(ctx, tenantID, pending.ID)
		if err != nil || a.Status != domain.ApprovalStatusPending {
			return err
		}
		now, err := r.Repo.NowDB(ctx)
		if err != nil {
			return err
		}
		prev := a.Version
		if err := a.Approve(actorID, "confirmed directly", now); err != nil {
			return err
		}
		if ok, err := r.Repo.UpdateDecision(ctx, a, prev); err != nil {
			return err
		} else if !ok {
			return domain.ErrApprovalVersionConflict
		}
		a.Version = prev + 1
		ev, err := NewOutboxEvent(ctx, domain.SubjectApprovalDecided, domain.NewApprovalDecidedPayload(a, req, "approved"))
		if err != nil {
			return err
		}
		return r.Outbox.InsertOutboxEvent(ctx, ev)
	})
}
