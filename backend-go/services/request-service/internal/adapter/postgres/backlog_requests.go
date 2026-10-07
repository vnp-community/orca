package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
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
	query := `SELECT id, tenant_id, project_id, type, reporter_id, stage, status, returned_category, updated_at FROM request.requests WHERE tenant_id = $1 AND status = 'request_backlog'`
	args := []any{tenantID}
	idx := 2

	if f.ProjectID != "" {
		query += fmt.Sprintf(" AND project_id = $%d", idx)
		args = append(args, f.ProjectID)
		idx++
	}

	if len(f.Types) > 0 {
		var placeholders []string
		for _, t := range f.Types {
			placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
			args = append(args, t)
			idx++
		}
		query += fmt.Sprintf(" AND type IN (%s)", strings.Join(placeholders, ", "))
	}

	if len(f.Categories) > 0 {
		var placeholders []string
		for _, c := range f.Categories {
			placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
			args = append(args, c)
			idx++
		}
		query += fmt.Sprintf(" AND returned_category IN (%s)", strings.Join(placeholders, ", "))
	}

	if f.Cursor != nil {
		query += fmt.Sprintf(" AND (updated_at < $%d OR (updated_at = $%d AND id < $%d))", idx, idx+1, idx+2)
		args = append(args, f.Cursor.UpdatedAt, f.Cursor.UpdatedAt, f.Cursor.ID)
		idx += 3
	}

	query += fmt.Sprintf(" ORDER BY updated_at DESC, id DESC LIMIT $%d", idx)
	args = append(args, f.Limit+1)

	rows, err := r.exec(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres list backlog: %w", err)
	}
	defer rows.(pgx.Rows).Close()

	var list []domain.Request
	for rows.(pgx.Rows).Next() {
		var req domain.Request
		var proj, retCat *string
		if err := rows.(pgx.Rows).Scan(&req.ID, &req.TenantID, &proj, &req.Type, &req.ReporterID, &req.Stage, &req.Status, &retCat, &req.UpdatedAt); err != nil {
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

	return list, rows.(pgx.Rows).Err()
}

func (r *BacklogRequestReader) ParentRequestIDs(ctx context.Context, tenantID string, childIDs []string) (map[string][]string, error) {
	if len(childIDs) == 0 {
		return nil, nil
	}
	var placeholders []string
	var args []any
	args = append(args, tenantID)
	idx := 2
	for _, id := range childIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
		args = append(args, id)
		idx++
	}
	query := fmt.Sprintf(`SELECT child_request_id, parent_request_id FROM request.request_links WHERE tenant_id = $1 AND child_request_id IN (%s)`, strings.Join(placeholders, ","))

	rows, err := r.exec(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres request_links: %w", err)
	}
	defer rows.(pgx.Rows).Close()

	res := make(map[string][]string)
	for rows.(pgx.Rows).Next() {
		var cID, pID string
		if err := rows.(pgx.Rows).Scan(&cID, &pID); err != nil {
			return nil, err
		}
		res[cID] = append(res[cID], pID)
	}
	return res, rows.(pgx.Rows).Err()
}

func (r *BacklogRequestReader) LatestReturns(ctx context.Context, tenantID string, requestIDs []string) (map[string]domain.ReturnEvent, error) {
	if len(requestIDs) == 0 {
		return nil, nil
	}
	var placeholders []string
	var args []any
	args = append(args, tenantID)
	idx := 2
	for _, id := range requestIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
		args = append(args, id)
		idx++
	}
	query := fmt.Sprintf(`SELECT request_id, actor_id, at FROM request.request_return_history WHERE tenant_id = $1 AND action = 'returned' AND request_id IN (%s)`, strings.Join(placeholders, ","))

	rows, err := r.exec(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres request_return_history: %w", err)
	}
	defer rows.(pgx.Rows).Close()

	res := make(map[string]domain.ReturnEvent)
	for rows.(pgx.Rows).Next() {
		var ev domain.ReturnEvent
		if err := rows.(pgx.Rows).Scan(&ev.RequestID, &ev.ActorID, &ev.At); err != nil {
			return nil, err
		}
		if existing, ok := res[ev.RequestID]; !ok || ev.At.After(existing.At) {
			res[ev.RequestID] = ev
		}
	}
	return res, rows.(pgx.Rows).Err()
}
