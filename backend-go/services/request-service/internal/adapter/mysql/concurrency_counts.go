package mysql

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type ConcurrencyCountRepository struct{ *Repository }

func NewConcurrencyCountRepository(r *Repository) *ConcurrencyCountRepository {
	return &ConcurrencyCountRepository{Repository: r}
}

var _ usecase.ConcurrencyCounter = (*ConcurrencyCountRepository)(nil)

// CountRunning counts analysis runs in flight for the tenant, or for one project through the Request.
func (r *ConcurrencyCountRepository) CountRunning(ctx context.Context, projectID string) (int, error) {
	n := 0
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if projectID == "" {
			return db.QueryRowContext(ctx, `SELECT COUNT(*) FROM analysis_runs WHERE tenant_id = ? AND status = 'running'`, tenantID).Scan(&n)
		}
		return db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM analysis_runs a
			JOIN requests q ON q.tenant_id = a.tenant_id AND q.id = a.request_id
			WHERE a.tenant_id = ? AND a.status = 'running' AND q.project_id = ?`, tenantID, projectID).Scan(&n)
	})
	if err != nil {
		return 0, fmt.Errorf("mysql: count running runs: %w", err)
	}
	return n, nil
}

func (r *ConcurrencyCountRepository) CountOpenRequests(ctx context.Context) (int, error) {
	n := 0
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		return db.QueryRowContext(ctx, `SELECT COUNT(*) FROM requests WHERE tenant_id = ? AND status NOT IN ('completed','cancelled')`, tenantID).Scan(&n)
	})
	if err != nil {
		return 0, fmt.Errorf("mysql: count open requests: %w", err)
	}
	return n, nil
}
