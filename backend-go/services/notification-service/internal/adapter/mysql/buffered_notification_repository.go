package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// maxBufferedPerSubscription mirrors the Postgres adapter's cap (BR-MB-07).
const maxBufferedPerSubscription = 50

// BufferedNotificationStore implements usecase.BufferedNotificationRepository
// against MySQL/TiDB's buffered_notifications table (migrations/mysql/0003).
type BufferedNotificationStore struct {
	db *sql.DB
}

func NewBufferedNotificationStore(db *sql.DB) *BufferedNotificationStore {
	return &BufferedNotificationStore{db: db}
}

// Enqueue inserts eventJSON for subscriptionID and evicts the oldest
// undelivered row for that subscription once the count exceeds 50, inside
// one transaction — mirrors the Postgres adapter's shape exactly. Unlike
// the Postgres table, buffered_notifications.id has no DB-side default
// here (see migrations/mysql/0003's comment: MySQL's UUID()
// default-expression support is version-gated/non-deterministic, so this
// generates the id in Go instead, matching this service's dominant
// convention — every other table's id is already generated in Go).
func (s *BufferedNotificationStore) Enqueue(ctx context.Context, tenantID, userID, subscriptionID string, eventJSON []byte) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin buffer enqueue: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO buffered_notifications (id, tenant_id, user_id, subscription_id, notification_event_json)
		VALUES (?,?,?,?,?)`, uuid.NewString(), tenantID, userID, subscriptionID, eventJSON); err != nil {
		return fmt.Errorf("mysql: insert buffered notification: %w", err)
	}
	// MySQL's DELETE has no OFFSET-in-subquery-on-the-same-table
	// restriction the way some engines forbid "DELETE...WHERE id IN
	// (SELECT...FROM same_table)" — this driving subquery is wrapped in an
	// extra derived-table SELECT (`SELECT id FROM (SELECT ...) AS t`) to
	// sidestep MySQL's "You can't specify target table for update in FROM
	// clause" restriction, which DOES apply to a bare correlated subquery
	// against the table being deleted from.
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM buffered_notifications
		WHERE id IN (
			SELECT id FROM (
				SELECT id FROM buffered_notifications
				WHERE subscription_id = ? AND delivered_at IS NULL
				ORDER BY buffered_at ASC
				LIMIT 18446744073709551615 OFFSET ?
			) AS t
		)`, subscriptionID, maxBufferedPerSubscription); err != nil {
		return fmt.Errorf("mysql: evict oldest buffered notification: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit buffer enqueue: %w", err)
	}
	return nil
}

// ListPending returns undelivered rows for userID, oldest first, for
// StreamNotifications reconnect draining. A row whose
// notification_event_json fails to decode is skipped rather than failing
// the whole list — mirrors the Postgres adapter's defensive behavior.
func (s *BufferedNotificationStore) ListPending(ctx context.Context, tenantID, userID string) ([]domain.BufferedNotification, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, notification_event_json
		FROM buffered_notifications
		WHERE tenant_id = ? AND user_id = ? AND delivered_at IS NULL
		ORDER BY buffered_at ASC`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query pending buffered notifications: %w", err)
	}
	defer rows.Close()

	var out []domain.BufferedNotification
	for rows.Next() {
		var id string
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, fmt.Errorf("mysql: scan buffered notification row: %w", err)
		}
		var event domain.NotificationEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			continue
		}
		out = append(out, domain.BufferedNotification{ID: id, Event: event})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate pending buffered notifications: %w", err)
	}
	return out, nil
}

// MarkDelivered sets delivered_at = now() for the given row ids. A nil/empty
// ids slice is a no-op, not an error. Builds a dynamic IN (?,...,?) list —
// MySQL's IN () with zero placeholders is a syntax error, same pitfall
// documented for credential-broker-service's MarkSent (TASK-BE-DB-011), so
// the empty-slice guard above is what prevents it, not a driver difference.
func (s *BufferedNotificationStore) MarkDelivered(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+1)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	query := fmt.Sprintf(`UPDATE buffered_notifications SET delivered_at = NOW(6) WHERE id IN (%s)`, strings.Join(placeholders, ","))
	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mysql: mark buffered notifications delivered: %w", err)
	}
	return nil
}
