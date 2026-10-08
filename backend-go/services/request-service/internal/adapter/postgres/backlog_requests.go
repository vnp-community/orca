package postgres

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type BacklogRequestReader struct {
	*Repository
}

func NewBacklogRequestReader(r *Repository) *BacklogRequestReader {
	return &BacklogRequestReader{Repository: r}
}

var _ usecase.BacklogRequestReader = (*BacklogRequestReader)(nil)

func (r *BacklogRequestReader) ListReturnedRequests(ctx context.Context, tenantID string, f usecase.BacklogRequestFilter) ([]domain.Request, error) {
	return r.ListByStatus(ctx, tenantID, []domain.RequestStatus{domain.RequestStatusRequestBacklog}, f)
}

// ListByStatus is one keyset page, newest update first. The cursor is expanded (a < x OR a = x AND id < y) instead of
// a row comparison so both dialects order and page identically.
func (r *BacklogRequestReader) ListByStatus(ctx context.Context, tenantID string, statuses []domain.RequestStatus, f usecase.BacklogRequestFilter) ([]domain.Request, error) {
	args := []any{tenantID}
	next := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	statusNames := make([]string, len(statuses))
	for i, s := range statuses {
		statusNames[i] = string(s)
	}
	where := "tenant_id = $1 AND status = ANY(" + next(statusNames) + "::text[])"
	if f.ProjectID != "" {
		where += " AND project_id = " + next(f.ProjectID) + "::uuid"
	}
	if f.RequestID != "" {
		where += " AND id = " + next(f.RequestID) + "::uuid"
	}
	if len(f.ProjectIDs) > 0 {
		where += " AND project_id = ANY(" + next(f.ProjectIDs) + "::uuid[])"
	}
	if len(f.Types) > 0 {
		where += " AND type = ANY(" + next(f.Types) + "::text[])"
	}
	if len(f.Categories) > 0 {
		where += " AND returned_category = ANY(" + next(f.Categories) + "::text[])"
	}
	if f.Cursor != nil {
		at, id := next(f.Cursor.UpdatedAt), next(f.Cursor.ID)
		where += " AND (updated_at < " + at + " OR (updated_at = " + at + " AND id < " + id + "::uuid))"
	}
	query := "SELECT " + requestColumns + " FROM request.requests WHERE " + where + " ORDER BY updated_at DESC, id DESC LIMIT " + next(f.Limit+1)

	var list []domain.Request
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return domain.ErrRequestTenantRequired()
		}
		rows, err := db.Query(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("postgres list backlog: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			req, err := scanRequest(rows)
			if err != nil {
				return err
			}
			list = append(list, req)
		}
		return rows.Err()
	})
	return list, err
}

func (r *BacklogRequestReader) ParentRequestIDs(ctx context.Context, tenantID string, childIDs []string) (map[string][]string, error) {
	res := make(map[string][]string)
	if len(childIDs) == 0 {
		return res, nil
	}
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return domain.ErrRequestTenantRequired()
		}
		rows, err := db.Query(ctx, `SELECT child_request_id, parent_request_id FROM request.request_links
			WHERE tenant_id = $1 AND child_request_id = ANY($2::uuid[])`, tenantID, childIDs)
		if err != nil {
			return fmt.Errorf("postgres request_links: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var child, parent string
			if err := rows.Scan(&child, &parent); err != nil {
				return err
			}
			res[child] = append(res[child], parent)
		}
		return rows.Err()
	})
	return res, err
}

func (r *BacklogRequestReader) LatestReturns(ctx context.Context, tenantID string, requestIDs []string) (map[string]domain.ReturnEvent, error) {
	if len(requestIDs) == 0 {
		return nil, nil
	}
	res := make(map[string]domain.ReturnEvent)
	err := r.scoped(ctx, func(ctx context.Context, scopedTenant string, db dbExecer) error {
		if scopedTenant != tenantID {
			return domain.ErrRequestTenantRequired()
		}
		// Ascending order so the last row per request overwrites earlier returns.
		rows, err := db.Query(ctx, `SELECT request_id, actor_id, at FROM request.request_return_history
			WHERE tenant_id = $1 AND action = 'returned' AND request_id = ANY($2::uuid[]) ORDER BY at, id`, tenantID, requestIDs)
		if err != nil {
			return fmt.Errorf("postgres latest returns: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var ev domain.ReturnEvent
			var actor *string
			if err := rows.Scan(&ev.RequestID, &actor, &ev.At); err != nil {
				return err
			}
			ev.ActorID, ev.At = derefString(actor), ev.At.UTC()
			res[ev.RequestID] = ev
		}
		return rows.Err()
	})
	return res, err
}
