package postgres

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type RequestLinkRepository struct {
	*Repository
}

func NewRequestLinkRepository(r *Repository) *RequestLinkRepository {
	return &RequestLinkRepository{Repository: r}
}

var _ usecase.RequestLinkRepository = (*RequestLinkRepository)(nil)

func (r *RequestLinkRepository) Insert(ctx context.Context, l domain.RequestLink) error {
	if l.ParentRequestID == l.ChildRequestID {
		return domain.ErrRequestLinkSelf()
	}
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		_, err := db.Exec(ctx, `INSERT INTO request.request_links (tenant_id, parent_request_id, child_request_id, reason, created_by)
			VALUES ($1, $2, $3, $4, $5)`, tenantID, l.ParentRequestID, l.ChildRequestID, string(l.Reason), nullIfEmpty(l.CreatedBy))
		if err != nil {
			return fmt.Errorf("postgres: insert request link: %w", err)
		}
		return nil
	})
}

func (r *RequestLinkRepository) ListParents(ctx context.Context, childID string) ([]domain.RequestLink, error) {
	return r.list(ctx, `child_request_id = $2::uuid`, childID)
}

func (r *RequestLinkRepository) ListChildren(ctx context.Context, parentID string) ([]domain.RequestLink, error) {
	return r.list(ctx, `parent_request_id = $2::uuid`, parentID)
}

func (r *RequestLinkRepository) list(ctx context.Context, cond, id string) ([]domain.RequestLink, error) {
	var out []domain.RequestLink
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.Query(ctx, `SELECT parent_request_id, child_request_id, reason, created_by, created_at FROM request.request_links
			WHERE tenant_id = $1 AND `+cond+` ORDER BY parent_request_id, child_request_id`, tenantID, id)
		if err != nil {
			return fmt.Errorf("postgres: list request links: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var l domain.RequestLink
			var reason, createdBy *string
			if err := rows.Scan(&l.ParentRequestID, &l.ChildRequestID, &reason, &createdBy, &l.CreatedAt); err != nil {
				return err
			}
			l.CreatedBy, l.CreatedAt = derefString(createdBy), l.CreatedAt.UTC()
			l.Reason = domain.LinkReason(derefString(reason))
			out = append(out, l)
		}
		return rows.Err()
	})
	return out, err
}

func (r *RequestLinkRepository) Delete(ctx context.Context, parentID, childID string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		_, err := db.Exec(ctx, `DELETE FROM request.request_links WHERE tenant_id = $1 AND parent_request_id = $2::uuid AND child_request_id = $3::uuid`,
			tenantID, parentID, childID)
		if err != nil {
			return fmt.Errorf("postgres: delete request link: %w", err)
		}
		return nil
	})
}
