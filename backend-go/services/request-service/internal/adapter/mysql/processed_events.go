package mysql

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
		// No-op update rather than INSERT IGNORE, which would also hide real errors.
		res, err := db.ExecContext(ctx, `INSERT INTO processed_events (tenant_id, event_id, subject) VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE tenant_id = tenant_id`, tenantID, eventID, subject)
		if err != nil {
			return fmt.Errorf("mysql: mark processed: %w", err)
		}
		n, _ := res.RowsAffected()
		already = n == 0
		return nil
	})
	return already, err
}

func (r *ProcessedEventRepository) Prune(ctx context.Context, olderThan time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM processed_events WHERE processed_at < ?`, olderThan.UTC())
	if err != nil {
		return 0, fmt.Errorf("mysql: prune processed events: %w", err)
	}
	return res.RowsAffected()
}
