// Package postgres implements notification-service's SubscriptionRepository
// and VapidKeyRepository ports (defined in internal/usecase) against this
// service's own PostgreSQL database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule: this is the ONLY package in notification-service
// that knows SQL exists. No private-key column exists anywhere in this
// schema, ever — see notification-service.md §5/§9.
package postgres

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// Repository implements both usecase.SubscriptionRepository and
// usecase.VapidKeyRepository against Postgres via pgx — hand-written SQL
// (see architecture/04-tech-stack.md: sqlc codegen is the eventual target;
// this scaffold hand-writes the equivalent queries directly, matching
// usage-service's reference implementation).
type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Save upserts a push subscription. Re-subscribing to an endpoint already
// on file (a browser re-registering the same Web Push endpoint) updates
// the row in place and reactivates it, rather than erroring on the
// endpoint's UNIQUE index.
func (r *Repository) Save(ctx context.Context, s domain.PushSubscription) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO notification.push_subscriptions (
			id, tenant_id, user_id, channel, endpoint, p256dh_key, auth_key,
			device_label, status, created_at, updated_at, device_id
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10,$11)
		ON CONFLICT (endpoint) DO UPDATE SET
			tenant_id    = EXCLUDED.tenant_id,
			user_id      = EXCLUDED.user_id,
			channel      = EXCLUDED.channel,
			p256dh_key   = EXCLUDED.p256dh_key,
			auth_key     = EXCLUDED.auth_key,
			device_label = EXCLUDED.device_label,
			status       = 'active',
			updated_at   = EXCLUDED.updated_at,
			device_id    = COALESCE(EXCLUDED.device_id, notification.push_subscriptions.device_id)
	`,
		s.ID, s.TenantID, s.UserID, string(s.Channel), s.Endpoint, s.P256dhKey, s.AuthKey,
		s.DeviceLabel, string(s.Status), s.CreatedAt, s.DeviceID,
	)
	if err != nil {
		return fmt.Errorf("postgres: upsert push subscription: %w", err)
	}
	return nil
}

// ListByUser returns a tenant's user's active subscriptions.
func (r *Repository) ListByUser(ctx context.Context, tenantID, userID string) ([]domain.PushSubscription, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, user_id, channel, endpoint, p256dh_key, auth_key,
		       device_label, status, last_used_at, created_at, updated_at, device_id
		FROM notification.push_subscriptions
		WHERE tenant_id = $1 AND user_id = $2 AND status = 'active'
		ORDER BY created_at DESC
	`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query push subscriptions: %w", err)
	}
	defer rows.Close()

	var out []domain.PushSubscription
	for rows.Next() {
		var s domain.PushSubscription
		var channel, status string
		var lastUsedAt *time.Time
		if err := rows.Scan(&s.ID, &s.TenantID, &s.UserID, &channel, &s.Endpoint, &s.P256dhKey, &s.AuthKey,
			&s.DeviceLabel, &status, &lastUsedAt, &s.CreatedAt, &s.UpdatedAt, &s.DeviceID); err != nil {
			return nil, fmt.Errorf("postgres: scan push subscription row: %w", err)
		}
		s.Channel = domain.Channel(channel)
		s.Status = domain.SubscriptionStatus(status)
		if lastUsedAt != nil {
			s.LastUsedAt = *lastUsedAt
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate push subscription rows: %w", err)
	}
	return out, nil
}

