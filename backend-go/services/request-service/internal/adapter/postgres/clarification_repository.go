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

type ClarificationRepository struct {
	*Repository
}

func NewClarificationRepository(r *Repository) *ClarificationRepository {
	return &ClarificationRepository{Repository: r}
}

var _ usecase.ClarificationRepository = (*ClarificationRepository)(nil)

const clarificationColumns = `id, tenant_id, request_id, seq, source, source_ref, status, resume_status, round, asked_request_revision,
	answered_request_revision, due_at, reminded_at, cancel_reason, created_by, created_at, answered_at, version`

func scanClarification(row pgx.Row) (domain.Clarification, error) {
	var (
		c                      domain.Clarification
		source, status, resume string
		due, created           time.Time
		reminded, answeredAt   *time.Time
	)
	if err := row.Scan(&c.ID, &c.TenantID, &c.RequestID, &c.Seq, &source, &c.SourceRef, &status, &resume, &c.Round, &c.AskedRequestRevision,
		&c.AnsweredRequestRevision, &due, &reminded, &c.CancelReason, &c.CreatedBy, &created, &answeredAt, &c.Version); err != nil {
		return domain.Clarification{}, err
	}
	c.Source, c.Status, c.ResumeStatus = domain.ClarificationSource(source), domain.ClarificationStatus(status), domain.RequestStatus(resume)
	c.DueAt, c.CreatedAt = due.UTC(), created.UTC()
	c.RemindedAt, c.AnsweredAt = utcPtr(reminded), utcPtr(answeredAt)
	return c, nil
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func (r *ClarificationRepository) Insert(ctx context.Context, c domain.Clarification) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		_, err := db.Exec(ctx, `
			INSERT INTO request.clarifications (id, tenant_id, request_id, seq, source, source_ref, status, resume_status, round,
				asked_request_revision, due_at, cancel_reason, created_by, created_at, version)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			c.ID, tenantID, c.RequestID, c.Seq, string(c.Source), c.SourceRef, string(c.Status), string(c.ResumeStatus), c.Round,
			c.AskedRequestRevision, c.DueAt, c.CancelReason, c.CreatedBy, c.CreatedAt, max(c.Version, 1))
		switch {
		case isUniqueViolation(err, "clarifications_one_open"):
			return domain.ErrClarificationStateNotAllowed("this request already has an open clarification")
		case isUniqueViolation(err, "clarifications_request_seq"):
			return domain.ErrRequestVersionConflict(c.RequestID, int64(c.Seq))
		case err != nil:
			return fmt.Errorf("postgres: insert clarification: %w", err)
		}
		for _, q := range c.Questions {
			qid := q.ID
			if qid == "" {
				qid = uuid.NewString()
			}
			if _, err := db.Exec(ctx, `
				INSERT INTO request.clarification_questions (id, tenant_id, clarification_id, seq, question_key, kind, prompt, reason,
					options, suggested_default, required, target_path)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10::jsonb,$11,$12)`,
				qid, tenantID, c.ID, q.Seq, q.QuestionKey, string(q.Kind), q.Prompt, q.Reason, optionsArg(q.Options), rawArg(q.SuggestedDefault),
				q.Required, nullIfEmpty(q.TargetPath)); err != nil {
				return fmt.Errorf("postgres: insert clarification question: %w", err)
			}
		}
		for _, a := range c.Assignees {
			if _, err := db.Exec(ctx, `INSERT INTO request.clarification_assignees (clarification_id, tenant_id, principal_kind, principal_id)
				VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`, c.ID, tenantID, string(a.Kind), a.ID); err != nil {
				return fmt.Errorf("postgres: insert clarification assignee: %w", err)
			}
		}
		return nil
	})
}

func optionsArg(opts []domain.QuestionOption) any {
	if len(opts) == 0 {
		return nil
	}
	b, _ := json.Marshal(opts)
	return string(b)
}

func rawArg(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}

func (r *ClarificationRepository) load(ctx context.Context, db dbExecer, tenantID string, c *domain.Clarification) error {
	rows, err := db.Query(ctx, `
		SELECT id, seq, question_key, kind, prompt, reason, options, suggested_default, required, target_path, answer, answer_source, answered_by, answered_at
		FROM request.clarification_questions WHERE tenant_id = $1 AND clarification_id = $2 ORDER BY seq`, tenantID, c.ID)
	if err != nil {
		return fmt.Errorf("postgres: load clarification questions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			q               domain.ClarificationQuestion
			kind            string
			opts, def, ans  []byte
			target, src, by *string
			at              *time.Time
		)
		if err := rows.Scan(&q.ID, &q.Seq, &q.QuestionKey, &kind, &q.Prompt, &q.Reason, &opts, &def, &q.Required, &target, &ans, &src, &by, &at); err != nil {
			return err
		}
		q.Kind, q.TargetPath, q.AnswerSource, q.AnsweredBy, q.AnsweredAt = domain.QuestionKind(kind), derefString(target), derefString(src), derefString(by), utcPtr(at)
		if len(opts) > 0 {
			if err := json.Unmarshal(opts, &q.Options); err != nil {
				return err
			}
		}
		q.SuggestedDefault, q.Answer = def, ans
		c.Questions = append(c.Questions, q)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	arows, err := db.Query(ctx, `SELECT principal_kind, principal_id FROM request.clarification_assignees
		WHERE tenant_id = $1 AND clarification_id = $2 ORDER BY principal_kind, principal_id`, tenantID, c.ID)
	if err != nil {
		return fmt.Errorf("postgres: load clarification assignees: %w", err)
	}
	defer arows.Close()
	for arows.Next() {
		var kind string
		var a domain.Principal
		if err := arows.Scan(&kind, &a.ID); err != nil {
			return err
		}
		a.Kind = domain.PrincipalKind(kind)
		c.Assignees = append(c.Assignees, a)
	}
	return arows.Err()
}

func (r *ClarificationRepository) Get(ctx context.Context, id string) (domain.Clarification, error) {
	var out domain.Clarification
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, perr := uuid.Parse(id); perr != nil {
			return domain.ErrClarificationNotFound(id)
		}
		c, err := scanClarification(db.QueryRow(ctx, `SELECT `+clarificationColumns+` FROM request.clarifications WHERE tenant_id = $1 AND id = $2`, tenantID, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrClarificationNotFound(id)
		}
		if err != nil {
			return fmt.Errorf("postgres: get clarification: %w", err)
		}
		if err := r.load(ctx, db, tenantID, &c); err != nil {
			return err
		}
		out = c
		return nil
	})
	return out, err
}

func (r *ClarificationRepository) GetOpenByRequest(ctx context.Context, requestID string) (*domain.Clarification, error) {
	var out *domain.Clarification
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, perr := uuid.Parse(requestID); perr != nil {
			return nil
		}
		c, err := scanClarification(db.QueryRow(ctx, `SELECT `+clarificationColumns+` FROM request.clarifications
			WHERE tenant_id = $1 AND request_id = $2 AND status = 'open'`, tenantID, requestID))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("postgres: get open clarification: %w", err)
		}
		if err := r.load(ctx, db, tenantID, &c); err != nil {
			return err
		}
		out = &c
		return nil
	})
	return out, err
}

func (r *ClarificationRepository) List(ctx context.Context, f usecase.ClarificationListFilter) ([]domain.Clarification, error) {
	var out []domain.Clarification
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, perr := uuid.Parse(f.RequestID); perr != nil {
			return nil
		}
		limit := f.Limit
		if limit <= 0 {
			limit = 50
		}
		rows, err := db.Query(ctx, `SELECT `+clarificationColumns+` FROM request.clarifications
			WHERE tenant_id = $1 AND request_id = $2 AND seq > $3 AND ($4 = '' OR status = $4) ORDER BY seq LIMIT $5`,
			tenantID, f.RequestID, f.AfterSeq, string(f.Status), limit)
		if err != nil {
			return fmt.Errorf("postgres: list clarifications: %w", err)
		}
		for rows.Next() {
			c, err := scanClarification(rows)
			if err != nil {
				rows.Close()
				return err
			}
			out = append(out, c)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for i := range out {
			if err := r.load(ctx, db, tenantID, &out[i]); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

func (r *ClarificationRepository) NextSeq(ctx context.Context, requestID string) (int, error) {
	var n int
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if err := lockRequest(ctx, db, tenantID, requestID); err != nil {
			return err
		}
		return db.QueryRow(ctx, `SELECT COALESCE(MAX(seq), 0) + 1 FROM request.clarifications WHERE tenant_id = $1 AND request_id = $2`, tenantID, requestID).Scan(&n)
	})
	return n, err
}

func (r *ClarificationRepository) MaxRound(ctx context.Context, requestID string, source domain.ClarificationSource, sourceRef string) (int, error) {
	var n int
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, perr := uuid.Parse(requestID); perr != nil {
			return nil
		}
		return db.QueryRow(ctx, `SELECT COALESCE(MAX(round), 0) FROM request.clarifications
			WHERE tenant_id = $1 AND request_id = $2 AND source = $3 AND source_ref = $4`, tenantID, requestID, string(source), sourceRef).Scan(&n)
	})
	return n, err
}

// versionProbe tells why a guarded UPDATE touched no row.
func (r *ClarificationRepository) versionProbe(ctx context.Context, db dbExecer, tenantID, id string, expected int64) error {
	var status string
	var version int64
	err := db.QueryRow(ctx, `SELECT status, version FROM request.clarifications WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(&status, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrClarificationNotFound(id)
	}
	if err != nil {
		return err
	}
	if domain.ClarificationStatus(status) != domain.ClarificationStatusOpen {
		return domain.ErrClarificationNotOpen(id, domain.ClarificationStatus(status))
	}
	return domain.ErrClarificationVersionConflict(id, expected)
}

