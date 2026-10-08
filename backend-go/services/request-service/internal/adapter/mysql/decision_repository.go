package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type DecisionRepository struct {
	*Repository
}

func NewDecisionRepository(r *Repository) *DecisionRepository {
	return &DecisionRepository{Repository: r}
}

var _ usecase.DecisionRepository = (*DecisionRepository)(nil)

const decisionColumns = `id, tenant_id, request_id, seq, subject_kind, subject_id, subject_digest, question, options, recommended_option_id,
	recommendation_reason, chosen_option_id, chooser_id, chosen_at, rationale, risk_level, confirmed_by, confirmed_at, status, version, created_at`

func scanDecision(row rowScanner) (domain.Decision, error) {
	var (
		d                                 domain.Decision
		kind, risk, status                string
		opts                              []byte
		rec, chosen, chooser, confirmedBy sql.NullString
		chosenAt, confirmedAt             sql.NullTime
	)
	if err := row.Scan(&d.ID, &d.TenantID, &d.RequestID, &d.Seq, &kind, &d.SubjectID, &d.SubjectDigest, &d.Question, &opts, &rec,
		&d.RecommendationReason, &chosen, &chooser, &chosenAt, &d.Rationale, &risk, &confirmedBy, &confirmedAt, &status, &d.Version, &d.CreatedAt); err != nil {
		return domain.Decision{}, err
	}
	if err := json.Unmarshal(opts, &d.Options); err != nil {
		return domain.Decision{}, err
	}
	d.SubjectKind, d.RiskLevel, d.Status = domain.DecisionSubjectKind(kind), domain.RiskLevel(risk), domain.DecisionStatus(status)
	d.RecommendedOptionID, d.ChosenOptionID, d.ChooserID, d.ConfirmedBy = rec.String, chosen.String, chooser.String, confirmedBy.String
	d.ChosenAt, d.ConfirmedAt, d.CreatedAt = nullTimePtr(chosenAt), nullTimePtr(confirmedAt), d.CreatedAt.UTC()
	return d, nil
}

func riskOrNormal(r domain.RiskLevel) domain.RiskLevel {
	if r == "" {
		return domain.RiskNormal
	}
	return r
}

