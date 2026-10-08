package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

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
	in := func(values []string) string {
		ph := make([]string, len(values))
		for i, v := range values {
			ph[i] = "?"
			args = append(args, v)
		}
		return strings.Join(ph, ", ")
	}
	statusNames := make([]string, len(statuses))
	for i, s := range statuses {
		statusNames[i] = string(s)
	}
	where := "tenant_id = ? AND status IN (" + in(statusNames) + ")"
	if f.ProjectID != "" {
		where += " AND project_id = ?"
		args = append(args, f.ProjectID)
	}
	if f.RequestID != "" {
		where += " AND id = ?"
		args = append(args, f.RequestID)
	}
	if len(f.ProjectIDs) > 0 {
		where += " AND project_id IN (" + in(f.ProjectIDs) + ")"
	}
	if len(f.Types) > 0 {
		where += " AND type IN (" + in(f.Types) + ")"
	}
	if len(f.Categories) > 0 {
		where += " AND returned_category IN (" + in(f.Categories) + ")"
	}
	if f.Cursor != nil {
		where += " AND (updated_at < ? OR (updated_at = ? AND id < ?))"
		args = append(args, f.Cursor.UpdatedAt, f.Cursor.UpdatedAt, f.Cursor.ID)
	}
	args = append(args, f.Limit+1)

	var list []domain.Request
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return domain.ErrRequestTenantRequired()
		}
		rows, err := db.QueryContext(ctx, "SELECT "+requestColumns+" FROM requests WHERE "+where+" ORDER BY updated_at DESC, id DESC LIMIT ?", args...)
		if err != nil {
			return fmt.Errorf("mysql list backlog: %w", err)
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
		args := []any{tenantID}
		ph := make([]string, len(childIDs))
		for i, id := range childIDs {
			ph[i] = "?"
			args = append(args, id)
		}
		rows, err := db.QueryContext(ctx, fmt.Sprintf(`SELECT child_request_id, parent_request_id FROM request_links
			WHERE tenant_id = ? AND child_request_id IN (%s)`, strings.Join(ph, ",")), args...)
		if err != nil {
			return fmt.Errorf("mysql request_links: %w", err)
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
		ph := make([]string, len(requestIDs))
		args := []any{tenantID}
		for i, id := range requestIDs {
			ph[i] = "?"
			args = append(args, id)
		}
		// Ascending order so the last row per request overwrites earlier returns.
		rows, err := db.QueryContext(ctx, fmt.Sprintf(`SELECT request_id, actor_id, at FROM request_return_history
			WHERE tenant_id = ? AND action = 'returned' AND request_id IN (%s) ORDER BY at, id`, strings.Join(ph, ",")), args...)
		if err != nil {
			return fmt.Errorf("mysql latest returns: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var ev domain.ReturnEvent
			var actor sql.NullString
			if err := rows.Scan(&ev.RequestID, &actor, &ev.At); err != nil {
				return err
			}
			ev.ActorID, ev.At = actor.String, ev.At.UTC()
			res[ev.RequestID] = ev
		}
		return rows.Err()
	})
	return res, err
}
