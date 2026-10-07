package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

var (
	ErrApprovalExpired        = errors.New("approval expired")
	ErrApprovalDigestMismatch = errors.New("approval digest mismatch")
	ErrApprovalStageMismatch  = errors.New("approval stage mismatch")
	ErrVersionConflict        = errors.New("version conflict")
	ErrAlreadyDecided         = errors.New("already decided")
)

type DecideApprovalInput struct {
	ID             string
	Decision       string // "approve" | "reject"
	Comment        string
	ExpectedDigest string
}

type DecideApproval struct {
	Repo       ApprovalRepository
	Tx         TxRunner
	Registry   *SubjectHandlerRegistry
	Authorizer ApprovalAuthorizer
	Outbox     OutboxWriter
	// RequestGate would be here to lock request
}

func (uc *DecideApproval) Execute(ctx context.Context, in DecideApprovalInput) (*domain.Approval, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	var a domain.Approval
	err = uc.Tx.InTx(ctx, func(txCtx context.Context) error {
		// lock request here via RequestGate (omitted for stub)

		var err error
		a, err = uc.Repo.GetForUpdate(txCtx, tenantID, in.ID)
		if err != nil {
			return err
		}

		now := time.Now().UTC()

		if a.Status != domain.ApprovalStatusPending {
			// Idempotent retry logic could go here if checking same actor and decision
			return ErrAlreadyDecided
		}

		if a.EffectiveStatus(now) == domain.ApprovalStatusExpired {
			a.Expire(now)
			uc.Repo.UpdateDecision(txCtx, a, a.Version-1)
			return ErrApprovalExpired
		}

		if err := uc.Authorizer.CanDecide(ctx, domain.Request{}, a); err != nil { // passing dummy request
			return err
		}

		if a.SubjectDigest != in.ExpectedDigest {
			return ErrApprovalDigestMismatch
		}

		// Stage mismatch check would go here with real Request

		userID, _ := tenant.UserID(ctx)
		if in.Decision == "approve" {
			if err := a.Approve(userID, in.Comment, now); err != nil {
				return err
			}
		} else {
			if err := a.Reject(userID, in.Comment, now); err != nil {
				return err
			}
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
			if in.Decision == "approve" {
				if err := handler.OnApproved(ctx, txCtx, a); err != nil {
					return err
				}
			} else {
				if err := handler.OnRejected(ctx, txCtx, a); err != nil {
					return err
				}
			}
		}

		statusStr := "approved"
		if in.Decision != "approve" {
			statusStr = "rejected"
		}

		ev, err := NewOutboxEvent(ctx, "orca.request.approval.decided", map[string]any{
			"approval_id": a.ID,
			"decision":    statusStr,
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