func (r *ClarificationRepository) UpdateAnswers(ctx context.Context, id string, answers []usecase.AnswerRecord, expectedVersion int64) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `UPDATE request.clarifications SET version = version + 1
			WHERE tenant_id = $1 AND id = $2 AND status = 'open' AND ($3::bigint = 0 OR version = $3)`, tenantID, id, expectedVersion)
		if err != nil {
			return fmt.Errorf("postgres: touch clarification: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return r.versionProbe(ctx, db, tenantID, id, expectedVersion)
		}
		for _, a := range answers {
			if _, err := db.Exec(ctx, `UPDATE request.clarification_questions SET answer = $4::jsonb, answer_source = $5, answered_by = $6, answered_at = $7
				WHERE tenant_id = $1 AND clarification_id = $2 AND id = $3`, tenantID, id, a.QuestionID, string(a.Value), a.Source, nullIfEmpty(a.By), a.At); err != nil {
				return fmt.Errorf("postgres: store answer: %w", err)
			}
		}
		return nil
	})
}

func (r *ClarificationRepository) MarkAnswered(ctx context.Context, id string, answeredRevision int, at time.Time, expectedVersion int64) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `UPDATE request.clarifications SET status = 'answered', answered_request_revision = $3, answered_at = $4, version = version + 1
			WHERE tenant_id = $1 AND id = $2 AND status = 'open' AND ($5::bigint = 0 OR version = $5)`, tenantID, id, answeredRevision, at, expectedVersion)
		if err != nil {
			return fmt.Errorf("postgres: mark clarification answered: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return r.versionProbe(ctx, db, tenantID, id, expectedVersion)
		}
		return nil
	})
}

