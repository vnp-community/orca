package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// permanentFailureLogEvery throttles the error log of a row that fails on every 2s poll until someone fixes the cause.
const permanentFailureLogEvery = time.Minute

// ApprovalNotificationStore decorates the outbox store so approval events are enriched (recipients, title, body,
// deep link) right before the relay publishes them. The relay is shared with other events, so only
// orca.request.approval.* rows are touched.
type ApprovalNotificationStore struct {
	outbox.Store
	Notifier *usecase.PublishApprovalNotifications
	Log      *slog.Logger

	mu         sync.Mutex
	lastLogged map[string]time.Time
	now        func() time.Time // tests only; nil means time.Now
}

var _ outbox.Store = (*ApprovalNotificationStore)(nil)

// FetchUnpublished enriches approval rows in order. Two kinds of enrichment failure are treated differently:
//   - temporary (directory down, timeout): the batch stops at that row, so later rows wait and delivery order holds;
//     the relay retries the whole batch on its next poll.
//   - not retryable (directory refuses our credentials, is not configured, team gone): retrying cannot help, so the
//     row is left unpublished and out of the batch instead of freezing every other event behind it. Later events of
//     the same approval wait with it (requested before decided); other approvals and unrelated events go on. The
//     row stays in the outbox and is delivered once the cause is fixed. (A batch made only of such rows still
//     hides rows beyond the batch limit; the loud log is how that gets noticed.)
//
// An unparsable payload is published unchanged.
func (s *ApprovalNotificationStore) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	records, err := s.Store.FetchUnpublished(ctx, limit)
	if err != nil || s.Notifier == nil {
		return records, err
	}
	log := s.Log
	if log == nil {
		log = slog.Default()
	}
	ready := make([]outbox.Record, 0, len(records))
	heldApprovals := map[string]bool{}
	for i := range records {
		rec := records[i]
		if !s.Notifier.Handles(rec.Subject) {
			ready = append(ready, rec)
			continue
		}
		approvalID := approvalIDOf(rec.Event.Payload)
		if approvalID != "" && heldApprovals[approvalID] {
			continue
		}
		enriched, err := s.Notifier.Enrich(tenant.WithTenantID(ctx, rec.Event.TenantID), rec.Subject, rec.Event.Payload)
		switch {
		case err == nil:
			rec.Event.Payload = enriched
			ready = append(ready, rec)
		case errors.Is(err, usecase.ErrNotEnrichable):
			log.Warn("approval event payload not enrichable; publishing as is", slog.String("id", rec.ID))
			ready = append(ready, rec)
		case usecase.IsNotRetryable(err):
			s.logNotRetryable(log, rec, err)
			if approvalID != "" {
				heldApprovals[approvalID] = true
			}
		default:
			log.Warn("approval notification enrichment failed; will retry", slog.String("id", rec.ID), slog.Any("error", err))
			return ready, nil
		}
	}
	return ready, nil
}

func (s *ApprovalNotificationStore) logNotRetryable(log *slog.Logger, rec outbox.Record, err error) {
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastLogged == nil {
		s.lastLogged = map[string]time.Time{}
	}
	if at, ok := s.lastLogged[rec.ID]; ok && now().Sub(at) < permanentFailureLogEvery {
		return
	}
	s.lastLogged[rec.ID] = now()
	for id, at := range s.lastLogged { // rows fixed since then must not pile up
		if now().Sub(at) > 10*permanentFailureLogEvery {
			delete(s.lastLogged, id)
		}
	}
	log.Error("approval notification cannot be enriched until the directory configuration is fixed; event stays in the outbox, other events continue",
		slog.String("id", rec.ID), slog.String("subject", rec.Subject), slog.Any("error", err))
}

func approvalIDOf(payload []byte) string {
	var p struct {
		ApprovalID string `json:"approval_id"`
	}
	_ = json.Unmarshal(payload, &p)
	return p.ApprovalID
}
