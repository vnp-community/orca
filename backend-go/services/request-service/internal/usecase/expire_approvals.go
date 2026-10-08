package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RequestReturner moves a Request to request_backlog; *ReturnRequestToBacklog implements it.
type RequestReturner interface {
	Execute(ctx context.Context, in ReturnInput) (domain.Request, error)
}

var _ RequestReturner = (*ReturnRequestToBacklog)(nil)

const approvalExpiredReason = "approval_expired"

type ExpireApprovals struct {
	Repo     ApprovalRepository
	Tx       TxRunner
	Locker   RequestLocker
	Registry *SubjectHandlerRegistry
	Returner RequestReturner
	Outbox   OutboxWriter
	Log      *slog.Logger
}

// Execute expires up to batch due approvals. ClaimDue spans tenants, so each candidate runs under its own
// tenant before opening a transaction; one failing candidate never stops the others.
func (uc *ExpireApprovals) Execute(ctx context.Context, batch int) (int, error) {
	log := uc.Log
	if log == nil {
		log = slog.Default()
	}
	claims, err := uc.Repo.ClaimDue(ctx, batch)
	if err != nil {
		return 0, err
	}
	expired := 0
	for _, c := range claims {
		tctx := tenant.WithTenantID(ctx, c.TenantID)
		done := false
		err := uc.Tx.InTx(tctx, func(txCtx context.Context) error {
			req, err := uc.Locker.LockRequest(txCtx, c.RequestID)
			if err != nil {
				return err
			}
			a, err := uc.Repo.GetForUpdate(txCtx, c.TenantID, c.ApprovalID)
			if err != nil {
				return err
			}
			if a.Status != domain.ApprovalStatusPending {
				return nil // decided or cancelled while we were waiting for the lock
			}
			now, err := uc.Repo.NowDB(txCtx)
			if err != nil {
				return err
			}
			if a.DueAt == nil || now.Before(*a.DueAt) {
				return nil // extended meanwhile
			}
			if err := uc.ExpireLocked(txCtx, a, req, now); err != nil {
				return err
			}
			done = true
			return nil
		})
		if err != nil {
			log.Warn("approval expiry failed", slog.String("approval_id", c.ApprovalID), slog.Any("error", err))
			continue
		}
		if done {
			expired++
		}
	}
	return expired, nil
}

// ExpireLocked closes a pending approval as expired. The caller holds the Request and Approval locks and
// runs inside a transaction. A Request still at the approval's stage is returned to the backlog; one that
// already moved on is left alone.
func (uc *ExpireApprovals) ExpireLocked(ctx context.Context, a domain.Approval, req domain.Request, now time.Time) error {
	prev := a.Version
	if err := a.Expire(now); err != nil {
		return err
	}
	if ok, err := uc.Repo.UpdateDecision(ctx, a, prev); err != nil {
		return err
	} else if !ok {
		return domain.ErrApprovalVersionConflict
	}
	a.Version = prev + 1
	if h := uc.Registry.Get(a.SubjectType); h != nil {
		if err := h.OnClosedWithoutDecision(ctx, ctx, a, "expired"); err != nil {
			return err
		}
	}
	if string(req.Status) == a.Stage && uc.Returner != nil {
		if _, err := uc.Returner.Execute(ctx, ReturnInput{
			RequestID: req.ID, Stage: domain.ApprovalReturnStage(a.SubjectType, req.Status, req), Category: domain.ReturnCategoryOther,
			Reason: approvalExpiredReason, ActorID: "", ActorKind: domain.ActorKindSystem,
		}); err != nil {
			return err
		}
	}
	ev, err := NewOutboxEvent(ctx, domain.SubjectApprovalDecided, domain.NewApprovalDecidedPayload(a, req, "expired"))
	if err != nil {
		return err
	}
	return uc.Outbox.InsertOutboxEvent(ctx, ev)
}
