package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// CancelApproval lets the requester or an admin withdraw a pending approval.
type CancelApproval struct {
	Repo     ApprovalRepository
	Tx       TxRunner
	Locker   RequestLocker
	Registry *SubjectHandlerRegistry
	Outbox   OutboxWriter
}

func (uc *CancelApproval) Execute(ctx context.Context, id, reason string) (*domain.Approval, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	userID, _ := tenant.UserID(ctx)
	role, _ := tenant.Role(ctx)
	if userID == "" {
		return nil, domain.ErrNoUser
	}
	if tenant.ActorType(ctx) == tenant.ActorAgent {
		return nil, domain.ErrAgentForbidden
	}
	if _, err := domain.ValidateApprovalComment(reason); err != nil {
		return nil, err
	}
	probe, err := uc.Repo.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	var out domain.Approval
	err = uc.Tx.InTx(ctx, func(ctx context.Context) error {
		req, err := uc.Locker.LockRequest(ctx, probe.RequestID)
		if err != nil {
			return err
		}
		a, err := uc.Repo.GetForUpdate(ctx, tenantID, id)
		if err != nil {
			return err
		}
		if a.Status == domain.ApprovalStatusCancelled && a.DecidedBy != nil && *a.DecidedBy == userID {
			out = a // retry by the same caller
			return nil
		}
		if a.Status != domain.ApprovalStatusPending {
			return domain.ErrApprovalNotPending
		}
		if role != "admin" && a.RequestedBy != userID {
			return domain.ErrApprovalForbidden
		}
		now, err := uc.Repo.NowDB(ctx)
		if err != nil {
			return err
		}
		prev := a.Version
		if err := a.Cancel(userID, reason, now); err != nil {
			return err
		}
		if ok, err := uc.Repo.UpdateDecision(ctx, a, prev); err != nil {
			return err
		} else if !ok {
			return domain.ErrApprovalVersionConflict
		}
		a.Version = prev + 1
		if h := uc.Registry.Get(a.SubjectType); h != nil {
			if err := h.OnClosedWithoutDecision(ctx, ctx, a, "cancelled"); err != nil {
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
		out = a
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}
