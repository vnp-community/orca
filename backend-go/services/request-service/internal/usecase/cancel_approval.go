package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type CancelApproval struct {
	Repo     ApprovalRepository
	Tx       TxRunner
	Registry *SubjectHandlerRegistry
	Outbox   OutboxWriter
}

func (uc *CancelApproval) Execute(ctx context.Context, id, reason string) (*domain.Approval, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	var a domain.Approval
	err = uc.Tx.InTx(ctx, func(txCtx context.Context) error {
		var err error
		a, err = uc.Repo.GetForUpdate(txCtx, tenantID, id)
		if err != nil {
			return err
		}

		if a.Status != domain.ApprovalStatusPending {
			return ErrAlreadyDecided
		}

		now := time.Now().UTC()
		userID, _ := tenant.UserID(ctx)
		// Check permissions: should be admin or requester (omitted for stub)

		if err := a.Cancel(userID, reason, now); err != nil {
			return err
		}

		updated, err := uc.Repo.UpdateDecision(txCtx, a, a.Version-1)
		if err != nil {
			return err
		}
		if !updated {
			return ErrVersionConflict
		}

		handler := uc.Registry.Get(a.SubjectType)
		if handler != nil {
			if err := handler.OnClosedWithoutDecision(ctx, txCtx, a, reason); err != nil {
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
		return uc.Outbox.InsertOutboxEvent(txCtx, ev)
	})

	if err != nil {
		return nil, err
	}
	return &a, nil
}