func (r *ClarificationRepository) MarkCancelled(ctx context.Context, id, reason string, at time.Time, expectedVersion int64) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `UPDATE request.clarifications SET status = 'cancelled', cancel_reason = $3, version = version + 1
			WHERE tenant_id = $1 AND id = $2 AND status = 'open' AND ($4::bigint = 0 OR version = $4)`, tenantID, id, reason, expectedVersion)
		if err != nil {
			return fmt.Errorf("postgres: mark clarification cancelled: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return r.versionProbe(ctx, db, tenantID, id, expectedVersion)
		}
		return nil
	})
}

func (r *ClarificationRepository) ListDueRefs(ctx context.Context, now time.Time, batch int) ([]usecase.ClarificationRef, error) {
	return r.listRefs(ctx, `SELECT tenant_id, id FROM request.clarifications WHERE status = 'open' AND due_at <= $1 ORDER BY due_at LIMIT $2`, now, batch)
}

// ListRemindableRefs lists open clarifications past half of their window that were never reminded.
func (r *ClarificationRepository) ListRemindableRefs(ctx context.Context, now time.Time, batch int) ([]usecase.ClarificationRef, error) {
	return r.listRefs(ctx, `SELECT tenant_id, id FROM request.clarifications
		WHERE status = 'open' AND reminded_at IS NULL AND due_at > $1 AND $1 >= created_at + (due_at - created_at) / 2
		ORDER BY due_at LIMIT $2`, now, batch)
}

// listRefs reads across tenants through the relay policy; it writes nothing, every change then runs in the row's own tenant.
func (r *ClarificationRepository) listRefs(ctx context.Context, query string, now time.Time, batch int) ([]usecase.ClarificationRef, error) {
	var out []usecase.ClarificationRef
	err := r.withRelayTx(ctx, func(ctx context.Context, db dbExecer) error {
		rows, err := db.Query(ctx, query, now, batch)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var ref usecase.ClarificationRef
			if err := rows.Scan(&ref.TenantID, &ref.ID); err != nil {
				return err
			}
			out = append(out, ref)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: scan clarifications: %w", err)
	}
	return out, nil
}

func (r *ClarificationRepository) LockOpenDue(ctx context.Context, id string, now time.Time) (*domain.Clarification, error) {
	var out *domain.Clarification
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		c, err := scanClarification(db.QueryRow(ctx, `SELECT `+clarificationColumns+` FROM request.clarifications
			WHERE tenant_id = $1 AND id = $2 AND status = 'open' AND due_at <= $3 FOR UPDATE SKIP LOCKED`, tenantID, id, now))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("postgres: lock overdue clarification: %w", err)
		}
		if err := r.load(ctx, db, tenantID, &c); err != nil {
			return err
		}
		out = &c
		return nil
	})
	return out, err
}

