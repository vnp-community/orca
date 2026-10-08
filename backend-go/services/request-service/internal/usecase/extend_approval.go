package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ExtendApprovalInput struct {
	ID              string
	ExtendSeconds   int
	Reason          string
	ExpectedVersion int64 // 0 skips the check
}

// ExtendApproval pushes a pending approval's deadline out (admin or requester only) instead of letting it
// expire and forcing a reopen from the backlog.
type ExtendApproval struct {
	Repo   ApprovalRepository
	Tx     TxRunner
	Locker RequestLocker
}

func (uc *ExtendApproval) Execute(ctx context.Context, in ExtendApprovalInput) (*domain.Approval, error) {
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
	probe, err := uc.Repo.Get(ctx, tenantID, in.ID)
	if err != nil {
		return nil, err
	}
	var out domain.Approval
	err = uc.Tx.InTx(ctx, func(ctx context.Context) error {
		if _, err := uc.Locker.LockRequest(ctx, probe.RequestID); err != nil {
			return err
		}
		a, err := uc.Repo.GetForUpdate(ctx, tenantID, in.ID)
		if err != nil {
			return err
		}
		if a.Status != domain.ApprovalStatusPending {
			return domain.ErrApprovalNotPending
		}
		if role != "admin" && a.RequestedBy != userID {
			return domain.ErrApprovalForbidden
		}
		if in.ExpectedVersion != 0 && in.ExpectedVersion != a.Version {
			return domain.ErrApprovalVersionConflict
		}
		now, err := uc.Repo.NowDB(ctx)
		if err != nil {
			return err
		}
		prev := a.Version
		if err := a.Extend(in.ExtendSeconds, now); err != nil {
			return err
		}
		if ok, err := uc.Repo.UpdateDue(ctx, a, prev); err != nil {
			return err
		} else if !ok {
			return domain.ErrApprovalVersionConflict
		}
		a.Version = prev + 1
		out = a
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}
