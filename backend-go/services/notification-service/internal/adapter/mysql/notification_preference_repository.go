package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// NotificationPreferenceStore implements usecase.NotificationPreferenceRepository
// (BR-MB-08: per-event-type/per-channel opt-out) against MySQL/TiDB's
// notification_preferences table (migrations/mysql/0003).
type NotificationPreferenceStore struct {
	db *sql.DB
}

func NewNotificationPreferenceStore(db *sql.DB) *NotificationPreferenceStore {
	return &NotificationPreferenceStore{db: db}
}

// IsEnabled: absence of a row means enabled (default-on) — BR-MB-08.
func (s *NotificationPreferenceStore) IsEnabled(ctx context.Context, tenantID, userID, eventType, channel string) (bool, error) {
	var enabled bool
	err := s.db.QueryRowContext(ctx, `
		SELECT enabled FROM notification_preferences
		WHERE tenant_id=? AND user_id=? AND event_type=? AND channel=?`,
		tenantID, userID, eventType, channel).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("mysql: query notification preference: %w", err)
	}
	return enabled, nil
}

// Set upserts one (event_type, channel) preference row for a user, via
// ON DUPLICATE KEY UPDATE — MySQL's equivalent of Postgres's
// ON CONFLICT(...) DO UPDATE against the same composite primary key.
func (s *NotificationPreferenceStore) Set(ctx context.Context, tenantID, userID, eventType, channel string, enabled bool) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO notification_preferences (tenant_id, user_id, event_type, channel, enabled, updated_at)
		VALUES (?,?,?,?,?, NOW(6))
		ON DUPLICATE KEY UPDATE enabled=VALUES(enabled), updated_at=NOW(6)`,
		tenantID, userID, eventType, channel, enabled)
	if err != nil {
		return fmt.Errorf("mysql: upsert notification preference: %w", err)
	}
	return nil
}