func (r *DecisionRepository) Insert(ctx context.Context, d domain.Decision) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		opts, _ := json.Marshal(d.Options)
		_, err := db.ExecContext(ctx, `
			INSERT INTO decisions (id, tenant_id, request_id, seq, subject_kind, subject_id, subject_digest, question, options,
				recommended_option_id, recommendation_reason, chosen_option_id, chooser_id, chosen_at, rationale, risk_level,
				confirmed_by, confirmed_at, status, version, created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			d.ID, tenantID, d.RequestID, d.Seq, string(d.SubjectKind), d.SubjectID, d.SubjectDigest, d.Question, string(opts),
			nullIfEmpty(d.RecommendedOptionID), d.RecommendationReason, nullIfEmpty(d.ChosenOptionID), nullIfEmpty(d.ChooserID), d.ChosenAt,
			d.Rationale, string(riskOrNormal(d.RiskLevel)), nullIfEmpty(d.ConfirmedBy), d.ConfirmedAt, string(d.Status), max(d.Version, 1), d.CreatedAt)
		switch {
		case isDuplicateKey(err, "decisions_one_live"):
			return domain.ErrDecisionLiveExists(string(d.SubjectKind) + ":" + d.SubjectID)
		case isDuplicateKey(err, "decisions_request_seq"):
			return domain.ErrRequestVersionConflict(d.RequestID, int64(d.Seq))
		case err != nil:
			return fmt.Errorf("mysql: insert decision: %w", err)
		}
		return nil
	})
}

func (r *DecisionRepository) Get(ctx context.Context, id string) (domain.Decision, error) {
	var out domain.Decision
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		d, err := scanDecision(db.QueryRowContext(ctx, `SELECT `+decisionColumns+` FROM decisions WHERE tenant_id = ? AND id = ?`, tenantID, id))
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrDecisionNotFound(id)
		}
		if err != nil {
			return fmt.Errorf("mysql: get decision: %w", err)
		}
		out = d
		return nil
	})
	return out, err
}

func (r *DecisionRepository) GetLiveBySubject(ctx context.Context, kind domain.DecisionSubjectKind, subjectID string) (*domain.Decision, error) {
	var out *domain.Decision
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		d, err := scanDecision(db.QueryRowContext(ctx, `SELECT `+decisionColumns+` FROM decisions
			WHERE tenant_id = ? AND subject_kind = ? AND subject_id = ? AND status IN ('open','chosen','effective')`, tenantID, string(kind), subjectID))
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("mysql: get live decision: %w", err)
		}
		out = &d
		return nil
	})
	return out, err
}

func (r *DecisionRepository) Update(ctx context.Context, d domain.Decision, expectedVersion int64) (domain.Decision, error) {
	var out domain.Decision
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `
			UPDATE decisions SET subject_digest = ?, chosen_option_id = ?, chooser_id = ?, chosen_at = ?, rationale = ?, risk_level = ?,
				confirmed_by = ?, confirmed_at = ?, status = ?, version = version + 1
			WHERE tenant_id = ? AND id = ? AND version = ?`,
			d.SubjectDigest, nullIfEmpty(d.ChosenOptionID), nullIfEmpty(d.ChooserID), d.ChosenAt, d.Rationale, string(riskOrNormal(d.RiskLevel)),
			nullIfEmpty(d.ConfirmedBy), d.ConfirmedAt, string(d.Status), tenantID, d.ID, expectedVersion)
		if err != nil {
			return fmt.Errorf("mysql: update decision: %w", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			var one int
			if perr := db.QueryRowContext(ctx, `SELECT 1 FROM decisions WHERE tenant_id = ? AND id = ?`, tenantID, d.ID).Scan(&one); errors.Is(perr, sql.ErrNoRows) {
				return domain.ErrDecisionNotFound(d.ID)
			}
			return domain.ErrDecisionVersionConflict(d.ID, expectedVersion)
		}
		got, err := scanDecision(db.QueryRowContext(ctx, `SELECT `+decisionColumns+` FROM decisions WHERE tenant_id = ? AND id = ?`, tenantID, d.ID))
		if err != nil {
			return fmt.Errorf("mysql: reread decision: %w", err)
		}
		out = got
		return nil
	})
	return out, err
}

func (r *DecisionRepository) AppendHistory(ctx context.Context, h domain.DecisionHistory) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		id := h.ID
		if id == "" {
			id = uuid.NewString()
		}
		at := h.At
		if at.IsZero() {
			at = time.Now().UTC()
		}
		_, err := db.ExecContext(ctx, `INSERT INTO decision_history (id, tenant_id, decision_id, action, option_id, actor_id, rationale, at)
			VALUES (?,?,?,?,?,?,?,?)`, id, tenantID, h.DecisionID, string(h.Action), nullIfEmpty(h.OptionID), nullIfEmpty(h.ActorID), h.Rationale, at)
		if err != nil {
			return fmt.Errorf("mysql: append decision history: %w", err)
		}
		return nil
	})
}

func (r *DecisionRepository) ListHistory(ctx context.Context, decisionID string) ([]domain.DecisionHistory, error) {
	var out []domain.DecisionHistory
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.QueryContext(ctx, `SELECT id, tenant_id, decision_id, action, option_id, actor_id, rationale, at FROM decision_history
			WHERE tenant_id = ? AND decision_id = ? ORDER BY at, id`, tenantID, decisionID)
		if err != nil {
			return fmt.Errorf("mysql: list decision history: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var h domain.DecisionHistory
			var action string
			var opt, actor sql.NullString
			if err := rows.Scan(&h.ID, &h.TenantID, &h.DecisionID, &action, &opt, &actor, &h.Rationale, &h.At); err != nil {
				return err
			}
			h.Action, h.OptionID, h.ActorID, h.At = domain.DecisionAction(action), opt.String, actor.String, h.At.UTC()
			out = append(out, h)
		}
		return rows.Err()
	})
	return out, err
}

func (r *DecisionRepository) List(ctx context.Context, f usecase.DecisionListFilter) ([]domain.Decision, error) {
	var out []domain.Decision
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		limit := f.Limit
		if limit <= 0 {
			limit = 50
		}
		rows, err := db.QueryContext(ctx, `SELECT `+decisionColumns+` FROM decisions
			WHERE tenant_id = ? AND request_id = ? AND seq > ? AND (? = '' OR status = ?) ORDER BY seq LIMIT ?`,
			tenantID, f.RequestID, f.AfterSeq, string(f.Status), string(f.Status), limit)
		if err != nil {
			return fmt.Errorf("mysql: list decisions: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			d, err := scanDecision(rows)
			if err != nil {
				return err
			}
			out = append(out, d)
		}
		return rows.Err()
	})
	return out, err
}

func (r *DecisionRepository) NextSeq(ctx context.Context, requestID string) (int, error) {
	var n int
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if err := lockRequest(ctx, db, tenantID, requestID); err != nil {
			return err
		}
		return db.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) + 1 FROM decisions WHERE tenant_id = ? AND request_id = ?`, tenantID, requestID).Scan(&n)
	})
	return n, err
}

func (r *DecisionRepository) SupersedeLiveBySubject(ctx context.Context, kind domain.DecisionSubjectKind, subjectID string) ([]domain.Decision, error) {
	return r.supersede(ctx, `subject_kind = ? AND subject_id = ?`, string(kind), subjectID)
}

func (r *DecisionRepository) SupersedeLiveByRequest(ctx context.Context, requestID string) ([]domain.Decision, error) {
	return r.supersede(ctx, `request_id = ?`, requestID)
}

// supersede has no RETURNING in MySQL: lock the live rows, update them, hand back the pre-update rows with the new status.
func (r *DecisionRepository) supersede(ctx context.Context, where string, args ...any) ([]domain.Decision, error) {
	var out []domain.Decision
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.QueryContext(ctx, `SELECT `+decisionColumns+` FROM decisions
			WHERE tenant_id = ? AND `+where+` AND status IN ('open','chosen','effective') FOR UPDATE`, append([]any{tenantID}, args...)...)
		if err != nil {
			return fmt.Errorf("mysql: lock live decisions: %w", err)
		}
		for rows.Next() {
			d, err := scanDecision(rows)
			if err != nil {
				rows.Close()
				return err
			}
			out = append(out, d)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for i := range out {
			if _, err := db.ExecContext(ctx, `UPDATE decisions SET status = 'superseded', version = version + 1 WHERE tenant_id = ? AND id = ?`, tenantID, out[i].ID); err != nil {
				return fmt.Errorf("mysql: supersede decision: %w", err)
			}
			out[i].Status, out[i].Version = domain.DecisionStatusSuperseded, out[i].Version+1
		}
		return nil
	})
	return out, err
}
