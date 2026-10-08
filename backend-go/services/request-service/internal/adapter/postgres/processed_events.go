package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type ProcessedEventRepository struct {
	*Repository
}

func NewProcessedEventRepository(r *Repository) *ProcessedEventRepository {
	return &ProcessedEventRepository{Repository: r}
}

var _ usecase.ProcessedEventRepository = (*ProcessedEventRepository)(nil)

// MarkProcessed joins the ctx transaction so the marker commits or rolls back with the result it guards.
func (r *ProcessedEventRepository) MarkProcessed(ctx context.Context, eventID, subject string) (bool, error) {
	already := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `INSERT INTO request.processed_events (tenant_id, event_id, subject) VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING`, tenantID, eventID, subject)
		if err != nil {
			return fmt.Errorf("postgres: mark processed: %w", err)
		}
		already = tag.RowsAffected() == 0
		return nil
	})
	return already, err
}

// Prune deletes across tenants through the relay GUC; tenant policies would otherwise limit it to one tenant.
func (r *ProcessedEventRepository) Prune(ctx context.Context, olderThan time.Time) (int64, error) {
	var n int64
	err := r.withRelayTx(ctx, func(ctx context.Context, db dbExecer) error {
		tag, err := db.Exec(ctx, `DELETE FROM request.processed_events WHERE processed_at < $1`, olderThan)
		if err != nil {
			return err
		}
		n = tag.RowsAffected()
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("postgres: prune processed events: %w", err)
	}
	return n, nil
}
