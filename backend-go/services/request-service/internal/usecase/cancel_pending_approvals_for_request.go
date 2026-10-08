package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// CancelPendingApprovalsForRequest closes every pending approval when a Request leaves the flow
// (return, cancel, type change). Callers already changed the Request row, so the Request lock is held.
type CancelPendingApprovalsForRequest struct {
	Repo     ApprovalRepository
	Tx       TxRunner
	Locker   RequestLocker
	Registry *SubjectHandlerRegistry
	Outbox   OutboxWriter
}

func (uc *CancelPendingApprovalsForRequest) Execute(ctx context.Context, requestID, why string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return err
	}
	return uc.Tx.InTx(ctx, func(ctx context.Context) error {
		req, err := uc.Locker.LockRequest(ctx, requestID)
		if err != nil {
			return err
		}
		now, err := uc.Repo.NowDB(ctx)
		if err != nil {
			return err
		}
		list, err := uc.Repo.CancelPendingForRequest(ctx, tenantID, requestID, why, now)
		if err != nil {
			return err
		}
		for _, a := range list {
			if h := uc.Registry.Get(a.SubjectType); h != nil {
				if err := h.OnClosedWithoutDecision(ctx, ctx, a, why); err != nil {
					return err
				}
			}
			ev, err := NewOutboxEvent(ctx, domain.SubjectApprovalDecided, domain.NewApprovalDecidedPayload(a, req, "cancelled"))
			if err != nil {
				return err
			}
			if err := uc.Outbox.InsertOutboxEvent(ctx, ev); err != nil {
				return err
			}
		}
		return nil
	})
}

// PendingApprovalCanceller adapts CancelPendingApprovalsForRequest to the ApprovalCanceller port.
type PendingApprovalCanceller struct {
	Inner *CancelPendingApprovalsForRequest
}

var _ ApprovalCanceller = (*PendingApprovalCanceller)(nil)

func (c *PendingApprovalCanceller) CancelPending(ctx context.Context, requestID, why string) error {
	return c.Inner.Execute(ctx, requestID, why)
}
