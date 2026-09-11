// Package mysql implements notification-service's SubscriptionRepository,
// VapidKeyRepository, ProcessedEventRepository, and NotificationRepository
// ports (defined in internal/usecase) against MySQL/TiDB via database/sql +
// github.com/go-sql-driver/mysql — the multi-database rollout adapter for
// CR-DB-002/CR-DB-003 (batch 2), mirroring internal/adapter/postgres's
// behavior (idempotency, tenant scoping) 1:1 against the dialect-safe
// schema created by migrations/mysql (tables `push_subscriptions`,
// `vapid_key_metadata`, `processed_events`, `notification_events`, no
// schema/database prefix — a MySQL database is the schema-equivalent
// isolation unit). See
// specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-008.md.
package mysql

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// Repository implements usecase.SubscriptionRepository,
// usecase.VapidKeyRepository, usecase.ProcessedEventRepository, and
// usecase.NotificationRepository against MySQL/TiDB via database/sql. No
// RLS equivalent exists in MySQL — every query below filters by tenant_id
// (or recipient_user_id) explicitly, which is the ONLY tenant-isolation
// enforcement for this adapter (see migrations/mysql/0001_init.up.sql's
// comment: the Postgres variant's RLS policy never actually activated
// either, so this doesn't lower the bar).
type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Save upserts a push subscription via ON DUPLICATE KEY UPDATE — MySQL's
// equivalent of Postgres's ON CONFLICT(endpoint) DO UPDATE. Re-subscribing
// to an endpoint already on file updates the row in place and reactivates
// it, rather than erroring on the endpoint's UNIQUE index.
func (r *Repository) Save(ctx context.Context, s domain.PushSubscription) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO push_subscriptions (
			id, tenant_id, user_id, channel, endpoint, p256dh_key, auth_key,
			device_label, status, created_at, updated_at, device_id
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE
			tenant_id    = VALUES(tenant_id),
			user_id      = VALUES(user_id),
			channel      = VALUES(channel),
			p256dh_key   = VALUES(p256dh_key),
			auth_key     = VALUES(auth_key),
			device_label = VALUES(device_label),
			status       = 'active',
			updated_at   = VALUES(updated_at),
			device_id    = COALESCE(VALUES(device_id), push_subscriptions.device_id)
	`,
		s.ID, s.TenantID, s.UserID, string(s.Channel), s.Endpoint, s.P256dhKey, s.AuthKey,
		s.DeviceLabel, string(s.Status), s.CreatedAt, s.CreatedAt, s.DeviceID,
	)
	if err != nil {
		return fmt.Errorf("mysql: upsert push subscription: %w", err)
	}
	return nil
}

// ListByUser returns a tenant's user's active subscriptions.
func (r *Repository) ListByUser(ctx context.Context, tenantID, userID string) ([]domain.PushSubscription, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, tenant_id, user_id, channel, endpoint, p256dh_key, auth_key,
		       device_label, status, last_used_at, created_at, updated_at, device_id
		FROM push_subscriptions
		WHERE tenant_id = ? AND user_id = ? AND status = 'active'
		ORDER BY created_at DESC
	`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query push subscriptions: %w", err)
	}
	defer rows.Close()

	var out []domain.PushSubscription
	for rows.Next() {
		var s domain.PushSubscription
		var channel, status string
		var lastUsedAt, updatedAt sql.NullTime
		if err := rows.Scan(&s.ID, &s.TenantID, &s.UserID, &channel, &s.Endpoint, &s.P256dhKey, &s.AuthKey,
			&s.DeviceLabel, &status, &lastUsedAt, &s.CreatedAt, &updatedAt, &s.DeviceID); err != nil {
			return nil, fmt.Errorf("mysql: scan push subscription row: %w", err)
		}
		s.Channel = domain.Channel(channel)
		s.Status = domain.SubscriptionStatus(status)
		if lastUsedAt.Valid {
			s.LastUsedAt = lastUsedAt.Time
		}
		if updatedAt.Valid {
			s.UpdatedAt = updatedAt.Time
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate push subscription rows: %w", err)
	}
	return out, nil
}

// DeleteByEndpoint removes the subscription row for endpoint. DELETE
// affecting 0 rows is not an error — idempotent by design (see
// usecase.SubscriptionRepository's doc comment). DELETE's RowsAffected
// counts WHERE-matched rows on every dialect (no "rows changed vs rows
// matched" ambiguity — that only applies to UPDATE/ON DUPLICATE KEY
// UPDATE), but this method doesn't read RowsAffected anyway, same as the
// Postgres adapter.
func (r *Repository) DeleteByEndpoint(ctx context.Context, endpoint string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint)
	if err != nil {
		return fmt.Errorf("mysql: delete push subscription by endpoint: %w", err)
	}
	return nil
}

