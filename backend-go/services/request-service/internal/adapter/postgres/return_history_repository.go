package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type ReturnHistoryRepository struct {
	*Repository
}

func NewReturnHistoryRepository(r *Repository) *ReturnHistoryRepository {
	return &ReturnHistoryRepository{Repository: r}
}

var _ usecase.ReturnHistoryRepository = (*ReturnHistoryRepository)(nil)

func (r *ReturnHistoryRepository) Append(ctx context.Context, e domain.ReturnHistoryEntry) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		id := e.ID
		if id == "" {
			id = uuid.NewString()
		}
		at := e.At
		if at.IsZero() {
			at = time.Now().UTC()
		}
		_, err := db.Exec(ctx, `
			INSERT INTO request.request_return_history (id, tenant_id, request_id, action, stage, category, reason, actor_id, actor_kind, at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			id, tenantID, e.RequestID, string(e.Action), nullIfEmpty(string(e.Stage)), nullIfEmpty(string(e.Category)),
			e.Reason, nullIfEmpty(e.ActorID), string(e.ActorKind), at)
		if err != nil {
			return fmt.Errorf("postgres: append return history: %w", err)
		}
		return nil
	})
}

func (r *ReturnHistoryRepository) List(ctx context.Context, requestID string) ([]domain.ReturnHistoryEntry, error) {
	var out []domain.ReturnHistoryEntry
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.Query(ctx, `
			SELECT id, request_id, action, stage, category, reason, actor_id, actor_kind, at
			FROM request.request_return_history WHERE tenant_id = $1 AND request_id = $2::uuid ORDER BY at, id`, tenantID, requestID)
		if err != nil {
			return fmt.Errorf("postgres: list return history: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var e domain.ReturnHistoryEntry
			var action, kind string
			var stage, category, actor *string
			var at time.Time
			if err := rows.Scan(&e.ID, &e.RequestID, &action, &stage, &category, &e.Reason, &actor, &kind, &at); err != nil {
				return err
			}
			e.Action, e.ActorKind, e.At = domain.ReturnAction(action), domain.ActorKind(kind), at.UTC()
			e.Stage, e.Category, e.ActorID = domain.ReturnStage(derefString(stage)), domain.ReturnCategory(derefString(category)), derefString(actor)
			out = append(out, e)
		}
		return rows.Err()
	})
	return out, err
}
