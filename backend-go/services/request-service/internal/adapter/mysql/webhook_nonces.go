package mysql

import (
	"context"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type WebhookNonceRepository struct{ *Repository }

func NewWebhookNonceRepository(r *Repository) *WebhookNonceRepository {
	return &WebhookNonceRepository{Repository: r}
}

var _ usecase.WebhookNonceStore = (*WebhookNonceRepository)(nil)

func (r *WebhookNonceRepository) Remember(ctx context.Context, tenantID, source, nonceHash string, expiresAt time.Time) (bool, error) {
	accepted := false
	err := r.scoped(ctx, func(ctx context.Context, scopedTenant string, db dbExecer) error {
		if tenantID != scopedTenant {
			return fmt.Errorf("mysql: nonce tenant differs from the context tenant")
		}
		// An expired nonce no longer blocks: drop it so the insert below can succeed.
		if _, err := db.ExecContext(ctx, `DELETE FROM request_webhook_nonces WHERE tenant_id = ? AND source = ? AND nonce_hash = ? AND expires_at < ?`,
			scopedTenant, source, nonceHash, time.Now().UTC()); err != nil {
			return fmt.Errorf("mysql: drop expired nonce: %w", err)
		}
		// No-op update rather than INSERT IGNORE, which would also hide real errors.
		res, err := db.ExecContext(ctx, `INSERT INTO request_webhook_nonces (tenant_id, source, nonce_hash, expires_at) VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE tenant_id = tenant_id`, scopedTenant, source, nonceHash, expiresAt.UTC())
		if err != nil {
			return fmt.Errorf("mysql: remember webhook nonce: %w", err)
		}
		n, _ := res.RowsAffected()
		accepted = n == 1
		return nil
	})
	return accepted, err
}

func (r *WebhookNonceRepository) PruneExpired(ctx context.Context, now time.Time, limit int) (int, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM request_webhook_nonces WHERE expires_at < ? LIMIT ?`, now.UTC(), limit)
	if err != nil {
		return 0, fmt.Errorf("mysql: prune webhook nonces: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
