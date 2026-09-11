package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// systemTenantID mirrors internal/adapter/postgres.systemTenantID exactly —
// see that file's doc comment for why webhook_delivery_log rows carry a
// fixed placeholder tenant_id today.
const systemTenantID = "00000000-0000-0000-0000-000000000000"

// WebhookDeliveryRepository implements usecase.WebhookDeliveryStore against
// MySQL's webhook_delivery_log — this table's first writer on this dialect
// (BUG-PI-03), mirroring internal/adapter/postgres.WebhookDeliveryRepository.
type WebhookDeliveryRepository struct {
	db *sql.DB
}

func NewWebhookDeliveryRepository(db *sql.DB) *WebhookDeliveryRepository {
	return &WebhookDeliveryRepository{db: db}
}

var _ usecase.WebhookDeliveryStore = (*WebhookDeliveryRepository)(nil)

func (r *WebhookDeliveryRepository) Exists(ctx context.Context, provider domain.ScmProvider, deliveryID string) (bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT 1 FROM webhook_delivery_log WHERE provider = ? AND delivery_id = ?
	`, string(provider), deliveryID)
	var found int
	err := row.Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("mysql: query webhook delivery log: %w", err)
	}
	return true, nil
}

// Record uses INSERT IGNORE — MySQL's equivalent of Postgres's
// `ON CONFLICT (provider, delivery_id) DO NOTHING` against the
// uq_webhook_delivery_log_provider_delivery UNIQUE key (same translation
// usage-service's own idempotent-insert path already established).
func (r *WebhookDeliveryRepository) Record(ctx context.Context, provider domain.ScmProvider, deliveryID, status string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT IGNORE INTO webhook_delivery_log (id, tenant_id, provider, delivery_id, outcome)
		VALUES (?, ?, ?, ?, ?)
	`, uuid.NewString(), systemTenantID, string(provider), deliveryID, status)
	if err != nil {
		return fmt.Errorf("mysql: insert webhook delivery log: %w", err)
	}
	return nil
}
