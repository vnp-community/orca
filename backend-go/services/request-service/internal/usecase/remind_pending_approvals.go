package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type RemindPendingApprovals struct {
	Repo   ApprovalRepository
	Tx     TxRunner
	Outbox OutboxWriter
}

func (uc *RemindPendingApprovals) Execute(ctx context.Context, batch int) (int, error) {
	log := slog.Default()

	candidates, err := uc.Repo.ClaimDueForReminder(ctx, batch)
	if err != nil {
		return 0, err
	}

	remindedCount := 0
	for _, c := range candidates {
		err := uc.Tx.InTx(ctx, func(txCtx context.Context) error {
			// Lock Request (omitted)

			a, err := uc.Repo.GetForUpdate(txCtx, c.TenantID, c.ApprovalID)
			if err != nil {
				return err
			}

			if a.Status != domain.ApprovalStatusPending || a.RemindedAt != nil {
				return nil
			}

			nowDB, err := uc.Repo.NowDB(txCtx)
			if err != nil {
				return err
			}

			// Check 75%
			if a.DueAt == nil || a.DueAt.IsZero() {
				return nil
			}
			duration := (*a.DueAt).Sub(a.CreatedAt)
			thresh := a.CreatedAt.Add(duration * 3 / 4)
			if nowDB.Before(thresh) {
				return nil
			}

			a.RemindedAt = &nowDB

			updated, err := uc.Repo.UpdateDecision(txCtx, a, a.Version-1)
			if err != nil {
				return err
			}
			if !updated {
				return ErrVersionConflict
			}

			ev, err := NewOutboxEvent(ctx, "orca.request.approval.requested", map[string]any{
				"approval_id":           a.ID,
				"request_id":            a.RequestID,
				"subject_type":          a.SubjectType,
				"subject_id":            a.SubjectID,
				"reason":                "reminder",
				"self_approval_allowed": a.SelfApprovalAllowed,
				"reporter_id":           a.RequestedBy, // stub
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
			log.Warn("failed to remind approval", slog.String("approval_id", c.ApprovalID), slog.Any("error", err))
		} else {
			remindedCount++
		}
	}

	return remindedCount, nil
}
