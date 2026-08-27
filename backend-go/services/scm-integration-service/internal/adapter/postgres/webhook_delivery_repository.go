package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// systemTenantID is a placeholder tenant_id for scm.webhook_delivery_log
// rows (that column is NOT NULL — migrations/0001_init.up.sql) until this
// service can resolve a real tenant_id from an inbound webhook's
// provider+repo (no such mapping exists yet — see usecase.WebhookVerifier's
// doc comment for the same gap on the verification side). A fixed, non-nil
// UUID keeps this table's existing RLS policy well-defined rather than
// widening the schema for a single caller.
const systemTenantID = "00000000-0000-0000-0000-000000000000"

// WebhookDeliveryRepository implements usecase.WebhookDeliveryStore against
// scm.webhook_delivery_log's existing (provider, delivery_id) uniqueness
// constraint — this table's first writer (BUG-PI-03).
type WebhookDeliveryRepository struct {
	pool *pgxpool.Pool
}

func NewWebhookDeliveryRepository(pool *pgxpool.Pool) *WebhookDeliveryRepository {
	return &WebhookDeliveryRepository{pool: pool}
}

var _ usecase.WebhookDeliveryStore = (*WebhookDeliveryRepository)(nil)

func (r *WebhookDeliveryRepository) Exists(ctx context.Context, provider domain.ScmProvider, deliveryID string) (bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT 1 FROM scm.webhook_delivery_log WHERE provider = $1 AND delivery_id = $2
	`, string(provider), deliveryID)
	var found int
	err := row.Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("postgres: query webhook delivery log: %w", err)
	}
	return true, nil
}

func (r *WebhookDeliveryRepository) Record(ctx context.Context, provider domain.ScmProvider, deliveryID, status string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO scm.webhook_delivery_log (id, tenant_id, provider, delivery_id, outcome)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (provider, delivery_id) DO NOTHING
	`, uuid.NewString(), systemTenantID, string(provider), deliveryID, status)
	if err != nil {
		return fmt.Errorf("postgres: insert webhook delivery log: %w", err)
	}
	return nil
}
