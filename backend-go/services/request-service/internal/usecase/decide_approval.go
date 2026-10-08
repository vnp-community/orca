package usecase

import (
	"context"
	"strings"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const (
	DecisionApprove = "approve"
	DecisionReject  = "reject"
)

type DecideApprovalInput struct {
	ID       string
	Decision string // DecisionApprove | DecisionReject
	Comment  string
	// ExpectedDigest must equal the stored digest for approve; for reject it is checked only when set.
	ExpectedDigest  string
	ExpectedVersion int64 // 0 skips the check
}

type DecideApprovalResult struct {
	Approval      domain.Approval
	RequestStatus domain.RequestStatus
}

// DecideApproval approves or rejects in one transaction: Request lock, Approval lock, checks, state change,
// subject handler (which moves the Request), outbox. A handler error rolls everything back.
type DecideApproval struct {
	Repo       ApprovalRepository
	Tx         TxRunner
	Locker     RequestLocker
	Registry   *SubjectHandlerRegistry
	Authorizer ApprovalAuthorizer
	Outbox     OutboxWriter
	// Expirer persists lazy expiry when a decision arrives after the deadline but before the sweeper ran.
	Expirer *ExpireApprovals
}

func (uc *DecideApproval) Execute(ctx context.Context, in DecideApprovalInput) (DecideApprovalResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return DecideApprovalResult{}, err
	}
	in.Decision = strings.ToLower(strings.TrimSpace(in.Decision))
	if in.Decision != DecisionApprove && in.Decision != DecisionReject {
		return DecideApprovalResult{}, domain.ErrApprovalDecisionInvalid
	}
	if in.Decision == DecisionReject {
		if _, err := domain.ValidateApprovalComment(in.Comment); err != nil {
			return DecideApprovalResult{}, err
		}
	}
	userID, _ := tenant.UserID(ctx)

	// Read the request id before locking so the lock order stays Request, then Approval.
	probe, err := uc.Repo.Get(ctx, tenantID, in.ID)
	if err != nil {
		return DecideApprovalResult{}, err
	}

	if p, ok := uc.Authorizer.(interface {
		Prefetch(context.Context, domain.Approval) (context.Context, error)
	}); ok && probe.Status == domain.ApprovalStatusPending {
		if ctx, err = p.Prefetch(ctx, probe); err != nil {
			return DecideApprovalResult{}, err
		}
	}

	var res DecideApprovalResult
	var lateErr error
	err = uc.Tx.InTx(ctx, func(ctx context.Context) error {
		req, err := uc.Locker.LockRequest(ctx, probe.RequestID)
		if err != nil {
			return err
		}
		a, err := uc.Repo.GetForUpdate(ctx, tenantID, in.ID)
		if err != nil {
			return err
		}
		if a.Status != domain.ApprovalStatusPending {
			if sameDecision(a, in, userID) {
				res = DecideApprovalResult{Approval: a, RequestStatus: req.Status} // retry of the same decision: no second event
				return nil
			}
			return domain.ErrApprovalNotPending
		}
		now, err := uc.Repo.NowDB(ctx)
		if err != nil {
			return err
		}
		if a.EffectiveStatus(now) == domain.ApprovalStatusExpired {
			// Committing the expiry (instead of returning the error inside the tx) is what makes it stick.
			if err := uc.Expirer.ExpireLocked(ctx, a, req, now); err != nil {
				return err
			}
			lateErr = domain.ErrApprovalExpired
			return nil
		}
		if err := uc.Authorizer.CanDecide(ctx, req, a); err != nil {
			return err
		}
		if in.ExpectedVersion != 0 && in.ExpectedVersion != a.Version {
			return domain.ErrApprovalVersionConflict
		}
		if (in.Decision == DecisionApprove || in.ExpectedDigest != "") && in.ExpectedDigest != a.SubjectDigest {
			return domain.ErrApprovalDigestMismatch
		}
		if string(req.Status) != a.Stage {
			return domain.ErrApprovalStageMismatch
		}

		prev := a.Version
		if in.Decision == DecisionApprove {
			err = a.Approve(userID, in.Comment, now)
		} else {
			err = a.Reject(userID, in.Comment, now)
		}
		if err != nil {
			return err
		}
		if ok, err := uc.Repo.UpdateDecision(ctx, a, prev); err != nil {
			return err
		} else if !ok {
			return domain.ErrApprovalVersionConflict
		}
		a.Version = prev + 1

		decision := "approved"
		if h := uc.Registry.Get(a.SubjectType); h != nil {
			if in.Decision == DecisionApprove {
				err = h.OnApproved(ctx, ctx, a)
			} else {
				err = h.OnRejected(ctx, ctx, a)
			}
			if err != nil {
				return err
			}
		}
		if in.Decision == DecisionReject {
			decision = "rejected"
		}
		after, err := uc.Locker.LockRequest(ctx, a.RequestID) // re-read: the handler may have moved the Request
		if err != nil {
			return err
		}
		ev, err := NewOutboxEvent(ctx, domain.SubjectApprovalDecided, domain.NewApprovalDecidedPayload(a, after, decision))
		if err != nil {
			return err
		}
		if err := uc.Outbox.InsertOutboxEvent(ctx, ev); err != nil {
			return err
		}
		res = DecideApprovalResult{Approval: a, RequestStatus: after.Status}
		return nil
	})
	if err != nil {
		return DecideApprovalResult{}, err
	}
	if lateErr != nil {
		return DecideApprovalResult{}, lateErr
	}
	return res, nil
}

func sameDecision(a domain.Approval, in DecideApprovalInput, userID string) bool {
	if a.DecidedBy == nil || *a.DecidedBy != userID || userID == "" {
		return false
	}
	return (in.Decision == DecisionApprove && a.Status == domain.ApprovalStatusApproved) ||
		(in.Decision == DecisionReject && a.Status == domain.ApprovalStatusRejected)
}
