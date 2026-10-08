package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

func scanDecision(row pgx.Row) (domain.Decision, error) {
	var (
		d                                 domain.Decision
		kind, risk, status                string
		opts                              []byte
		rec, chosen, chooser, confirmedBy *string
		chosenAt, confirmedAt             *time.Time
		created                           time.Time
	)
	if err := row.Scan(&d.ID, &d.TenantID, &d.RequestID, &d.Seq, &kind, &d.SubjectID, &d.SubjectDigest, &d.Question, &opts, &rec,
		&d.RecommendationReason, &chosen, &chooser, &chosenAt, &d.Rationale, &risk, &confirmedBy, &confirmedAt, &status, &d.Version, &created); err != nil {
		return domain.Decision{}, err
	}
	if err := json.Unmarshal(opts, &d.Options); err != nil {
		return domain.Decision{}, err
	}
	d.SubjectKind, d.RiskLevel, d.Status = domain.DecisionSubjectKind(kind), domain.RiskLevel(risk), domain.DecisionStatus(status)
	d.RecommendedOptionID, d.ChosenOptionID, d.ChooserID, d.ConfirmedBy = derefString(rec), derefString(chosen), derefString(chooser), derefString(confirmedBy)
	d.ChosenAt, d.ConfirmedAt, d.CreatedAt = utcPtr(chosenAt), utcPtr(confirmedAt), created.UTC()
	return d, nil
}

func (r *DecisionRepository) Insert(ctx context.Context, d domain.Decision) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		opts, _ := json.Marshal(d.Options)
		_, err := db.Exec(ctx, `
			INSERT INTO request.decisions (id, tenant_id, request_id, seq, subject_kind, subject_id, subject_digest, question, options,
				recommended_option_id, recommendation_reason, chosen_option_id, chooser_id, chosen_at, rationale, risk_level,
				confirmed_by, confirmed_at, status, version, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
			d.ID, tenantID, d.RequestID, d.Seq, string(d.SubjectKind), d.SubjectID, d.SubjectDigest, d.Question, string(opts),
			nullIfEmpty(d.RecommendedOptionID), d.RecommendationReason, nullIfEmpty(d.ChosenOptionID), nullIfEmpty(d.ChooserID), d.ChosenAt,
			d.Rationale, string(riskOrNormal(d.RiskLevel)), nullIfEmpty(d.ConfirmedBy), d.ConfirmedAt, string(d.Status), max(d.Version, 1), d.CreatedAt)
		switch {
		case isUniqueViolation(err, "decisions_one_live"):
			return domain.ErrDecisionLiveExists(string(d.SubjectKind) + ":" + d.SubjectID)
		case isUniqueViolation(err, "decisions_request_seq"):
			return domain.ErrRequestVersionConflict(d.RequestID, int64(d.Seq))
		case err != nil:
			return fmt.Errorf("postgres: insert decision: %w", err)
		}
		return nil
	})
}

func riskOrNormal(r domain.RiskLevel) domain.RiskLevel {
	if r == "" {
		return domain.RiskNormal
	}
	return r
}

func (r *DecisionRepository) Get(ctx context.Context, id string) (domain.Decision, error) {
	var out domain.Decision
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, perr := uuid.Parse(id); perr != nil {
			return domain.ErrDecisionNotFound(id)
		}
		d, err := scanDecision(db.QueryRow(ctx, `SELECT `+decisionColumns+` FROM request.decisions WHERE tenant_id = $1 AND id = $2`, tenantID, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrDecisionNotFound(id)
		}
		if err != nil {
			return fmt.Errorf("postgres: get decision: %w", err)
		}
		out = d
		return nil
	})
	return out, err
}

func (r *DecisionRepository) GetLiveBySubject(ctx context.Context, kind domain.DecisionSubjectKind, subjectID string) (*domain.Decision, error) {
	var out *domain.Decision
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		d, err := scanDecision(db.QueryRow(ctx, `SELECT `+decisionColumns+` FROM request.decisions
			WHERE tenant_id = $1 AND subject_kind = $2 AND subject_id = $3 AND status IN ('open','chosen','effective')`, tenantID, string(kind), subjectID))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("postgres: get live decision: %w", err)
		}
		out = &d
		return nil
	})
	return out, err
}

func (r *DecisionRepository) Update(ctx context.Context, d domain.Decision, expectedVersion int64) (domain.Decision, error) {
	var out domain.Decision
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		got, err := scanDecision(db.QueryRow(ctx, `
			UPDATE request.decisions SET subject_digest = $4, chosen_option_id = $5, chooser_id = $6, chosen_at = $7, rationale = $8, risk_level = $9,
				confirmed_by = $10, confirmed_at = $11, status = $12, version = version + 1
			WHERE tenant_id = $1 AND id = $2 AND version = $3
			RETURNING `+decisionColumns,
			tenantID, d.ID, expectedVersion, d.SubjectDigest, nullIfEmpty(d.ChosenOptionID), nullIfEmpty(d.ChooserID), d.ChosenAt, d.Rationale,
			string(riskOrNormal(d.RiskLevel)), nullIfEmpty(d.ConfirmedBy), d.ConfirmedAt, string(d.Status)))
		if err == nil {
			out = got
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("postgres: update decision: %w", err)
		}
		var one int
		if perr := db.QueryRow(ctx, `SELECT 1 FROM request.decisions WHERE tenant_id = $1 AND id = $2`, tenantID, d.ID).Scan(&one); errors.Is(perr, pgx.ErrNoRows) {
			return domain.ErrDecisionNotFound(d.ID)
		}
		return domain.ErrDecisionVersionConflict(d.ID, expectedVersion)
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
		_, err := db.Exec(ctx, `INSERT INTO request.decision_history (id, tenant_id, decision_id, action, option_id, actor_id, rationale, at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, id, tenantID, h.DecisionID, string(h.Action), nullIfEmpty(h.OptionID), nullIfEmpty(h.ActorID), h.Rationale, at)
		if err != nil {
			return fmt.Errorf("postgres: append decision history: %w", err)
		}
		return nil
	})
}

