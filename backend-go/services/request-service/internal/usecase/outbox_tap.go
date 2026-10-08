package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// OutboxTap observes events that were written to the outbox, after their transaction committed.
// Taps must be cheap and must not fail the caller: audit and metrics are derived from events so the
// use cases that emit them stay unaware.
type OutboxTap interface {
	OnEvent(ctx context.Context, ev domain.OutboxEvent)
}

// TappedOutbox forwards to Inner and then notifies the taps after commit (see AfterCommit).
type TappedOutbox struct {
	Inner OutboxWriter
	Taps  []OutboxTap
}

var _ OutboxWriter = (*TappedOutbox)(nil)

func (o *TappedOutbox) InsertOutboxEvent(ctx context.Context, ev domain.OutboxEvent) error {
	if err := o.Inner.InsertOutboxEvent(ctx, ev); err != nil {
		return err
	}
	AfterCommit(ctx, func() {
		for _, t := range o.Taps {
			t.OnEvent(ctx, ev)
		}
	})
	return nil
}