func (r *ClarificationRepository) MarkExpired(ctx context.Context, id string, at time.Time) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `UPDATE request.clarifications SET status = 'expired', version = version + 1
			WHERE tenant_id = $1 AND id = $2 AND status = 'open'`, tenantID, id)
		if err != nil {
			return fmt.Errorf("postgres: mark clarification expired: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return r.versionProbe(ctx, db, tenantID, id, 0)
		}
		return nil
	})
}

func (r *ClarificationRepository) MarkReminded(ctx context.Context, id string, at time.Time) (bool, error) {
	claimed := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `UPDATE request.clarifications SET reminded_at = $3
			WHERE tenant_id = $1 AND id = $2 AND status = 'open' AND reminded_at IS NULL`, tenantID, id, at)
		if err != nil {
			return fmt.Errorf("postgres: mark clarification reminded: %w", err)
		}
		claimed = tag.RowsAffected() == 1
		return nil
	})
	return claimed, err
}

// ListPendingForUser lists open clarifications the user may answer: named as user, member of a listed team,
// holder of a listed role, the reporter of the request, or any when the caller is an admin. No JSON operators.
func (r *ClarificationRepository) ListPendingForUser(ctx context.Context, f usecase.PendingFilter) ([]domain.Clarification, string, error) {
	var out []domain.Clarification
	next := ""
	size := f.PageSize
	if size <= 0 {
		size = 50
	}
	if size > 200 {
		size = 200
	}
	cursorAt, cursorID, err := usecase.DecodePageToken(f.PageToken)
	if err != nil {
		return nil, "", err
	}
	err = r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		teams, roles := f.Teams, f.Roles
		if teams == nil {
			teams = []string{}
		}
		if roles == nil {
			roles = []string{}
		}
		var cursorArgAt, cursorArgID any
		if !cursorAt.IsZero() {
			cursorArgAt, cursorArgID = cursorAt, cursorID
		}
		rows, err := db.Query(ctx, `
			SELECT c.id, c.tenant_id, c.request_id, c.seq, c.source, c.source_ref, c.status, c.resume_status, c.round, c.asked_request_revision,
				c.answered_request_revision, c.due_at, c.reminded_at, c.cancel_reason, c.created_by, c.created_at, c.answered_at, c.version
			FROM request.clarifications c
			WHERE c.tenant_id = $1 AND c.status = 'open'
			  AND ($7::timestamptz IS NULL OR (c.created_at, c.id) < ($7::timestamptz, $8::uuid))
			  AND ($2::boolean
			    OR EXISTS (SELECT 1 FROM request.clarification_assignees a WHERE a.clarification_id = c.id AND a.tenant_id = c.tenant_id AND (
			         (a.principal_kind = 'user' AND a.principal_id = $3)
			      OR (a.principal_kind = 'team' AND a.principal_id = ANY($4::text[]))
			      OR (a.principal_kind = 'role' AND a.principal_id = ANY($5::text[]))))
			    OR (EXISTS (SELECT 1 FROM request.clarification_assignees a2 WHERE a2.clarification_id = c.id AND a2.tenant_id = c.tenant_id AND a2.principal_kind = 'reporter')
			        AND EXISTS (SELECT 1 FROM request.requests r WHERE r.id = c.request_id AND r.tenant_id = c.tenant_id AND r.reporter_id::text = $3)))
			ORDER BY c.created_at DESC, c.id DESC LIMIT $6`,
			tenantID, f.IsAdmin, f.UserID, teams, roles, size+1, cursorArgAt, cursorArgID)
		if err != nil {
			return fmt.Errorf("postgres: list pending clarifications: %w", err)
		}
		for rows.Next() {
			c, err := scanClarification(rows)
			if err != nil {
				rows.Close()
				return err
			}
			out = append(out, c)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(out) > size {
			out = out[:size]
			last := out[len(out)-1]
			next = usecase.EncodePageToken(last.CreatedAt, last.ID)
		}
		for i := range out {
			if err := r.load(ctx, db, tenantID, &out[i]); err != nil {
				return err
			}
		}
		return nil
	})
	return out, next, err
}
