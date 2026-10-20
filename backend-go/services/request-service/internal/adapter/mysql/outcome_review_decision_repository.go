package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// OutcomeReviewDecisionRepository keeps the decisions people made about the items of a review.
type OutcomeReviewDecisionRepository struct {
	*Repository
}

func NewOutcomeReviewDecisionRepository(r *Repository) *OutcomeReviewDecisionRepository {
	return &OutcomeReviewDecisionRepository{Repository: r}
}

var _ usecase.OutcomeReviewDecisionRepository = (*OutcomeReviewDecisionRepository)(nil)

const reviewDecisionColumns = `id, review_id, request_id, item_key, decision, note, COALESCE(child_request_id, ''), decided_by, decided_at, active_key IS NOT NULL`

func scanReviewDecision(row interface{ Scan(dest ...any) error }) (domain.ItemDecision, error) {
	var d domain.ItemDecision
	var decision string
	if err := row.Scan(&d.ID, &d.ReviewID, &d.RequestID, &d.ItemKey, &decision, &d.Note, &d.ChildRequestID, &d.DecidedBy, &d.DecidedAt, &d.Active); err != nil {
		return domain.ItemDecision{}, err
	}
	d.Decision, d.DecidedAt = domain.ReviewDecision(decision), d.DecidedAt.UTC()
	return d, nil
}

func reviewDecisionKey(tenantID, reviewID, itemKey string) string {
	return tenantID + ":" + reviewID + ":" + itemKey
}

func (r *OutcomeReviewDecisionRepository) Record(ctx context.Context, in domain.ItemDecision) (domain.ItemDecision, error) {
	var out domain.ItemDecision
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(in.ReviewID) || notUUID(in.RequestID) || notUUID(in.DecidedBy) {
			return domain.ErrReviewDecisionInvalid("review, request and person must be ids")
		}
		key := reviewDecisionKey(tenantID, in.ReviewID, in.ItemKey)
		if _, err := db.ExecContext(ctx, `UPDATE outcome_review_decisions SET active_key = NULL WHERE tenant_id = ? AND active_key = ?`, tenantID, key); err != nil {
			return err
		}
		id := in.ID
		if id == "" {
			id = uuid.NewString()
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO outcome_review_decisions (id, tenant_id, review_id, request_id, item_key, decision, note, child_request_id, decided_by, active_key)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, tenantID, in.ReviewID, in.RequestID, in.ItemKey, string(in.Decision), in.Note, nullIfEmpty(in.ChildRequestID), in.DecidedBy, key); err != nil {
			return fmt.Errorf("mysql: record review decision: %w", err)
		}
		var err error
		out, err = scanReviewDecision(db.QueryRowContext(ctx, `SELECT `+reviewDecisionColumns+` FROM outcome_review_decisions WHERE tenant_id = ? AND id = ?`, tenantID, id))
		return err
	})
	return out, err
}

func (r *OutcomeReviewDecisionRepository) list(ctx context.Context, where string, args ...any) ([]domain.ItemDecision, error) {
	var out []domain.ItemDecision
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.QueryContext(ctx, `SELECT `+reviewDecisionColumns+` FROM outcome_review_decisions WHERE tenant_id = ? AND `+where, append([]any{tenantID}, args...)...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			d, err := scanReviewDecision(rows)
			if err != nil {
				return err
			}
			out = append(out, d)
		}
		return rows.Err()
	})
	return out, err
}

func (r *OutcomeReviewDecisionRepository) InForce(ctx context.Context, reviewID string) ([]domain.ItemDecision, error) {
	if notUUID(reviewID) {
		return nil, nil
	}
	return r.list(ctx, `review_id = ? AND active_key IS NOT NULL ORDER BY decided_at, id`, reviewID)
}

func (r *OutcomeReviewDecisionRepository) History(ctx context.Context, reviewID, itemKey string) ([]domain.ItemDecision, error) {
	if notUUID(reviewID) {
		return nil, nil
	}
	return r.list(ctx, `review_id = ? AND item_key = ? ORDER BY decided_at DESC, id DESC`, reviewID, itemKey)
}

func (r *OutcomeReviewDecisionRepository) LinkChild(ctx context.Context, reviewID string, itemKeys []string, childRequestID string) (int, error) {
	n := 0
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(reviewID) || notUUID(childRequestID) || len(itemKeys) == 0 {
			return nil
		}
		marks := strings.TrimSuffix(strings.Repeat("?,", len(itemKeys)), ",")
		args := []any{childRequestID, tenantID, reviewID}
		for _, k := range itemKeys {
			args = append(args, k)
		}
		res, err := db.ExecContext(ctx, `UPDATE outcome_review_decisions SET child_request_id = ?
			WHERE tenant_id = ? AND review_id = ? AND active_key IS NOT NULL AND decision = 'send_back' AND item_key IN (`+marks+`)`, args...)
		if err != nil {
			return err
		}
		c, _ := res.RowsAffected()
		n = int(c)
		return nil
	})
	return n, err
}
