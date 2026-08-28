package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NotificationPreferenceStore implements usecase.NotificationPreferenceRepository
// (BR-MB-08: per-event-type/per-channel opt-out) against
// notification.notification_preferences (migrations/0003).
type NotificationPreferenceStore struct {
	pool *pgxpool.Pool
}

func NewNotificationPreferenceStore(pool *pgxpool.Pool) *NotificationPreferenceStore {
	return &NotificationPreferenceStore{pool: pool}
}

// IsEnabled: absence of a row means enabled (default-on) — BR-MB-08. This
// table only ever carries explicit opt-outs, never a full cross-product
// seed per user.
func (s *NotificationPreferenceStore) IsEnabled(ctx context.Context, tenantID, userID, eventType, channel string) (bool, error) {
	var enabled bool
	err := s.pool.QueryRow(ctx, `
		SELECT enabled FROM notification.notification_preferences
		WHERE tenant_id=$1 AND user_id=$2 AND event_type=$3 AND channel=$4`,
		tenantID, userID, eventType, channel).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("postgres: query notification preference: %w", err)
	}
	return enabled, nil
}

// Set upserts one (event_type, channel) preference row for a user.
func (s *NotificationPreferenceStore) Set(ctx context.Context, tenantID, userID, eventType, channel string, enabled bool) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO notification.notification_preferences (tenant_id, user_id, event_type, channel, enabled, updated_at)
		VALUES ($1,$2,$3,$4,$5, now())
		ON CONFLICT (tenant_id, user_id, event_type, channel) DO UPDATE SET enabled=$5, updated_at=now()`,
		tenantID, userID, eventType, channel, enabled)
	if err != nil {
		return fmt.Errorf("postgres: upsert notification preference: %w", err)
	}
	return nil
}