// DeviceIDFor returns the paired mobile device id associated with a push
// subscription, or "" if none is paired.
func (r *Repository) DeviceIDFor(ctx context.Context, subscriptionID string) (string, error) {
	var deviceID sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT device_id FROM push_subscriptions WHERE id = ?`, subscriptionID).Scan(&deviceID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("mysql: query push subscription device id: %w", err)
	}
	if !deviceID.Valid {
		return "", nil
	}
	return deviceID.String, nil
}

// MarkExpired transitions endpoint's subscription to 'expired'. Unlike
// Save's upsert (which always forces status back to 'active' on conflict),
// this only ever moves a row toward expired. 0 rows affected
// (unknown/already-expired endpoint) is not an error — idempotent by
// design, same contract as DeleteByEndpoint. This method never branches on
// RowsAffected() (matches the Postgres adapter exactly), so go-sql-driver's
// default "rows changed, not rows matched" UPDATE counting — the pitfall
// that bit annotation-service's UpdateAnnotation (TASK-BE-DB-010) — does
// not apply here: idempotency holds under the MySQL translation regardless
// of which count the driver reports.
func (r *Repository) MarkExpired(ctx context.Context, endpoint string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE push_subscriptions
		SET status = ?, updated_at = ?
		WHERE endpoint = ?
	`, string(domain.SubscriptionExpired), time.Now().UTC(), endpoint)
	if err != nil {
		return fmt.Errorf("mysql: mark push subscription expired: %w", err)
	}
	return nil
}

// GetPublicKey returns the tenant's active VAPID key metadata row. Returns
// domain.ErrNoActiveVapidKey (not a raw sql error) when none exists.
func (r *Repository) GetPublicKey(ctx context.Context, tenantID string) (domain.VapidKeyMetadata, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT key_id, tenant_id, public_key, vault_key_ref, status, created_at, revoked_at
		FROM vapid_key_metadata
		WHERE tenant_id = ? AND status = 'active'
	`, tenantID)

	var key domain.VapidKeyMetadata
	var status string
	var revokedAt sql.NullTime
	err := row.Scan(&key.KeyID, &key.TenantID, &key.PublicKey, &key.VaultKeyRef, &status, &key.CreatedAt, &revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.VapidKeyMetadata{}, domain.ErrNoActiveVapidKey
	}
	if err != nil {
		return domain.VapidKeyMetadata{}, fmt.Errorf("mysql: query vapid key metadata: %w", err)
	}
	key.Status = domain.VapidKeyStatus(status)
	if revokedAt.Valid {
		key.RevokedAt = revokedAt.Time
	}
	return key, nil
}

// MarkProcessed atomically reserves eventID for JetStream consumer-side
// dedup via INSERT IGNORE — MySQL's equivalent of Postgres's
// INSERT ... ON CONFLICT DO NOTHING; not a racy check-then-insert.
// RowsAffected() == 0 unambiguously means "row for eventID already
// existed, INSERT IGNORE skipped it" on both dialects — the
// rows-changed-vs-rows-matched ambiguity is specific to UPDATE/ON
// DUPLICATE KEY UPDATE, not a plain INSERT IGNORE (no existing row to
// diff against), so this reads cleanly.
func (r *Repository) MarkProcessed(ctx context.Context, eventID, subject string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT IGNORE INTO processed_events (event_id, subject)
		VALUES (?, ?)
	`, eventID, subject)
	if err != nil {
		return false, fmt.Errorf("mysql: mark event processed: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("mysql: mark event processed rows affected: %w", err)
	}
	return affected == 0, nil
}

