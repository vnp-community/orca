package mysql

import (
	"context"
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
	query := `SELECT id, tenant_id, project_id, type, reporter_id, stage, status, returned_category, updated_at FROM requests WHERE tenant_id = ? AND status = 'request_backlog'`
	args := []any{tenantID}

	if f.ProjectID != "" {
		query += " AND project_id = ?"
		args = append(args, f.ProjectID)
	}

	if len(f.Types) > 0 {
		var placeholders []string
		for _, t := range f.Types {
			placeholders = append(placeholders, "?")
			args = append(args, t)
		}
		query += fmt.Sprintf(" AND type IN (%s)", strings.Join(placeholders, ", "))
	}

	if len(f.Categories) > 0 {
		var placeholders []string
		for _, c := range f.Categories {
			placeholders = append(placeholders, "?")
			args = append(args, c)
		}
		query += fmt.Sprintf(" AND returned_category IN (%s)", strings.Join(placeholders, ", "))
	}

	if f.Cursor != nil {
		query += " AND (updated_at < ? OR (updated_at = ? AND id < ?))"
		args = append(args, f.Cursor.UpdatedAt, f.Cursor.UpdatedAt, f.Cursor.ID)
	}

	query += " ORDER BY updated_at DESC, id DESC LIMIT ?"
	args = append(args, f.Limit+1)

	rows, err := r.exec(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql list backlog: %w", err)
	}
	defer rows.Close()

	var list []domain.Request
	for rows.Next() {
		var req domain.Request
		var proj, retCat *string
		if err := rows.Scan(&req.ID, &req.TenantID, &proj, &req.Type, &req.ReporterID, &req.Stage, &req.Status, &retCat, &req.UpdatedAt); err != nil {
			return nil, err
		}
		if proj != nil {
			req.ProjectID = *proj
		}
		if retCat != nil {
			req.ReturnedCategory = *retCat
		}
		list = append(list, req)
	}

	return list, rows.Err()
}

func (r *BacklogRequestReader) ParentRequestIDs(ctx context.Context, tenantID string, childIDs []string) (map[string][]string, error) {
	if len(childIDs) == 0 {
		return nil, nil
	}
	var placeholders []string
	var args []any
	args = append(args, tenantID)
	for _, id := range childIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	query := fmt.Sprintf(`SELECT child_request_id, parent_request_id FROM request_links WHERE tenant_id = ? AND child_request_id IN (%s)`, strings.Join(placeholders, ","))

	rows, err := r.exec(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql request_links: %w", err)
	}
	defer rows.Close()

	res := make(map[string][]string)
	for rows.Next() {
		var cID, pID string
		if err := rows.Scan(&cID, &pID); err != nil {
			return nil, err
		}
		res[cID] = append(res[cID], pID)
	}
	return res, rows.Err()
}

func (r *BacklogRequestReader) LatestReturns(ctx context.Context, tenantID string, requestIDs []string) (map[string]domain.ReturnEvent, error) {
	if len(requestIDs) == 0 {
		return nil, nil
	}
	var placeholders []string
	var args []any
	args = append(args, tenantID)
	for _, id := range requestIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	query := fmt.Sprintf(`SELECT request_id, actor_id, at FROM request_return_history WHERE tenant_id = ? AND action = 'returned' AND request_id IN (%s)`, strings.Join(placeholders, ","))

	rows, err := r.exec(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql request_return_history: %w", err)
	}
	defer rows.Close()

	res := make(map[string]domain.ReturnEvent)
	for rows.Next() {
		var ev domain.ReturnEvent
		if err := rows.Scan(&ev.RequestID, &ev.ActorID, &ev.At); err != nil {
			return nil, err
		}
		if existing, ok := res[ev.RequestID]; !ok || ev.At.After(existing.At) {
			res[ev.RequestID] = ev
		}
	}
	return res, rows.Err()
}
