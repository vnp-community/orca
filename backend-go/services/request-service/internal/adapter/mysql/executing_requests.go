package mysql

import (
	"context"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type ExecutingRequestRepository struct {
	*Repository
}

func NewExecutingRequestRepository(r *Repository) *ExecutingRequestRepository {
	return &ExecutingRequestRepository{Repository: r}
}

var (
	_ usecase.ExecutingRequestScanner = (*ExecutingRequestRepository)(nil)
	_ usecase.ReconcileLeases         = (*ExecutingRequestRepository)(nil)
)

// ListQuietExecuting picks candidates across tenants (MySQL has no RLS to lift). The per-Request lease below,
// not this read, is what keeps two replicas apart.
func (r *ExecutingRequestRepository) ListQuietExecuting(ctx context.Context, quietFor time.Duration, limit int) ([]usecase.ExecutingRef, error) {
	if limit <= 0 {
		limit = 50
	}
	micros := quietFor.Microseconds()
	rows, err := r.exec(ctx).QueryContext(ctx, `
		SELECT r.tenant_id, r.id FROM requests r
		WHERE r.status = 'executing' AND r.updated_at < TIMESTAMPADD(MICROSECOND, -?, CURRENT_TIMESTAMP(6))
		  AND NOT EXISTS (SELECT 1 FROM task_run_outcomes o
		      WHERE o.tenant_id = r.tenant_id AND o.request_id = r.id AND o.cause <> 'dispatch_error'
		        AND o.occurred_at >= TIMESTAMPADD(MICROSECOND, -?, CURRENT_TIMESTAMP(6)))
		  AND NOT EXISTS (SELECT 1 FROM execution_reconcile_state s
		      WHERE s.tenant_id = r.tenant_id AND s.request_id = r.id
		        AND (s.lease_until > CURRENT_TIMESTAMP(6) OR s.last_run_at >= TIMESTAMPADD(MICROSECOND, -?, CURRENT_TIMESTAMP(6))))
		ORDER BY r.updated_at LIMIT ?`, micros, micros, micros, limit)
	if err != nil {
		return nil, fmt.Errorf("mysql: list quiet executing requests: %w", err)
	}
	defer rows.Close()
	var refs []usecase.ExecutingRef
	for rows.Next() {
		var ref usecase.ExecutingRef
		if err := rows.Scan(&ref.TenantID, &ref.RequestID); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

// Claim is a compare-and-set on the lease row: exactly one of two racing callers sees one row changed.
func (r *ExecutingRequestRepository) Claim(ctx context.Context, requestID, owner string, lease, quiet time.Duration) (bool, error) {
	claimed := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, err := db.ExecContext(ctx, `INSERT INTO execution_reconcile_state (tenant_id, request_id) VALUES (?, ?)
			ON DUPLICATE KEY UPDATE tenant_id = tenant_id`, tenantID, requestID); err != nil {
			return err
		}
		res, err := db.ExecContext(ctx, `UPDATE execution_reconcile_state
			SET lease_owner = ?, lease_until = TIMESTAMPADD(MICROSECOND, ?, CURRENT_TIMESTAMP(6))
			WHERE tenant_id = ? AND request_id = ? AND lease_until < CURRENT_TIMESTAMP(6)
			  AND (last_run_at IS NULL OR last_run_at < TIMESTAMPADD(MICROSECOND, -?, CURRENT_TIMESTAMP(6)))`,
			owner, lease.Microseconds(), tenantID, requestID, quiet.Microseconds())
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		claimed = n == 1
		return err
	})
	if err != nil {
		return false, fmt.Errorf("mysql: claim reconcile lease: %w", err)
	}
	return claimed, nil
}

func (r *ExecutingRequestRepository) Release(ctx context.Context, requestID, owner string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		_, err := db.ExecContext(ctx, `UPDATE execution_reconcile_state SET lease_until = '2000-01-01 00:00:01', last_run_at = CURRENT_TIMESTAMP(6)
			WHERE tenant_id = ? AND request_id = ? AND lease_owner = ?`, tenantID, requestID, owner)
		return err
	})
}