// SaveNotificationEvent persists 1 row per event.RecipientUserIDs entry —
// a plain loop of individual INSERTs, matching the Postgres adapter's
// pgx.Batch usage: that batch is pipelined for round-trip efficiency but
// was never wrapped in an explicit transaction either, so this preserves
// the same (non-atomic-across-recipients) behavior, not a stronger
// guarantee the original didn't have.
func (r *Repository) SaveNotificationEvent(ctx context.Context, event domain.NotificationEvent) error {
	for _, userID := range event.RecipientUserIDs {
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO notification_events (
				id, tenant_id, recipient_user_id, source_event_id, source_subject,
				type, title, body, deep_link, severity, is_read, created_at
			) VALUES (?,?,?,?,?,?,?,?,?,?,false,?)
		`, event.ID, event.TenantID, userID, event.SourceEventID, event.SourceSubject,
			event.Type, event.Title, event.Body, event.DeepLink, string(event.Severity), event.CreatedAt)
		if err != nil {
			return fmt.Errorf("mysql: insert notification_events row: %w", err)
		}
	}
	return nil
}

// notificationListDefaultLimit mirrors the Postgres adapter's constant of
// the same purpose.
const notificationListDefaultLimit = 20

// ListByRecipient returns cursor-paginated notification_events rows for
// tenantID+userID, newest first. Keyset pagination on
// (created_at DESC, id DESC) — not OFFSET — mirrors the Postgres adapter
// exactly, including the "fetch limit+1 to detect a next page" trick.
func (r *Repository) ListByRecipient(ctx context.Context, tenantID, userID, cursor string, limit int32, unreadOnly bool) ([]domain.NotificationEvent, string, error) {
	if limit <= 0 {
		limit = notificationListDefaultLimit
	}

	args := []any{tenantID, userID}
	var query strings.Builder
	query.WriteString(`
		SELECT id, tenant_id, recipient_user_id, source_event_id, source_subject,
		       type, title, body, deep_link, severity, is_read, read_at, created_at
		FROM notification_events
		WHERE tenant_id = ? AND recipient_user_id = ?
	`)
	if unreadOnly {
		query.WriteString(" AND is_read = false")
	}
	if cursor != "" {
		cursorCreatedAt, cursorID, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		args = append(args, cursorCreatedAt, cursorID)
		query.WriteString(" AND (created_at, id) < (?, ?)")
	}
	args = append(args, limit+1) // fetch 1 extra row to know whether a next page exists
	query.WriteString(" ORDER BY created_at DESC, id DESC LIMIT ?")

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: query notification events: %w", err)
	}
	defer rows.Close()

	var out []domain.NotificationEvent
	for rows.Next() {
		var e domain.NotificationEvent
		var recipientUserID, severity string
		var deepLink sql.NullString
		var readAt sql.NullTime
		if err := rows.Scan(&e.ID, &e.TenantID, &recipientUserID, &e.SourceEventID, &e.SourceSubject,
			&e.Type, &e.Title, &e.Body, &deepLink, &severity, &e.IsRead, &readAt, &e.CreatedAt); err != nil {
			return nil, "", fmt.Errorf("mysql: scan notification event row: %w", err)
		}
		e.RecipientUserIDs = []string{recipientUserID}
		e.Severity = domain.Severity(severity)
		if deepLink.Valid {
			e.DeepLink = deepLink.String
		}
		if readAt.Valid {
			t := readAt.Time
			e.ReadAt = &t
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate notification event rows: %w", err)
	}

	var nextCursor string
	if int32(len(out)) > limit {
		last := out[limit-1]
		nextCursor = encodeCursor(last.CreatedAt, last.ID)
		out = out[:limit]
	}
	return out, nextCursor, nil
}

// MarkAsRead sets is_read=true, read_at=now() for notificationID scoped to
// tenantID+userID. 0 rows affected (already read, or wrong id/user/tenant)
// is not an error — idempotent by design. The WHERE clause already filters
// to is_read = false, so a matched row is always a changed row here: the
// rows-changed-vs-rows-matched UPDATE ambiguity (TASK-BE-DB-010's
// UpdateAnnotation finding) can't bite this query even though this method
// doesn't read RowsAffected() at all.
func (r *Repository) MarkAsRead(ctx context.Context, tenantID, userID, notificationID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE notification_events
		SET is_read = true, read_at = ?
		WHERE tenant_id = ? AND recipient_user_id = ? AND id = ? AND is_read = false
	`, time.Now().UTC(), tenantID, userID, notificationID)
	if err != nil {
		return fmt.Errorf("mysql: mark notification as read: %w", err)
	}
	return nil
}

// MarkAllAsRead sets is_read=true, read_at=now() for every unread row of
// tenantID+userID; returns the number of rows updated. Same "WHERE already
// filters is_read = false, so matched == changed" reasoning as MarkAsRead
// makes the returned RowsAffected() count identical across dialects here.
func (r *Repository) MarkAllAsRead(ctx context.Context, tenantID, userID string) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE notification_events
		SET is_read = true, read_at = ?
		WHERE tenant_id = ? AND recipient_user_id = ? AND is_read = false
	`, time.Now().UTC(), tenantID, userID)
	if err != nil {
		return 0, fmt.Errorf("mysql: mark all notifications as read: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("mysql: mark all notifications as read rows affected: %w", err)
	}
	return affected, nil
}

// CountUnread returns the count of is_read=false rows for tenantID+userID.
func (r *Repository) CountUnread(ctx context.Context, tenantID, userID string) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `
		SELECT count(*) FROM notification_events
		WHERE tenant_id = ? AND recipient_user_id = ? AND is_read = false
	`, tenantID, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("mysql: count unread notifications: %w", err)
	}
	return count, nil
}

// encodeCursor/decodeCursor mirror the Postgres adapter's opaque keyset
// cursor exactly ("<rfc3339nano>|<id>", base64url) — the wire format is a
// repository implementation detail, not dialect-specific, so both adapters
// must agree byte-for-byte in case a caller round-trips a cursor across a
// dialect migration.
func encodeCursor(createdAt time.Time, id string) string {
	raw := createdAt.Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(cursor string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", domain.ErrInvalidCursor
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 || parts[1] == "" {
		return time.Time{}, "", domain.ErrInvalidCursor
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", domain.ErrInvalidCursor
	}
	return ts, parts[1], nil
}
