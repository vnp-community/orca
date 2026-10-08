package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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

func scanClarification(row rowScanner) (domain.Clarification, error) {
	var (
		c                      domain.Clarification
		source, status, resume string
		answeredRev            sql.NullInt64
		reminded, answeredAt   sql.NullTime
	)
	if err := row.Scan(&c.ID, &c.TenantID, &c.RequestID, &c.Seq, &source, &c.SourceRef, &status, &resume, &c.Round, &c.AskedRequestRevision,
		&answeredRev, &c.DueAt, &reminded, &c.CancelReason, &c.CreatedBy, &c.CreatedAt, &answeredAt, &c.Version); err != nil {
		return domain.Clarification{}, err
	}
	c.Source, c.Status, c.ResumeStatus = domain.ClarificationSource(source), domain.ClarificationStatus(status), domain.RequestStatus(resume)
	c.DueAt, c.CreatedAt = c.DueAt.UTC(), c.CreatedAt.UTC()
	if answeredRev.Valid {
		v := int(answeredRev.Int64)
		c.AnsweredRequestRevision = &v
	}
	c.RemindedAt, c.AnsweredAt = nullTimePtr(reminded), nullTimePtr(answeredAt)
	return c, nil
}

func nullTimePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	u := t.Time.UTC()
	return &u
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

func (r *ClarificationRepository) Insert(ctx context.Context, c domain.Clarification) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		_, err := db.ExecContext(ctx, `
			INSERT INTO clarifications (id, tenant_id, request_id, seq, source, source_ref, status, resume_status, round,
				asked_request_revision, due_at, cancel_reason, created_by, created_at, version)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			c.ID, tenantID, c.RequestID, c.Seq, string(c.Source), c.SourceRef, string(c.Status), string(c.ResumeStatus), c.Round,
			c.AskedRequestRevision, c.DueAt, c.CancelReason, c.CreatedBy, c.CreatedAt, max(c.Version, 1))
		switch {
		case isDuplicateKey(err, "clarifications_one_open"):
			return domain.ErrClarificationStateNotAllowed("this request already has an open clarification")
		case isDuplicateKey(err, "clarifications_request_seq"):
			return domain.ErrRequestVersionConflict(c.RequestID, int64(c.Seq))
		case err != nil:
			return fmt.Errorf("mysql: insert clarification: %w", err)
		}
		for _, q := range c.Questions {
			qid := q.ID
			if qid == "" {
				qid = uuid.NewString()
			}
			if _, err := db.ExecContext(ctx, `
				INSERT INTO clarification_questions (id, tenant_id, clarification_id, seq, question_key, kind, prompt, reason,
					options, suggested_default, required, target_path)
				VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
				qid, tenantID, c.ID, q.Seq, q.QuestionKey, string(q.Kind), q.Prompt, q.Reason, optionsArg(q.Options), rawArg(q.SuggestedDefault),
				q.Required, nullIfEmpty(q.TargetPath)); err != nil {
				return fmt.Errorf("mysql: insert clarification question: %w", err)
			}
		}
		for _, a := range c.Assignees {
			if _, err := db.ExecContext(ctx, `INSERT INTO clarification_assignees (clarification_id, tenant_id, principal_kind, principal_id)
				VALUES (?,?,?,?) ON DUPLICATE KEY UPDATE tenant_id = tenant_id`, c.ID, tenantID, string(a.Kind), a.ID); err != nil {
				return fmt.Errorf("mysql: insert clarification assignee: %w", err)
			}
		}
		return nil
	})
}

