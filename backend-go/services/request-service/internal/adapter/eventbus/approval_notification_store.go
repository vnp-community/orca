package eventbus

import (
	"context"
	"errors"
	"log/slog"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// ApprovalNotificationStore decorates the outbox store so approval events are enriched (recipients, title, body,
// deep link) right before the relay publishes them. The relay is shared with other events, so only
// orca.request.approval.* rows are touched.
type ApprovalNotificationStore struct {
	outbox.Store
	Notifier *usecase.PublishApprovalNotifications
	Log      *slog.Logger
}

var _ outbox.Store = (*ApprovalNotificationStore)(nil)

// FetchUnpublished stops the batch at the first row whose directory lookup failed: later rows wait so delivery order
// holds, and the relay retries the whole batch on its next poll. An unparsable payload is published unchanged.
func (s *ApprovalNotificationStore) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	records, err := s.Store.FetchUnpublished(ctx, limit)
	if err != nil || s.Notifier == nil {
		return records, err
	}
	log := s.Log
	if log == nil {
		log = slog.Default()
	}
	for i := range records {
		rec := &records[i]
		if !s.Notifier.Handles(rec.Subject) {
			continue
		}
		enriched, err := s.Notifier.Enrich(tenant.WithTenantID(ctx, rec.Event.TenantID), rec.Subject, rec.Event.Payload)
		switch {
		case err == nil:
			rec.Event.Payload = enriched
		case errors.Is(err, usecase.ErrNotEnrichable):
			log.Warn("approval event payload not enrichable; publishing as is", slog.String("id", rec.ID))
		default:
			log.Warn("approval notification enrichment failed; will retry", slog.String("id", rec.ID), slog.Any("error", err))
			return records[:i], nil
		}
	}
	return records, nil
}
