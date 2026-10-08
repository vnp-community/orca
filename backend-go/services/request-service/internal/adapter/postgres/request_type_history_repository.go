package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type RequestTypeHistoryRepository struct {
	*Repository
}

func NewRequestTypeHistoryRepository(r *Repository) *RequestTypeHistoryRepository {
	return &RequestTypeHistoryRepository{Repository: r}
}

var _ usecase.RequestTypeHistoryRepository = (*RequestTypeHistoryRepository)(nil)

func (r *RequestTypeHistoryRepository) Append(ctx context.Context, h domain.RequestTypeChange) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		at := h.At
		if at.IsZero() {
			at = time.Now().UTC()
		}
		_, err := db.Exec(ctx, `
			INSERT INTO request.request_type_history (tenant_id, request_id, at, from_type, to_type, actor_id, actor_kind, reason)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			tenantID, h.RequestID, at, nullIfEmpty(string(h.FromType)), string(h.ToType), h.ActorID, string(h.ActorKind), h.Reason)
		if err != nil {
			return fmt.Errorf("postgres: append type history: %w", err)
		}
		return nil
	})
}

func (r *RequestTypeHistoryRepository) List(ctx context.Context, requestID string) ([]domain.RequestTypeChange, error) {
	var out []domain.RequestTypeChange
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.Query(ctx, `
			SELECT request_id, at, from_type, to_type, actor_id, actor_kind, reason
			FROM request.request_type_history WHERE tenant_id = $1 AND request_id = $2::uuid ORDER BY at`, tenantID, requestID)
		if err != nil {
			return fmt.Errorf("postgres: list type history: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var h domain.RequestTypeChange
			var from *string
			var to, kind string
			if err := rows.Scan(&h.RequestID, &h.At, &from, &to, &h.ActorID, &kind, &h.Reason); err != nil {
				return err
			}
			h.FromType, h.ToType, h.ActorKind = domain.RequestType(derefString(from)), domain.RequestType(to), domain.ActorKind(kind)
			h.At = h.At.UTC()
			out = append(out, h)
		}
		return rows.Err()
	})
	return out, err
}
