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
	"errors"
	"fmt"
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
			device_label, status, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10)
		ON CONFLICT (endpoint) DO UPDATE SET
			tenant_id    = EXCLUDED.tenant_id,
			user_id      = EXCLUDED.user_id,
			channel      = EXCLUDED.channel,
			p256dh_key   = EXCLUDED.p256dh_key,
			auth_key     = EXCLUDED.auth_key,
			device_label = EXCLUDED.device_label,
			status       = 'active',
			updated_at   = EXCLUDED.updated_at
	`,
		s.ID, s.TenantID, s.UserID, string(s.Channel), s.Endpoint, s.P256dhKey, s.AuthKey,
		s.DeviceLabel, string(s.Status), s.CreatedAt,
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
		       device_label, status, last_used_at, created_at, updated_at
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
			&s.DeviceLabel, &status, &lastUsedAt, &s.CreatedAt, &s.UpdatedAt); err != nil {
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
