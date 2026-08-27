# TASK-MB-02-06: Add `buffered_notifications` + `notification_preferences` tables and repositories

**From Solution:** SOL-MB-02
**Priority:** P1
**Service:** `notification-service`
**File:** `backend-go/services/notification-service/migrations/0003_mobile_buffering_preferences.up.sql`, `backend-go/services/notification-service/internal/adapter/postgres/buffered_notification_repository.go`, `backend-go/services/notification-service/internal/adapter/postgres/notification_preference_repository.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BR-MB-07 (offline buffering, max 50/subscription) and BR-MB-08
(per-event-type preferences) both need new tables.
`notification-service.md` §2 states per-user notification preferences are
explicitly out of scope for the legacy TS surface — this task adds the
table anyway, since BL-MB-02's BR-MB-08 requires it; flag the TDD §2
mismatch for reconciliation, don't silently pretend it doesn't exist.

## Changes to make

`backend-go/services/notification-service/migrations/0003_mobile_buffering_preferences.up.sql`:

```sql
-- BR-MB-07: offline buffering, max 50 per (tenant,user,subscription).
CREATE TABLE notification.buffered_notifications (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                UUID NOT NULL,
    user_id                  UUID NOT NULL,
    subscription_id          UUID NOT NULL REFERENCES notification.push_subscriptions(id) ON DELETE CASCADE,
    notification_event_json  JSONB NOT NULL,
    buffered_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at             TIMESTAMPTZ
);
CREATE INDEX idx_buffered_notifications_pending ON notification.buffered_notifications(subscription_id, buffered_at)
    WHERE delivered_at IS NULL;

-- BR-MB-08: per-event-type settings. Amendment to notification-service.md
-- §2's stated non-goal — flagged in this task's Context, not silently
-- overridden.
CREATE TABLE notification.notification_preferences (
    tenant_id   UUID NOT NULL,
    user_id     UUID NOT NULL,
    event_type  TEXT NOT NULL,
    channel     TEXT NOT NULL CHECK (channel IN ('ws','web','ios','android')),
    enabled     BOOLEAN NOT NULL DEFAULT true,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, user_id, event_type, channel)
);
-- Absence of a row == enabled (default-on); this table only carries
-- explicit opt-outs, never a full cross-product seed per user.
```

`.down.sql`:
```sql
DROP TABLE IF EXISTS notification.notification_preferences;
DROP TABLE IF EXISTS notification.buffered_notifications;
```

`backend-go/services/notification-service/internal/adapter/postgres/buffered_notification_repository.go`:

```go
package postgres

type BufferedNotificationStore struct{ pool *pgxpool.Pool }

func NewBufferedNotificationStore(pool *pgxpool.Pool) *BufferedNotificationStore {
	return &BufferedNotificationStore{pool: pool}
}

const maxBufferedPerSubscription = 50 // BR-MB-07

// Enqueue inserts eventJSON for subscriptionID and evicts the oldest
// undelivered row for that subscription once the count exceeds 50, inside
// the same transaction — no separate reaper needed for the cap itself.
func (s *BufferedNotificationStore) Enqueue(ctx context.Context, tenantID, userID, subscriptionID string, eventJSON []byte) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO notification.buffered_notifications (tenant_id, user_id, subscription_id, notification_event_json)
		VALUES ($1,$2,$3,$4)`, tenantID, userID, subscriptionID, eventJSON); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM notification.buffered_notifications
		WHERE id IN (
			SELECT id FROM notification.buffered_notifications
			WHERE subscription_id = $1 AND delivered_at IS NULL
			ORDER BY buffered_at ASC
			OFFSET $2
		)`, subscriptionID, maxBufferedPerSubscription); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ListPending returns undelivered rows for userID, oldest first, for
// StreamNotifications reconnect draining.
func (s *BufferedNotificationStore) ListPending(ctx context.Context, tenantID, userID string) ([]BufferedRow, error) { /* ... */ }

// MarkDelivered sets delivered_at = now() for the given row ids.
func (s *BufferedNotificationStore) MarkDelivered(ctx context.Context, ids []string) error { /* ... */ }
```

`backend-go/services/notification-service/internal/adapter/postgres/notification_preference_repository.go`:

```go
package postgres

type NotificationPreferenceStore struct{ pool *pgxpool.Pool }

func NewNotificationPreferenceStore(pool *pgxpool.Pool) *NotificationPreferenceStore {
	return &NotificationPreferenceStore{pool: pool}
}

// IsEnabled: absence of a row means enabled (default-on) — BR-MB-08.
func (s *NotificationPreferenceStore) IsEnabled(ctx context.Context, tenantID, userID, eventType, channel string) (bool, error) {
	var enabled bool
	err := s.pool.QueryRow(ctx, `
		SELECT enabled FROM notification.notification_preferences
		WHERE tenant_id=$1 AND user_id=$2 AND event_type=$3 AND channel=$4`,
		tenantID, userID, eventType, channel).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	return enabled, err
}

func (s *NotificationPreferenceStore) Set(ctx context.Context, tenantID, userID, eventType, channel string, enabled bool) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO notification.notification_preferences (tenant_id, user_id, event_type, channel, enabled, updated_at)
		VALUES ($1,$2,$3,$4,$5, now())
		ON CONFLICT (tenant_id, user_id, event_type, channel) DO UPDATE SET enabled=$5, updated_at=now()`,
		tenantID, userID, eventType, channel, enabled)
	return err
}
```

Add both as new ports (`BufferedNotificationRepository`,
`NotificationPreferenceRepository`) in `internal/usecase/ports.go`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/notification-service/... && go vet ./services/notification-service/...
go test ./services/notification-service/internal/adapter/postgres/... -run 'BufferedNotification|NotificationPreference'
```

Test: 51st `Enqueue` for one `subscriptionID` evicts the oldest undelivered
row (BR-MB-07's cap — count never exceeds 50). `IsEnabled` with no row
returns `true`; with an explicit `enabled=false` row returns `false`.