// DeleteByEndpoint removes the subscription row for endpoint. DELETE
// affecting 0 rows is not an error — idempotent by design (see
// usecase.SubscriptionRepository's doc comment).
func (r *Repository) DeleteByEndpoint(ctx context.Context, endpoint string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM notification.push_subscriptions WHERE endpoint = $1`, endpoint)
	if err != nil {
		return fmt.Errorf("postgres: delete push subscription by endpoint: %w", err)
	}
	return nil
}

// DeviceIDFor returns the paired mobile device id associated with a push
// subscription, or "" if none is paired (a standard Web Push subscription
// with no BL-MB-01 mobile pairing).
func (r *Repository) DeviceIDFor(ctx context.Context, subscriptionID string) (string, error) {
	var deviceID *string
	err := r.pool.QueryRow(ctx, `SELECT device_id FROM notification.push_subscriptions WHERE id = $1`, subscriptionID).Scan(&deviceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("postgres: query push subscription device id: %w", err)
	}
	if deviceID == nil {
		return "", nil
	}
	return *deviceID, nil
}

// MarkExpired transitions endpoint's subscription to 'expired'. Unlike
// Save's upsert (which always forces status back to 'active' on conflict),
// this only ever moves a row toward expired — a device token APNs/FCM
// reports dead stays dead until Subscribe re-registers it. 0 rows affected
// (unknown/already-expired endpoint) is not an error, same idempotent
// contract as DeleteByEndpoint.
func (r *Repository) MarkExpired(ctx context.Context, endpoint string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE notification.push_subscriptions
		SET status = $2, updated_at = now()
		WHERE endpoint = $1
	`, endpoint, string(domain.SubscriptionExpired))
	if err != nil {
		return fmt.Errorf("postgres: mark push subscription expired: %w", err)
	}
	return nil
}

// GetPublicKey returns the tenant's active VAPID key metadata row.
// Returns domain.ErrNoActiveVapidKey (not a raw pgx error) when none
// exists, so usecase/ can map it to a NotFound status without depending
// on pgx.
func (r *Repository) GetPublicKey(ctx context.Context, tenantID string) (domain.VapidKeyMetadata, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT key_id, tenant_id, public_key, vault_key_ref, status, created_at, revoked_at
		FROM notification.vapid_key_metadata
		WHERE tenant_id = $1 AND status = 'active'
	`, tenantID)

	var key domain.VapidKeyMetadata
	var status string
	var revokedAt *time.Time
	err := row.Scan(&key.KeyID, &key.TenantID, &key.PublicKey, &key.VaultKeyRef, &status, &key.CreatedAt, &revokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.VapidKeyMetadata{}, domain.ErrNoActiveVapidKey
	}
	if err != nil {
		return domain.VapidKeyMetadata{}, fmt.Errorf("postgres: query vapid key metadata: %w", err)
	}
	key.Status = domain.VapidKeyStatus(status)
	if revokedAt != nil {
		key.RevokedAt = *revokedAt
	}
	return key, nil
}

// MarkProcessed atomically reserves eventID for JetStream consumer-side
// dedup (notification-service.md §5/§8) via a single
// INSERT ... ON CONFLICT DO NOTHING — not a racy check-then-insert, so
// concurrent replicas racing the same redelivered message (each with its
// own independent SubscribeEphemeral consumer) agree on exactly one
// winner. RowsAffected() == 0 means a row for eventID already existed, i.e.
// this call lost the race / saw a redelivery.
func (r *Repository) MarkProcessed(ctx context.Context, eventID, subject string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO notification.processed_events (event_id, subject)
		VALUES ($1, $2)
		ON CONFLICT (event_id) DO NOTHING
	`, eventID, subject)
	if err != nil {
		return false, fmt.Errorf("postgres: mark event processed: %w", err)
	}
	return tag.RowsAffected() == 0, nil
}

// SaveNotificationEvent persists 1 row per event.RecipientUserIDs entry —
// implements usecase.NotificationRepository.SaveNotificationEvent. Named
// SaveNotificationEvent, not Save, because Repository already has a
// Save(ctx, domain.PushSubscription) method for SubscriptionRepository —
// see usecase.NotificationRepository's doc comment.
func (r *Repository) SaveNotificationEvent(ctx context.Context, event domain.NotificationEvent) error {
	batch := &pgx.Batch{}
	for _, userID := range event.RecipientUserIDs {
		batch.Queue(`
			INSERT INTO notification.notification_events (
				id, tenant_id, recipient_user_id, source_event_id, source_subject,
				type, title, body, deep_link, severity, is_read, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,false,$11)
		`, event.ID, event.TenantID, userID, event.SourceEventID, event.SourceSubject,
			event.Type, event.Title, event.Body, event.DeepLink, string(event.Severity), event.CreatedAt)
	}
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range event.RecipientUserIDs {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("postgres: insert notification_events row: %w", err)
		}
	}
	return nil
}

