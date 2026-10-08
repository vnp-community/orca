package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RemindPendingApprovals re-emits approval.requested once per approval at 75% of its time window.
// Reminders never lock the Request: they change no Request state and must not queue behind a long decision.
type RemindPendingApprovals struct {
	Repo     ApprovalRepository
	Requests RequestRepository
	Tx       TxRunner
	Outbox   OutboxWriter
	Log      *slog.Logger
}

func (uc *RemindPendingApprovals) Execute(ctx context.Context, batch int) (int, error) {
	log := uc.Log
	if log == nil {
		log = slog.Default()
	}
	claims, err := uc.Repo.ClaimDueForReminder(ctx, batch)
	if err != nil {
		return 0, err
	}
	reminded := 0
	for _, c := range claims {
		tctx := tenant.WithTenantID(ctx, c.TenantID)
		done := false
		err := uc.Tx.InTx(tctx, func(txCtx context.Context) error {
			a, err := uc.Repo.Get(txCtx, c.TenantID, c.ApprovalID)
			if err != nil {
				return err
			}
			if a.Status != domain.ApprovalStatusPending || a.RemindedAt != nil {
				return nil
			}
			req, err := uc.Requests.Get(txCtx, a.RequestID)
			if err != nil {
				return err
			}
			now, err := uc.Repo.NowDB(txCtx)
			if err != nil {
				return err
			}
			// Compare-and-set on reminded_at: of several replicas exactly one emits.
			ok, err := uc.Repo.MarkReminded(txCtx, c.TenantID, a.ID, now)
			if err != nil || !ok {
				return err
			}
			ev, err := NewOutboxEvent(txCtx, domain.SubjectApprovalRequested, domain.NewApprovalRequestedPayload(a, req, "reminder"))
			if err != nil {
				return err
			}
			if err := uc.Outbox.InsertOutboxEvent(txCtx, ev); err != nil {
				return err
			}
			done = true
			return nil
		})
		if err != nil {
			log.Warn("approval reminder failed", slog.String("approval_id", c.ApprovalID), slog.Any("error", err))
			continue
		}
		if done {
			reminded++
		}
	}
	return reminded, nil
}