func (r *ClarificationRepository) load(ctx context.Context, db dbExecer, tenantID string, c *domain.Clarification) error {
	rows, err := db.QueryContext(ctx, `
		SELECT id, seq, question_key, kind, prompt, reason, options, suggested_default, required, target_path, answer, answer_source, answered_by, answered_at
		FROM clarification_questions WHERE tenant_id = ? AND clarification_id = ? ORDER BY seq`, tenantID, c.ID)
	if err != nil {
		return fmt.Errorf("mysql: load clarification questions: %w", err)
	}
	for rows.Next() {
		var (
			q               domain.ClarificationQuestion
			kind            string
			opts, def, ans  []byte
			target, src, by sql.NullString
			at              sql.NullTime
		)
		if err := rows.Scan(&q.ID, &q.Seq, &q.QuestionKey, &kind, &q.Prompt, &q.Reason, &opts, &def, &q.Required, &target, &ans, &src, &by, &at); err != nil {
			rows.Close()
			return err
		}
		q.Kind, q.TargetPath, q.AnswerSource, q.AnsweredBy, q.AnsweredAt = domain.QuestionKind(kind), target.String, src.String, by.String, nullTimePtr(at)
		if len(opts) > 0 {
			if err := json.Unmarshal(opts, &q.Options); err != nil {
				rows.Close()
				return err
			}
		}
		q.SuggestedDefault, q.Answer = canonicalOrNil(def), canonicalOrNil(ans)
		c.Questions = append(c.Questions, q)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	arows, err := db.QueryContext(ctx, `SELECT principal_kind, principal_id FROM clarification_assignees
		WHERE tenant_id = ? AND clarification_id = ? ORDER BY principal_kind, principal_id`, tenantID, c.ID)
	if err != nil {
		return fmt.Errorf("mysql: load clarification assignees: %w", err)
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

// canonicalOrNil returns JSON in the one canonical form so MySQL's own spacing never leaks into comparisons.
func canonicalOrNil(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	if c, err := domain.CanonicalJSON(raw); err == nil {
		return c
	}
	return raw
}

func (r *ClarificationRepository) Get(ctx context.Context, id string) (domain.Clarification, error) {
	var out domain.Clarification
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		c, err := scanClarification(db.QueryRowContext(ctx, `SELECT `+clarificationColumns+` FROM clarifications WHERE tenant_id = ? AND id = ?`, tenantID, id))
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrClarificationNotFound(id)
		}
		if err != nil {
			return fmt.Errorf("mysql: get clarification: %w", err)
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
		c, err := scanClarification(db.QueryRowContext(ctx, `SELECT `+clarificationColumns+` FROM clarifications
			WHERE tenant_id = ? AND request_id = ? AND status = 'open'`, tenantID, requestID))
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("mysql: get open clarification: %w", err)
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
		limit := f.Limit
		if limit <= 0 {
			limit = 50
		}
		rows, err := db.QueryContext(ctx, `SELECT `+clarificationColumns+` FROM clarifications
			WHERE tenant_id = ? AND request_id = ? AND seq > ? AND (? = '' OR status = ?) ORDER BY seq LIMIT ?`,
			tenantID, f.RequestID, f.AfterSeq, string(f.Status), string(f.Status), limit)
		if err != nil {
			return fmt.Errorf("mysql: list clarifications: %w", err)
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
		return db.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) + 1 FROM clarifications WHERE tenant_id = ? AND request_id = ?`, tenantID, requestID).Scan(&n)
	})
	return n, err
}

func (r *ClarificationRepository) MaxRound(ctx context.Context, requestID string, source domain.ClarificationSource, sourceRef string) (int, error) {
	var n int
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		return db.QueryRowContext(ctx, `SELECT COALESCE(MAX(round), 0) FROM clarifications
			WHERE tenant_id = ? AND request_id = ? AND source = ? AND source_ref = ?`, tenantID, requestID, string(source), sourceRef).Scan(&n)
	})
	return n, err
}

func (r *ClarificationRepository) versionProbe(ctx context.Context, db dbExecer, tenantID, id string, expected int64) error {
	var status string
	err := db.QueryRowContext(ctx, `SELECT status FROM clarifications WHERE tenant_id = ? AND id = ?`, tenantID, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
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
		res, err := db.ExecContext(ctx, `UPDATE clarifications SET version = version + 1
			WHERE tenant_id = ? AND id = ? AND status = 'open' AND (? = 0 OR version = ?)`, tenantID, id, expectedVersion, expectedVersion)
		if err != nil {
			return fmt.Errorf("mysql: touch clarification: %w", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return r.versionProbe(ctx, db, tenantID, id, expectedVersion)
		}
		for _, a := range answers {
			if _, err := db.ExecContext(ctx, `UPDATE clarification_questions SET answer = ?, answer_source = ?, answered_by = ?, answered_at = ?
				WHERE tenant_id = ? AND clarification_id = ? AND id = ?`, string(a.Value), a.Source, nullIfEmpty(a.By), a.At, tenantID, id, a.QuestionID); err != nil {
				return fmt.Errorf("mysql: store answer: %w", err)
			}
		}
		return nil
	})
}

func (r *ClarificationRepository) MarkAnswered(ctx context.Context, id string, answeredRevision int, at time.Time, expectedVersion int64) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `UPDATE clarifications SET status = 'answered', answered_request_revision = ?, answered_at = ?, version = version + 1
			WHERE tenant_id = ? AND id = ? AND status = 'open' AND (? = 0 OR version = ?)`, answeredRevision, at, tenantID, id, expectedVersion, expectedVersion)
		if err != nil {
			return fmt.Errorf("mysql: mark clarification answered: %w", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return r.versionProbe(ctx, db, tenantID, id, expectedVersion)
		}
		return nil
	})
}

func (r *ClarificationRepository) MarkCancelled(ctx context.Context, id, reason string, at time.Time, expectedVersion int64) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `UPDATE clarifications SET status = 'cancelled', cancel_reason = ?, version = version + 1
			WHERE tenant_id = ? AND id = ? AND status = 'open' AND (? = 0 OR version = ?)`, reason, tenantID, id, expectedVersion, expectedVersion)
		if err != nil {
			return fmt.Errorf("mysql: mark clarification cancelled: %w", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return r.versionProbe(ctx, db, tenantID, id, expectedVersion)
		}
		return nil
	})
}

func (r *ClarificationRepository) ListDueRefs(ctx context.Context, now time.Time, batch int) ([]usecase.ClarificationRef, error) {
	return r.listRefs(ctx, `SELECT tenant_id, id FROM clarifications WHERE status = 'open' AND due_at <= ? ORDER BY due_at LIMIT ?`, now, batch)
}

// ListRemindableRefs lists open clarifications past half of their window that were never reminded.
func (r *ClarificationRepository) ListRemindableRefs(ctx context.Context, now time.Time, batch int) ([]usecase.ClarificationRef, error) {
	return r.listRefs(ctx, `SELECT tenant_id, id FROM clarifications
		WHERE status = 'open' AND reminded_at IS NULL AND due_at > ? AND ? >= TIMESTAMPADD(MICROSECOND, TIMESTAMPDIFF(MICROSECOND, created_at, due_at) DIV 2, created_at)
		ORDER BY due_at LIMIT ?`, now, now, batch)
}

// listRefs reads across tenants (MySQL has no RLS) and writes nothing; every change then runs in the row's own tenant.
func (r *ClarificationRepository) listRefs(ctx context.Context, query string, args ...any) ([]usecase.ClarificationRef, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql: scan clarifications: %w", err)
	}
	defer rows.Close()
	var out []usecase.ClarificationRef
	for rows.Next() {
		var ref usecase.ClarificationRef
		if err := rows.Scan(&ref.TenantID, &ref.ID); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

func (r *ClarificationRepository) LockOpenDue(ctx context.Context, id string, now time.Time) (*domain.Clarification, error) {
	var out *domain.Clarification
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		c, err := scanClarification(db.QueryRowContext(ctx, `SELECT `+clarificationColumns+` FROM clarifications
			WHERE tenant_id = ? AND id = ? AND status = 'open' AND due_at <= ? FOR UPDATE SKIP LOCKED`, tenantID, id, now))
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("mysql: lock overdue clarification: %w", err)
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
		res, err := db.ExecContext(ctx, `UPDATE clarifications SET status = 'expired', version = version + 1
			WHERE tenant_id = ? AND id = ? AND status = 'open'`, tenantID, id)
		if err != nil {
			return fmt.Errorf("mysql: mark clarification expired: %w", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return r.versionProbe(ctx, db, tenantID, id, 0)
		}
		return nil
	})
}

func (r *ClarificationRepository) MarkReminded(ctx context.Context, id string, at time.Time) (bool, error) {
	claimed := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `UPDATE clarifications SET reminded_at = ?
			WHERE tenant_id = ? AND id = ? AND status = 'open' AND reminded_at IS NULL`, at, tenantID, id)
		if err != nil {
			return fmt.Errorf("mysql: mark clarification reminded: %w", err)
		}
		n, _ := res.RowsAffected()
		claimed = n == 1
		return nil
	})
	return claimed, err
}

const maxPendingTeams = 100

// ListPendingForUser mirrors the Postgres query without JSON operators or array parameters.
func (r *ClarificationRepository) ListPendingForUser(ctx context.Context, f usecase.PendingFilter) ([]domain.Clarification, string, error) {
	if len(f.Teams) > maxPendingTeams || len(f.Roles) > maxPendingTeams {
		return nil, "", domain.ErrClarificationStateNotAllowed(fmt.Sprintf("at most %d teams or roles per query", maxPendingTeams))
	}
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
	var out []domain.Clarification
	next := ""
	err = r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		args := []any{tenantID}
		var access []string
		if f.IsAdmin {
			access = append(access, "1 = 1")
		} else {
			clause := "(a.principal_kind = 'user' AND a.principal_id = ?)"
			args = append(args, f.UserID)
			for _, group := range []struct {
				kind string
				ids  []string
			}{{"team", f.Teams}, {"role", f.Roles}} {
				if len(group.ids) == 0 {
					continue
				}
				marks := strings.TrimSuffix(strings.Repeat("?,", len(group.ids)), ",")
				clause += fmt.Sprintf(" OR (a.principal_kind = '%s' AND a.principal_id IN (%s))", group.kind, marks)
				for _, id := range group.ids {
					args = append(args, id)
				}
			}
			access = append(access, "EXISTS (SELECT 1 FROM clarification_assignees a WHERE a.clarification_id = c.id AND a.tenant_id = c.tenant_id AND ("+clause+"))")
			access = append(access, `(EXISTS (SELECT 1 FROM clarification_assignees a2 WHERE a2.clarification_id = c.id AND a2.tenant_id = c.tenant_id AND a2.principal_kind = 'reporter')
				AND EXISTS (SELECT 1 FROM requests r WHERE r.id = c.request_id AND r.tenant_id = c.tenant_id AND r.reporter_id = ?))`)
			args = append(args, f.UserID)
		}
		q := `SELECT c.id, c.tenant_id, c.request_id, c.seq, c.source, c.source_ref, c.status, c.resume_status, c.round, c.asked_request_revision,
				c.answered_request_revision, c.due_at, c.reminded_at, c.cancel_reason, c.created_by, c.created_at, c.answered_at, c.version
			FROM clarifications c WHERE c.tenant_id = ? AND c.status = 'open' AND (` + strings.Join(access, " OR ") + `)`
		// The tenant placeholder is the first argument; the cursor goes after the access arguments.
		if !cursorAt.IsZero() {
			q += ` AND (c.created_at, c.id) < (?, ?)`
			args = append(args, cursorAt, cursorID)
		}
		q += ` ORDER BY c.created_at DESC, c.id DESC LIMIT ?`
		args = append(args, size+1)
		rows, err := db.QueryContext(ctx, q, args...)
		if err != nil {
			return fmt.Errorf("mysql: list pending clarifications: %w", err)
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