// notificationListDefaultLimit is used when the caller passes limit <= 0 —
// guards against a malformed/zero LIMIT reaching Postgres as an error
// rather than a sane default page size.
const notificationListDefaultLimit = 20

// ListByRecipient returns cursor-paginated notification_events rows for
// tenantID+userID, newest first, hitting
// idx_notification_events_recipient_unread (tenant_id, recipient_user_id,
// is_read, created_at DESC). Keyset pagination on (created_at DESC, id
// DESC) — not OFFSET — so paging deep into a growing table never rescans
// skipped rows.
func (r *Repository) ListByRecipient(ctx context.Context, tenantID, userID, cursor string, limit int32, unreadOnly bool) ([]domain.NotificationEvent, string, error) {
	if limit <= 0 {
		limit = notificationListDefaultLimit
	}

	args := []any{tenantID, userID}
	var query strings.Builder
	query.WriteString(`
		SELECT id, tenant_id, recipient_user_id, source_event_id, source_subject,
		       type, title, body, deep_link, severity, is_read, read_at, created_at
		FROM notification.notification_events
		WHERE tenant_id = $1 AND recipient_user_id = $2
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
		query.WriteString(fmt.Sprintf(" AND (created_at, id) < ($%d, $%d::uuid)", len(args)-1, len(args)))
	}
	args = append(args, limit+1) // fetch 1 extra row to know whether a next page exists
	query.WriteString(fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", len(args)))

	rows, err := r.pool.Query(ctx, query.String(), args...)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query notification events: %w", err)
	}
	defer rows.Close()

	var out []domain.NotificationEvent
	for rows.Next() {
		var e domain.NotificationEvent
		var recipientUserID, severity string
		var deepLink *string
		var readAt *time.Time
		if err := rows.Scan(&e.ID, &e.TenantID, &recipientUserID, &e.SourceEventID, &e.SourceSubject,
			&e.Type, &e.Title, &e.Body, &deepLink, &severity, &e.IsRead, &readAt, &e.CreatedAt); err != nil {
			return nil, "", fmt.Errorf("postgres: scan notification event row: %w", err)
		}
		e.RecipientUserIDs = []string{recipientUserID}
		e.Severity = domain.Severity(severity)
		if deepLink != nil {
			e.DeepLink = *deepLink
		}
		e.ReadAt = readAt
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate notification event rows: %w", err)
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
// is not an error — idempotent by design, mirrors DeleteByEndpoint.
func (r *Repository) MarkAsRead(ctx context.Context, tenantID, userID, notificationID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE notification.notification_events
		SET is_read = true, read_at = now()
		WHERE tenant_id = $1 AND recipient_user_id = $2 AND id = $3 AND is_read = false
	`, tenantID, userID, notificationID)
	if err != nil {
		return fmt.Errorf("postgres: mark notification as read: %w", err)
	}
	return nil
}

// MarkAllAsRead sets is_read=true, read_at=now() for every unread row of
// tenantID+userID; returns the number of rows updated (0 is not an error).
func (r *Repository) MarkAllAsRead(ctx context.Context, tenantID, userID string) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE notification.notification_events
		SET is_read = true, read_at = now()
		WHERE tenant_id = $1 AND recipient_user_id = $2 AND is_read = false
	`, tenantID, userID)
	if err != nil {
		return 0, fmt.Errorf("postgres: mark all notifications as read: %w", err)
	}
	return tag.RowsAffected(), nil
}

// CountUnread returns the count of is_read=false rows for tenantID+userID.
func (r *Repository) CountUnread(ctx context.Context, tenantID, userID string) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM notification.notification_events
		WHERE tenant_id = $1 AND recipient_user_id = $2 AND is_read = false
	`, tenantID, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("postgres: count unread notifications: %w", err)
	}
	return count, nil
}

// encodeCursor/decodeCursor implement ListByRecipient's opaque keyset
// cursor as "<rfc3339nano>|<id>", base64url-wrapped so the wire value stays
// opaque to callers per NotificationRepository's doc comment.
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