func (r *DecisionRepository) ListHistory(ctx context.Context, decisionID string) ([]domain.DecisionHistory, error) {
	var out []domain.DecisionHistory
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, perr := uuid.Parse(decisionID); perr != nil {
			return nil
		}
		rows, err := db.Query(ctx, `SELECT id, tenant_id, decision_id, action, option_id, actor_id, rationale, at FROM request.decision_history
			WHERE tenant_id = $1 AND decision_id = $2 ORDER BY at, id`, tenantID, decisionID)
		if err != nil {
			return fmt.Errorf("postgres: list decision history: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var h domain.DecisionHistory
			var action string
			var opt, actor *string
			if err := rows.Scan(&h.ID, &h.TenantID, &h.DecisionID, &action, &opt, &actor, &h.Rationale, &h.At); err != nil {
				return err
			}
			h.Action, h.OptionID, h.ActorID, h.At = domain.DecisionAction(action), derefString(opt), derefString(actor), h.At.UTC()
			out = append(out, h)
		}
		return rows.Err()
	})
	return out, err
}

func (r *DecisionRepository) List(ctx context.Context, f usecase.DecisionListFilter) ([]domain.Decision, error) {
	var out []domain.Decision
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, perr := uuid.Parse(f.RequestID); perr != nil {
			return nil
		}
		limit := f.Limit
		if limit <= 0 {
			limit = 50
		}
		rows, err := db.Query(ctx, `SELECT `+decisionColumns+` FROM request.decisions
			WHERE tenant_id = $1 AND request_id = $2 AND seq > $3 AND ($4 = '' OR status = $4) ORDER BY seq LIMIT $5`,
			tenantID, f.RequestID, f.AfterSeq, string(f.Status), limit)
		if err != nil {
			return fmt.Errorf("postgres: list decisions: %w", err)
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
		return db.QueryRow(ctx, `SELECT COALESCE(MAX(seq), 0) + 1 FROM request.decisions WHERE tenant_id = $1 AND request_id = $2`, tenantID, requestID).Scan(&n)
	})
	return n, err
}

func (r *DecisionRepository) SupersedeLiveBySubject(ctx context.Context, kind domain.DecisionSubjectKind, subjectID string) ([]domain.Decision, error) {
	return r.supersede(ctx, `subject_kind = $2 AND subject_id = $3`, string(kind), subjectID)
}

func (r *DecisionRepository) SupersedeLiveByRequest(ctx context.Context, requestID string) ([]domain.Decision, error) {
	if _, err := uuid.Parse(requestID); err != nil {
		return nil, nil
	}
	return r.supersede(ctx, `request_id = $2`, requestID)
}

// supersede closes every live decision matching where ($1 is the tenant, filter arguments start at $2).
func (r *DecisionRepository) supersede(ctx context.Context, where string, args ...any) ([]domain.Decision, error) {
	var out []domain.Decision
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.Query(ctx, `UPDATE request.decisions SET status = 'superseded', version = version + 1
			WHERE tenant_id = $1 AND `+where+` AND status IN ('open','chosen','effective') RETURNING `+decisionColumns, append([]any{tenantID}, args...)...)
		if err != nil {
			return fmt.Errorf("postgres: supersede decisions: %w", err)
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
