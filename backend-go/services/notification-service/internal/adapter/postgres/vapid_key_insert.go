package postgres

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// InsertActiveIfAbsent inserts key as the tenant's active VAPID key unless
// the partial unique index (tenant_id, status) WHERE status='active' already
// has one, then re-reads the winner so racing replicas converge.
func (r *Repository) InsertActiveIfAbsent(ctx context.Context, key domain.VapidKeyMetadata) (domain.VapidKeyMetadata, bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO notification.vapid_key_metadata (key_id, tenant_id, public_key, vault_key_ref, status, created_at)
		VALUES ($1, $2, $3, $4, 'active', $5)
		ON CONFLICT DO NOTHING
	`, key.KeyID, key.TenantID, key.PublicKey, key.VaultKeyRef, key.CreatedAt)
	if err != nil {
		return domain.VapidKeyMetadata{}, false, fmt.Errorf("postgres: insert vapid key metadata: %w", err)
	}
	stored, err := r.GetPublicKey(ctx, key.TenantID)
	if err != nil {
		return domain.VapidKeyMetadata{}, false, err
	}
	return stored, tag.RowsAffected() == 1, nil
}
