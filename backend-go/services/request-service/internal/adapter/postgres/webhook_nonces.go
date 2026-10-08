package postgres

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
			return fmt.Errorf("postgres: nonce tenant differs from the context tenant")
		}
		// An expired nonce no longer blocks: replace it, otherwise keep the first sighting.
		tag, err := db.Exec(ctx, `
			INSERT INTO request.request_webhook_nonces (tenant_id, source, nonce_hash, expires_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (tenant_id, source, nonce_hash) DO UPDATE SET expires_at = EXCLUDED.expires_at
			WHERE request.request_webhook_nonces.expires_at < now()`, scopedTenant, source, nonceHash, expiresAt)
		if err != nil {
			return fmt.Errorf("postgres: remember webhook nonce: %w", err)
		}
		accepted = tag.RowsAffected() == 1
		return nil
	})
	return accepted, err
}

func (r *WebhookNonceRepository) PruneExpired(ctx context.Context, now time.Time, limit int) (int, error) {
	n := 0
	err := r.withRelayTx(ctx, func(ctx context.Context, db dbExecer) error {
		tag, err := db.Exec(ctx, `
			DELETE FROM request.request_webhook_nonces WHERE ctid IN (
				SELECT ctid FROM request.request_webhook_nonces WHERE expires_at < $1 LIMIT $2)`, now, limit)
		if err != nil {
			return fmt.Errorf("postgres: prune webhook nonces: %w", err)
		}
		n = int(tag.RowsAffected())
		return nil
	})
	return n, err
}
