package postgres

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

// ListQuietExecuting reads across tenants under the relay setting (policy relay_scan, SELECT only). It only picks
// candidates; the lease taken per Request under its own tenant is what keeps two replicas apart.
func (r *ExecutingRequestRepository) ListQuietExecuting(ctx context.Context, quietFor time.Duration, limit int) ([]usecase.ExecutingRef, error) {
	if limit <= 0 {
		limit = 50
	}
	var refs []usecase.ExecutingRef
	err := r.withRelayTx(ctx, func(ctx context.Context, db dbExecer) error {
		rows, err := db.Query(ctx, `
			SELECT r.tenant_id, r.id FROM request.requests r
			WHERE r.status = 'executing' AND r.updated_at < now() - make_interval(secs => $1)
			  AND NOT EXISTS (SELECT 1 FROM request.task_run_outcomes o
			      WHERE o.tenant_id = r.tenant_id AND o.request_id = r.id AND o.cause <> 'dispatch_error'
			        AND o.occurred_at >= now() - make_interval(secs => $1))
			  AND NOT EXISTS (SELECT 1 FROM request.execution_reconcile_state s
			      WHERE s.tenant_id = r.tenant_id AND s.request_id = r.id
			        AND (s.lease_until > now() OR s.last_run_at >= now() - make_interval(secs => $1)))
			ORDER BY r.updated_at LIMIT $2`, quietFor.Seconds(), limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var ref usecase.ExecutingRef
			if err := rows.Scan(&ref.TenantID, &ref.RequestID); err != nil {
				return err
			}
			refs = append(refs, ref)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: list quiet executing requests: %w", err)
	}
	return refs, nil
}

// Claim is a compare-and-set on the lease row: exactly one of two racing callers sees one row updated.
func (r *ExecutingRequestRepository) Claim(ctx context.Context, requestID, owner string, lease, quiet time.Duration) (bool, error) {
	claimed := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, err := db.Exec(ctx, `INSERT INTO request.execution_reconcile_state (tenant_id, request_id) VALUES ($1, $2)
			ON CONFLICT (tenant_id, request_id) DO NOTHING`, tenantID, requestID); err != nil {
			return err
		}
		tag, err := db.Exec(ctx, `UPDATE request.execution_reconcile_state
			SET lease_owner = $3, lease_until = now() + make_interval(secs => $4)
			WHERE tenant_id = $1 AND request_id = $2::uuid AND lease_until < now()
			  AND (last_run_at IS NULL OR last_run_at < now() - make_interval(secs => $5))`,
			tenantID, requestID, owner, lease.Seconds(), quiet.Seconds())
		if err != nil {
			return err
		}
		claimed = tag.RowsAffected() == 1
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("postgres: claim reconcile lease: %w", err)
	}
	return claimed, nil
}

func (r *ExecutingRequestRepository) Release(ctx context.Context, requestID, owner string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		_, err := db.Exec(ctx, `UPDATE request.execution_reconcile_state SET lease_until = '2000-01-01', last_run_at = now()
			WHERE tenant_id = $1 AND request_id = $2::uuid AND lease_owner = $3`, tenantID, requestID, owner)
		return err
	})
}
