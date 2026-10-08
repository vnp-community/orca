package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// ApprovalRepository is a separate type so Repository does not grow generic Insert/Get names.
type ApprovalRepository struct {
	*Repository
}

func NewApprovalRepository(r *Repository) *ApprovalRepository {
	return &ApprovalRepository{Repository: r}
}

var _ usecase.ApprovalRepository = (*ApprovalRepository)(nil)

const approvalColumns = `id, tenant_id, request_id, subject_type, subject_id, stage, status, requested_by, decided_by, decided_at, comment, due_at, version, subject_digest, self_approval_allowed, idempotency_key, reminded_at, created_at, updated_at`

func approvalDests(a *domain.Approval) []any {
	return []any{&a.ID, &a.TenantID, &a.RequestID, &a.SubjectType, &a.SubjectID, &a.Stage, &a.Status,
		&a.RequestedBy, &a.DecidedBy, &a.DecidedAt, &a.Comment, &a.DueAt, &a.Version,
		&a.SubjectDigest, &a.SelfApprovalAllowed, &a.IdempotencyKey, &a.RemindedAt, &a.CreatedAt, &a.UpdatedAt}
}

func scanApproval(row pgx.Row) (domain.Approval, error) {
	var a domain.Approval
	if err := row.Scan(approvalDests(&a)...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return a, usecase.ErrApprovalNotFound
		}
		return a, fmt.Errorf("postgres: scan approval: %w", err)
	}
	return a, nil
}

