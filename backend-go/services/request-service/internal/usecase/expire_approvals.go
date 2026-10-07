package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ExpireApprovals struct {
	Repo     ApprovalRepository
	Tx       TxRunner
	Registry *SubjectHandlerRegistry
	Outbox   OutboxWriter
}

func (uc *ExpireApprovals) Execute(ctx context.Context, batch int) (int, error) {
	log := slog.Default()

	// Fetch candidates
	candidates, err := uc.Repo.ClaimDue(ctx, batch)
	if err != nil {
		return 0, err
	}

	expiredCount := 0
	for _, c := range candidates {
		err := uc.Tx.InTx(ctx, func(txCtx context.Context) error {
			// Lock Request: this is a stub, assuming request locked via RequestGate
			
			a, err := uc.Repo.GetForUpdate(txCtx, c.TenantID, c.ApprovalID)
			if err != nil {
				return err
			}

			if a.Status != domain.ApprovalStatusPending {
				return nil // Handled by someone else
			}

			nowDB, err := uc.Repo.NowDB(txCtx)
			if err != nil {
				return err
			}

			if a.DueAt == nil || a.DueAt.IsZero() || nowDB.Before(*a.DueAt) {
				return nil // Not due
			}

			a.Expire(nowDB)

			updated, err := uc.Repo.UpdateDecision(txCtx, a, a.Version-1)
			if err != nil {
				return err
			}
			if !updated {
				return ErrVersionConflict
			}

			handler := uc.Registry.Get(a.SubjectType)
			if handler != nil {
				if err := handler.OnClosedWithoutDecision(ctx, txCtx, a, "expired"); err != nil {
					return err
				}
			}

			ev, err := NewOutboxEvent(ctx, "orca.request.approval.decided", map[string]any{
				"approval_id": a.ID,
				"request_id":  a.RequestID,
				"decision":    "expired",
			})
			if err != nil {
				return err
			}
			if err := uc.Outbox.InsertOutboxEvent(txCtx, ev); err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			log.Warn("failed to expire approval", slog.String("approval_id", c.ApprovalID), slog.Any("error", err))
		} else {
			expiredCount++
		}
	}

	return expiredCount, nil
}
