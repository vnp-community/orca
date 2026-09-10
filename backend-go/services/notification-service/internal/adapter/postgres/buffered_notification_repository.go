package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// maxBufferedPerSubscription is BR-MB-07's cap — the 51st undelivered row
// for a subscription evicts the oldest, inside the same transaction as the
// insert, so no separate reaper is needed to enforce it.
const maxBufferedPerSubscription = 50

// BufferedNotificationStore implements usecase.BufferedNotificationRepository
// (BR-MB-07: offline buffering, max 50 per subscription) against
// notification.buffered_notifications (migrations/0003).
type BufferedNotificationStore struct {
	pool *pgxpool.Pool
}

func NewBufferedNotificationStore(pool *pgxpool.Pool) *BufferedNotificationStore {
	return &BufferedNotificationStore{pool: pool}
}

// Enqueue inserts eventJSON for subscriptionID and evicts the oldest
// undelivered row for that subscription once the count exceeds 50, inside
// the same transaction — no separate reaper needed for the cap itself.
func (s *BufferedNotificationStore) Enqueue(ctx context.Context, tenantID, userID, subscriptionID string, eventJSON []byte) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin buffer enqueue: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		INSERT INTO notification.buffered_notifications (tenant_id, user_id, subscription_id, notification_event_json)
		VALUES ($1,$2,$3,$4::jsonb)`, tenantID, userID, subscriptionID, eventJSON); err != nil {
		return fmt.Errorf("postgres: insert buffered notification: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM notification.buffered_notifications
		WHERE id IN (
			SELECT id FROM notification.buffered_notifications
			WHERE subscription_id = $1 AND delivered_at IS NULL
			ORDER BY buffered_at ASC
			OFFSET $2
		)`, subscriptionID, maxBufferedPerSubscription); err != nil {
		return fmt.Errorf("postgres: evict oldest buffered notification: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit buffer enqueue: %w", err)
	}
	return nil
}

// ListPending returns undelivered rows for userID, oldest first, for
// StreamNotifications reconnect draining. A row whose notification_event_json
// fails to decode (should never happen — this service is the only writer)
// is skipped rather than failing the whole list, so one corrupt row can't
// block every other pending notification from draining.
func (s *BufferedNotificationStore) ListPending(ctx context.Context, tenantID, userID string) ([]domain.BufferedNotification, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, notification_event_json
		FROM notification.buffered_notifications
		WHERE tenant_id = $1 AND user_id = $2 AND delivered_at IS NULL
		ORDER BY buffered_at ASC`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query pending buffered notifications: %w", err)
	}
	defer rows.Close()

	var out []domain.BufferedNotification
	for rows.Next() {
		var id string
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, fmt.Errorf("postgres: scan buffered notification row: %w", err)
		}
		var event domain.NotificationEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			continue
		}
		out = append(out, domain.BufferedNotification{ID: id, Event: event})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate pending buffered notifications: %w", err)
	}
	return out, nil
}

// MarkDelivered sets delivered_at = now() for the given row ids. A nil/empty
// ids slice is a no-op, not an error.
func (s *BufferedNotificationStore) MarkDelivered(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE notification.buffered_notifications SET delivered_at = now()
		WHERE id = ANY($1::uuid[])`, ids)
	if err != nil {
		return fmt.Errorf("postgres: mark buffered notifications delivered: %w", err)
	}
	return nil
}