func (r *ApprovalRepository) Insert(ctx context.Context, a domain.Approval) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if a.TenantID != tenantID {
			return domain.ErrRequestTenantRequired()
		}
		_, err := db.Exec(ctx, `INSERT INTO request.approvals (`+approvalColumns+`)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
			a.ID, a.TenantID, a.RequestID, a.SubjectType, a.SubjectID, a.Stage, a.Status,
			a.RequestedBy, a.DecidedBy, a.DecidedAt, a.Comment, a.DueAt, a.Version,
			a.SubjectDigest, a.SelfApprovalAllowed, a.IdempotencyKey, a.RemindedAt, a.CreatedAt, a.UpdatedAt)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				if strings.Contains(pgErr.ConstraintName, "approvals_one_pending") {
					return usecase.ErrPendingExists
				}
				if strings.Contains(pgErr.ConstraintName, "approvals_idem") {
					return usecase.ErrIdempotencyConflict
				}
			}
			return fmt.Errorf("postgres: insert approval: %w", err)
		}
		return nil
	})
}

func (r *ApprovalRepository) getWhere(ctx context.Context, tenantID, cond string, lock bool, args ...any) (domain.Approval, error) {
	var out domain.Approval
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return usecase.ErrApprovalNotFound
		}
		got, err := scanApproval(db.QueryRow(ctx, `SELECT `+approvalColumns+` FROM request.approvals WHERE tenant_id = $1 AND `+cond+suffix,
			append([]any{tenantID}, args...)...))
		out = got
		return err
	})
	return out, err
}

func (r *ApprovalRepository) Get(ctx context.Context, tenantID, id string) (domain.Approval, error) {
	if _, err := uuid.Parse(id); err != nil {
		return domain.Approval{}, usecase.ErrApprovalNotFound
	}
	return r.getWhere(ctx, tenantID, "id = $2", false, id)
}

func (r *ApprovalRepository) GetForUpdate(ctx context.Context, tenantID, id string) (domain.Approval, error) {
	if _, err := uuid.Parse(id); err != nil {
		return domain.Approval{}, usecase.ErrApprovalNotFound
	}
	return r.getWhere(ctx, tenantID, "id = $2", true, id)
}

func (r *ApprovalRepository) FindPendingBySubject(ctx context.Context, tenantID string, st domain.SubjectType, subjectID string) (*domain.Approval, error) {
	a, err := r.getWhere(ctx, tenantID, "subject_type = $2 AND subject_id = $3 AND status = 'pending'", false, string(st), subjectID)
	if errors.Is(err, usecase.ErrApprovalNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *ApprovalRepository) FindByIdempotencyKey(ctx context.Context, tenantID, key string) (*domain.Approval, error) {
	a, err := r.getWhere(ctx, tenantID, "idempotency_key = $2", false, key)
	if errors.Is(err, usecase.ErrApprovalNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *ApprovalRepository) execAffected(ctx context.Context, sql string, args ...any) (bool, error) {
	var affected int64
	err := r.scoped(ctx, func(ctx context.Context, _ string, db dbExecer) error {
		tag, err := db.Exec(ctx, sql, args...)
		affected = tag.RowsAffected()
		return err
	})
	return affected > 0, err
}

// UpdateDecision writes the new state of the passed approval; a is the post-transition value, so a.Version-1 is not assumed.
func (r *ApprovalRepository) UpdateDecision(ctx context.Context, a domain.Approval, expectedVersion int64) (bool, error) {
	ok, err := r.execAffected(ctx, `UPDATE request.approvals
		SET status = $1, decided_by = $2, decided_at = $3, comment = $4, version = version + 1, updated_at = $5
		WHERE id = $6 AND tenant_id = $7 AND status = 'pending' AND version = $8`,
		a.Status, a.DecidedBy, a.DecidedAt, a.Comment, a.UpdatedAt, a.ID, a.TenantID, expectedVersion)
	if err != nil {
		return false, fmt.Errorf("postgres: update approval decision: %w", err)
	}
	return ok, nil
}

func (r *ApprovalRepository) UpdateDue(ctx context.Context, a domain.Approval, expectedVersion int64) (bool, error) {
	ok, err := r.execAffected(ctx, `UPDATE request.approvals
		SET due_at = $1, reminded_at = NULL, version = version + 1, updated_at = $2
		WHERE id = $3 AND tenant_id = $4 AND status = 'pending' AND version = $5`,
		a.DueAt, a.UpdatedAt, a.ID, a.TenantID, expectedVersion)
	if err != nil {
		return false, fmt.Errorf("postgres: update approval due: %w", err)
	}
	return ok, nil
}

func (r *ApprovalRepository) MarkReminded(ctx context.Context, tenantID, id string, at time.Time) (bool, error) {
	ok, err := r.execAffected(ctx, `UPDATE request.approvals SET reminded_at = $1, updated_at = $1
		WHERE id = $2 AND tenant_id = $3 AND status = 'pending' AND reminded_at IS NULL`, at, id, tenantID)
	if err != nil {
		return false, fmt.Errorf("postgres: mark approval reminded: %w", err)
	}
	return ok, nil
}

func (r *ApprovalRepository) UpdatePendingDigest(ctx context.Context, tenantID string, st domain.SubjectType, subjectID, digest string) (bool, error) {
	ok, err := r.execAffected(ctx, `UPDATE request.approvals SET subject_digest = $1, updated_at = now()
		WHERE tenant_id = $2 AND subject_type = $3 AND subject_id = $4 AND status = 'pending'`, digest, tenantID, string(st), subjectID)
	if err != nil {
		return false, fmt.Errorf("postgres: update approval digest: %w", err)
	}
	return ok, nil
}

func (r *ApprovalRepository) CancelPendingForRequest(ctx context.Context, tenantID, requestID, why string, now time.Time) ([]domain.Approval, error) {
	var list []domain.Approval
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return usecase.ErrApprovalNotFound
		}
		rows, err := db.Query(ctx, `UPDATE request.approvals
			SET status = 'cancelled', comment = $1, decided_at = $2, version = version + 1, updated_at = $2
			WHERE tenant_id = $3 AND request_id = $4 AND status = 'pending'
			RETURNING `+approvalColumns, why, now, tenantID, requestID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var a domain.Approval
			if err := rows.Scan(approvalDests(&a)...); err != nil {
				return err
			}
			list = append(list, a)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: cancel pending approvals: %w", err)
	}
	return list, nil
}

func (r *ApprovalRepository) List(ctx context.Context, tenantID string, f usecase.ApprovalListFilter) ([]domain.Approval, string, error) {
	where := []string{"tenant_id = $1"}
	args := []any{tenantID}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.RequestID != "" {
		add("request_id = $%d", f.RequestID)
	}
	if f.SubjectType != "" {
		add("subject_type = $%d", string(f.SubjectType))
	}
	if f.Status != "" {
		add("status = $%d", string(f.Status))
	}
	if f.PageToken != "" {
		at, id, err := usecase.DecodeApprovalCursor(f.PageToken)
		if err != nil {
			return nil, "", err
		}
		args = append(args, at, id)
		where = append(where, fmt.Sprintf("(created_at, id) < ($%d, $%d::uuid)", len(args)-1, len(args)))
	}
	args = append(args, f.PageSize+1)
	var list []domain.Approval
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return nil
		}
		rows, err := db.Query(ctx, fmt.Sprintf(`SELECT `+approvalColumns+` FROM request.approvals WHERE %s ORDER BY created_at DESC, id DESC LIMIT $%d`,
			strings.Join(where, " AND "), len(args)), args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var a domain.Approval
			if err := rows.Scan(approvalDests(&a)...); err != nil {
				return err
			}
			list = append(list, a)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, "", fmt.Errorf("postgres: list approvals: %w", err)
	}
	next := ""
	if len(list) > f.PageSize {
		list = list[:f.PageSize]
		last := list[len(list)-1]
		next = usecase.EncodeApprovalCursor(last.CreatedAt, last.ID)
	}
	return list, next, nil
}

// ListPendingForUser matches approver snapshots (user, team, role) and the reporter, hides overdue rows and
// applies separation of duties in SQL so pagination stays correct.
func (r *ApprovalRepository) ListPendingForUser(ctx context.Context, tenantID string, f usecase.PendingForUserFilter) ([]usecase.PendingApproval, string, error) {
	args := []any{tenantID, f.UserID}
	where := []string{"a.tenant_id = $1", "a.status = 'pending'", "(a.due_at IS NULL OR a.due_at > now())",
		"(a.self_approval_allowed OR (rq.reporter_id::text <> $2::text AND NOT (a.requested_by = $2::text AND a.requested_by <> 'system')))"}
	if f.Role != "admin" {
		args = append(args, f.Role, f.TeamIDs)
		where = append(where, fmt.Sprintf(`EXISTS (SELECT 1 FROM request.approval_approvers p WHERE p.approval_id = a.id AND p.tenant_id = a.tenant_id AND (
			(p.principal_kind = 'user' AND p.principal_id = $2::text)
			OR (p.principal_kind = 'role' AND p.principal_id = $%d::text)
			OR (p.principal_kind = 'team' AND p.principal_id = ANY($%d::text[]))
			OR (p.principal_kind = 'reporter' AND rq.reporter_id::text = $2::text)))`, len(args)-1, len(args)))
	}
	if f.SubjectType != "" {
		args = append(args, string(f.SubjectType))
		where = append(where, fmt.Sprintf("a.subject_type = $%d", len(args)))
	}
	if f.PageToken != "" {
		at, id, err := usecase.DecodeApprovalCursor(f.PageToken)
		if err != nil {
			return nil, "", err
		}
		args = append(args, at, id)
		where = append(where, fmt.Sprintf("(a.created_at, a.id) < ($%d, $%d::uuid)", len(args)-1, len(args)))
	}
	args = append(args, f.PageSize+1)
	var out []usecase.PendingApproval
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return nil
		}
		cols := "a." + strings.ReplaceAll(approvalColumns, ", ", ", a.")
		rows, err := db.Query(ctx, fmt.Sprintf(`SELECT %s, rq.title, COALESCE(rq.type, ''), rq.number
			FROM request.approvals a JOIN request.requests rq ON rq.id = a.request_id AND rq.tenant_id = a.tenant_id
			WHERE %s ORDER BY a.created_at DESC, a.id DESC LIMIT $%d`, cols, strings.Join(where, " AND "), len(args)), args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p usecase.PendingApproval
			if err := rows.Scan(append(approvalDests(&p.Approval), &p.RequestTitle, &p.RequestType, &p.RequestNumber)...); err != nil {
				return err
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, "", fmt.Errorf("postgres: list pending approvals: %w", err)
	}
	next := ""
	if len(out) > f.PageSize {
		out = out[:f.PageSize]
		last := out[len(out)-1]
		next = usecase.EncodeApprovalCursor(last.CreatedAt, last.ID)
	}
	return out, next, nil
}

// ClaimDue and ClaimDueForReminder scan every tenant under the relay setting (policy relay_scan, read only);
// the winner is decided later by the per-candidate transaction, which re-checks status under the row lock.
func (r *ApprovalRepository) claim(ctx context.Context, where string, batch int) ([]usecase.ApprovalClaim, error) {
	if batch <= 0 {
		batch = 100
	}
	var claims []usecase.ApprovalClaim
	err := r.withRelayTx(ctx, func(ctx context.Context, db dbExecer) error {
		rows, err := db.Query(ctx, `SELECT tenant_id, id, request_id FROM request.approvals
			WHERE status = 'pending' AND due_at IS NOT NULL AND `+where+` ORDER BY due_at LIMIT $1`, batch)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c usecase.ApprovalClaim
			if err := rows.Scan(&c.TenantID, &c.ApprovalID, &c.RequestID); err != nil {
				return err
			}
			claims = append(claims, c)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: claim approvals: %w", err)
	}
	return claims, nil
}

func (r *ApprovalRepository) ClaimDue(ctx context.Context, batch int) ([]usecase.ApprovalClaim, error) {
	return r.claim(ctx, "due_at <= now()", batch)
}

func (r *ApprovalRepository) ClaimDueForReminder(ctx context.Context, batch int) ([]usecase.ApprovalClaim, error) {
	return r.claim(ctx, "reminded_at IS NULL AND due_at > now() AND created_at + (due_at - created_at) * 0.75 <= now()", batch)
}

// NowDB is the clock for every due/remind comparison; inside a transaction it is the transaction start time,
// which is what makes expire-vs-approve ordering consistent with the row lock.
func (r *ApprovalRepository) NowDB(ctx context.Context) (time.Time, error) {
	var now time.Time
	err := r.exec(ctx).QueryRow(ctx, "SELECT now()").Scan(&now)
	return now.UTC(), err
}
