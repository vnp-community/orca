package mysql

import (
	"context"
	"errors"
	"fmt"

	driver "github.com/go-sql-driver/mysql"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// errDupEntry is MySQL's ER_DUP_ENTRY.
const errDupEntry = 1062

// InsertActiveIfAbsent inserts key as the tenant's active VAPID key unless
// idx_vapid_key_active (unique on active_tenant_id) already has one, then
// re-reads the winner. A plain INSERT with the duplicate-key error ignored
// is used instead of INSERT IGNORE, which would also swallow unrelated
// errors (truncation, CHECK violations) as warnings.
func (r *Repository) InsertActiveIfAbsent(ctx context.Context, key domain.VapidKeyMetadata) (domain.VapidKeyMetadata, bool, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO vapid_key_metadata (key_id, tenant_id, public_key, vault_key_ref, status, created_at)
		VALUES (?, ?, ?, ?, 'active', ?)
	`, key.KeyID, key.TenantID, key.PublicKey, key.VaultKeyRef, key.CreatedAt)
	inserted := true
	if err != nil {
		var myErr *driver.MySQLError
		if !errors.As(err, &myErr) || myErr.Number != errDupEntry {
			return domain.VapidKeyMetadata{}, false, fmt.Errorf("mysql: insert vapid key metadata: %w", err)
		}
		inserted = false
	}
	stored, err := r.GetPublicKey(ctx, key.TenantID)
	if err != nil {
		return domain.VapidKeyMetadata{}, false, err
	}
	return stored, inserted, nil
}
