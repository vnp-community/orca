package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
)

type CancelPendingApprovalsForRequest struct {
	Repo     ApprovalRepository
	Registry *SubjectHandlerRegistry
	Outbox   OutboxWriter
}

func (uc *CancelPendingApprovalsForRequest) Execute(ctx context.Context, txCtx context.Context, requestID, why string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	list, err := uc.Repo.CancelPendingForRequest(txCtx, tenantID, requestID, why, now)
	if err != nil {
		return err
	}

	for _, a := range list {
		handler := uc.Registry.Get(a.SubjectType)
		if handler != nil {
			if err := handler.OnClosedWithoutDecision(ctx, txCtx, a, why); err != nil {
				return err
			}
		}

		ev, err := NewOutboxEvent(ctx, "orca.request.approval.decided", map[string]any{
			"approval_id": a.ID,
			"decision":    "cancelled",
		})
		if err != nil {
			return err
		}
		if err := uc.Outbox.InsertOutboxEvent(txCtx, ev); err != nil {
			return err
		}
	}
	return nil
}
