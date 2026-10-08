package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ClarificationSweeper expires overdue clarifications and sends the one reminder at half of the window.
// Several instances may run it: a row is locked with SKIP LOCKED and processed in its own tenant transaction,
// so nothing is handled twice and a failure leaves the row open for the next sweep.
type ClarificationSweeper struct {
	repo           RequestRepository
	clarifications ClarificationRepository
	returner       *ReturnRequestToBacklog
	recipients     RecipientResolver
	tx             TxRunner
	outbox         OutboxWriter
	clock          func() time.Time
}

func NewClarificationSweeper(repo RequestRepository, clarifications ClarificationRepository, returner *ReturnRequestToBacklog,
	recipients RecipientResolver, tx TxRunner, outbox OutboxWriter) *ClarificationSweeper {
	return &ClarificationSweeper{repo: repo, clarifications: clarifications, returner: returner, recipients: recipients, tx: tx, outbox: outbox,
		clock: func() time.Time { return time.Now().UTC() }}
}

func (s *ClarificationSweeper) WithClock(c func() time.Time) *ClarificationSweeper {
	s.clock = c
	return s
}

// ExpireOnce expires up to batch overdue clarifications and returns how many it handled.
func (s *ClarificationSweeper) ExpireOnce(ctx context.Context, batch int) (int, error) {
	now := s.clock()
	refs, err := s.clarifications.ListDueRefs(ctx, now, batch)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, ref := range refs {
		handled, err := s.expireOne(tenant.WithTenantID(ctx, ref.TenantID), ref.ID, now)
		if err != nil {
			slog.WarnContext(ctx, "clarification expiry failed; it stays open for the next sweep", slog.String("clarification_id", ref.ID), slog.Any("error", err))
			continue
		}
		if handled {
			done++
		}
	}
	return done, nil
}

func (s *ClarificationSweeper) expireOne(ctx context.Context, id string, now time.Time) (bool, error) {
	handled := false
	err := s.tx.InTx(ctx, func(txCtx context.Context) error {
		c, err := s.clarifications.LockOpenDue(txCtx, id, now)
		if err != nil || c == nil {
			return err // nil: another worker has it, or it was answered meanwhile
		}
		r, err := s.repo.Get(txCtx, c.RequestID)
		if err != nil {
			return err
		}
		if err := s.clarifications.MarkExpired(txCtx, c.ID, now); err != nil {
			return err
		}
		if _, err := s.returner.Execute(txCtx, ReturnInput{
			RequestID: r.ID, Stage: domain.ReturnStageForResume(c.Source, c.ResumeStatus), Category: domain.ReturnCategoryMissingInfo,
			Reason: "clarification_expired", ActorKind: domain.ActorKindSystem,
		}); err != nil {
			return err // rolls MarkExpired back too
		}
		users, err := s.recipients.Resolve(txCtx, r, c.Assignees)
		if err != nil {
			return err
		}
		ev, err := NewOutboxEvent(txCtx, domain.SubjectClarificationExpired, clarificationExpiredNotice(r, *c, users))
		if err != nil {
			return err
		}
		ev.OccurredAt = now
		handled = true
		return s.outbox.InsertOutboxEvent(txCtx, ev)
	})
	return handled, err
}

// RemindOnce sends the single reminder of every clarification past half of its window.
func (s *ClarificationSweeper) RemindOnce(ctx context.Context, batch int) (int, error) {
	now := s.clock()
	refs, err := s.clarifications.ListRemindableRefs(ctx, now, batch)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, ref := range refs {
		ok, err := s.remindOne(tenant.WithTenantID(ctx, ref.TenantID), ref.ID, now)
		if err != nil {
			slog.WarnContext(ctx, "clarification reminder failed", slog.String("clarification_id", ref.ID), slog.Any("error", err))
			continue
		}
		if ok {
			sent++
		}
	}
	return sent, nil
}

func (s *ClarificationSweeper) remindOne(ctx context.Context, id string, now time.Time) (bool, error) {
	sent := false
	err := s.tx.InTx(ctx, func(txCtx context.Context) error {
		claimed, err := s.clarifications.MarkReminded(txCtx, id, now)
		if err != nil || !claimed {
			return err
		}
		c, err := s.clarifications.Get(txCtx, id)
		if err != nil {
			return err
		}
		r, err := s.repo.Get(txCtx, c.RequestID)
		if err != nil {
			return err
		}
		users, err := s.recipients.Resolve(txCtx, r, c.Assignees)
		if err != nil {
			return err
		}
		// Same subject as the first notice: a reminder is a repeat with reason=reminder, not a new kind of event.
		ev, err := NewOutboxEvent(txCtx, domain.SubjectClarificationRequested, clarificationRequestedNotice(r, c, users, true))
		if err != nil {
			return err
		}
		ev.OccurredAt = now
		sent = true
		return s.outbox.InsertOutboxEvent(txCtx, ev)
	})
	return sent, err
}

// RunLoop sweeps every interval (default 60 seconds) until ctx ends.
func (s *ClarificationSweeper) RunLoop(ctx context.Context, interval time.Duration, batch int) {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	if batch <= 0 {
		batch = 50
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if n, err := s.ExpireOnce(ctx, batch); err != nil {
			slog.WarnContext(ctx, "clarification expiry sweep failed", slog.Any("error", err))
		} else if n > 0 {
			slog.InfoContext(ctx, "clarifications expired", slog.Int("count", n))
		}
		if _, err := s.RemindOnce(ctx, batch); err != nil {
			slog.WarnContext(ctx, "clarification reminder sweep failed", slog.Any("error", err))
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
