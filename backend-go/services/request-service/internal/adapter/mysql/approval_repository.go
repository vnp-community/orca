package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
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

func scanApproval(row rowScanner) (domain.Approval, error) {
	var a domain.Approval
	if err := row.Scan(approvalDests(&a)...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return a, usecase.ErrApprovalNotFound
		}
		return a, fmt.Errorf("mysql: scan approval: %w", err)
	}
	return a, nil
}

func (r *ApprovalRepository) Insert(ctx context.Context, a domain.Approval) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if a.TenantID != tenantID {
			return domain.ErrRequestTenantRequired()
		}
		_, err := db.ExecContext(ctx, `INSERT INTO approvals (`+approvalColumns+`)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			a.ID, a.TenantID, a.RequestID, string(a.SubjectType), a.SubjectID, a.Stage, string(a.Status),
			a.RequestedBy, a.DecidedBy, a.DecidedAt, a.Comment, a.DueAt, a.Version,
			a.SubjectDigest, a.SelfApprovalAllowed, a.IdempotencyKey, a.RemindedAt, a.CreatedAt.UTC(), a.UpdatedAt.UTC())
		if err != nil {
			var myErr *mysql.MySQLError
			if errors.As(err, &myErr) && myErr.Number == 1062 {
				if strings.Contains(myErr.Message, "approvals_one_pending") {
					return usecase.ErrPendingExists
				}
				if strings.Contains(myErr.Message, "approvals_idem") {
					return usecase.ErrIdempotencyConflict
				}
			}
			return fmt.Errorf("mysql: insert approval: %w", err)
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
		got, err := scanApproval(db.QueryRowContext(ctx, `SELECT `+approvalColumns+` FROM approvals WHERE tenant_id = ? AND `+cond+suffix,
			append([]any{tenantID}, args...)...))
		out = got
		return err
	})
	return out, err
}

func (r *ApprovalRepository) Get(ctx context.Context, tenantID, id string) (domain.Approval, error) {
	return r.getWhere(ctx, tenantID, "id = ?", false, id)
}

func (r *ApprovalRepository) GetForUpdate(ctx context.Context, tenantID, id string) (domain.Approval, error) {
	return r.getWhere(ctx, tenantID, "id = ?", true, id)
}

func (r *ApprovalRepository) FindPendingBySubject(ctx context.Context, tenantID string, st domain.SubjectType, subjectID string) (*domain.Approval, error) {
	a, err := r.getWhere(ctx, tenantID, "subject_type = ? AND subject_id = ? AND status = 'pending'", false, string(st), subjectID)
	if errors.Is(err, usecase.ErrApprovalNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *ApprovalRepository) FindByIdempotencyKey(ctx context.Context, tenantID, key string) (*domain.Approval, error) {
	a, err := r.getWhere(ctx, tenantID, "idempotency_key = ?", false, key)
	if errors.Is(err, usecase.ErrApprovalNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *ApprovalRepository) execAffected(ctx context.Context, query string, args ...any) (bool, error) {
	var affected int64
	err := r.scoped(ctx, func(ctx context.Context, _ string, db dbExecer) error {
		res, err := db.ExecContext(ctx, query, args...)
		if err != nil {
			return err
		}
		affected, err = res.RowsAffected()
		return err
	})
	return affected > 0, err
}

func (r *ApprovalRepository) UpdateDecision(ctx context.Context, a domain.Approval, expectedVersion int64) (bool, error) {
	ok, err := r.execAffected(ctx, `UPDATE approvals
		SET status = ?, decided_by = ?, decided_at = ?, comment = ?, version = version + 1, updated_at = ?
		WHERE id = ? AND tenant_id = ? AND status = 'pending' AND version = ?`,
		string(a.Status), a.DecidedBy, a.DecidedAt, a.Comment, a.UpdatedAt.UTC(), a.ID, a.TenantID, expectedVersion)
	if err != nil {
		return false, fmt.Errorf("mysql: update approval decision: %w", err)
	}
	return ok, nil
}

func (r *ApprovalRepository) UpdateDue(ctx context.Context, a domain.Approval, expectedVersion int64) (bool, error) {
	ok, err := r.execAffected(ctx, `UPDATE approvals
		SET due_at = ?, reminded_at = NULL, version = version + 1, updated_at = ?
		WHERE id = ? AND tenant_id = ? AND status = 'pending' AND version = ?`,
		a.DueAt, a.UpdatedAt.UTC(), a.ID, a.TenantID, expectedVersion)
	if err != nil {
		return false, fmt.Errorf("mysql: update approval due: %w", err)
	}
	return ok, nil
}

func (r *ApprovalRepository) MarkReminded(ctx context.Context, tenantID, id string, at time.Time) (bool, error) {
	ok, err := r.execAffected(ctx, `UPDATE approvals SET reminded_at = ?, updated_at = ?
		WHERE id = ? AND tenant_id = ? AND status = 'pending' AND reminded_at IS NULL`, at.UTC(), at.UTC(), id, tenantID)
	if err != nil {
		return false, fmt.Errorf("mysql: mark approval reminded: %w", err)
	}
	return ok, nil
}

func (r *ApprovalRepository) UpdatePendingDigest(ctx context.Context, tenantID string, st domain.SubjectType, subjectID, digest string) (bool, error) {
	ok, err := r.execAffected(ctx, `UPDATE approvals SET subject_digest = ?, updated_at = CURRENT_TIMESTAMP(6)
		WHERE tenant_id = ? AND subject_type = ? AND subject_id = ? AND status = 'pending'`, digest, tenantID, string(st), subjectID)
	if err != nil {
		return false, fmt.Errorf("mysql: update approval digest: %w", err)
	}
	return ok, nil
}

// CancelPendingForRequest has no UPDATE ... RETURNING on MySQL: lock the rows, update them, return the new state.
func (r *ApprovalRepository) CancelPendingForRequest(ctx context.Context, tenantID, requestID, why string, now time.Time) ([]domain.Approval, error) {
	var list []domain.Approval
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return usecase.ErrApprovalNotFound
		}
		rows, err := db.QueryContext(ctx, `SELECT `+approvalColumns+` FROM approvals
			WHERE tenant_id = ? AND request_id = ? AND status = 'pending' FOR UPDATE`, tenantID, requestID)
		if err != nil {
			return err
		}
		var ids []any
		for rows.Next() {
			var a domain.Approval
			if err := rows.Scan(approvalDests(&a)...); err != nil {
				_ = rows.Close()
				return err
			}
			a.Status, a.Comment, a.DecidedAt, a.UpdatedAt = domain.ApprovalStatusCancelled, why, &now, now
			a.Version++
			list = append(list, a)
			ids = append(ids, a.ID)
		}
		_ = rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		args := append([]any{why, now.UTC(), now.UTC(), tenantID}, ids...)
		_, err = db.ExecContext(ctx, `UPDATE approvals SET status = 'cancelled', comment = ?, decided_at = ?, version = version + 1, updated_at = ?
			WHERE tenant_id = ? AND status = 'pending' AND id IN (`+strings.Repeat("?,", len(ids)-1)+`?)`, args...)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("mysql: cancel pending approvals: %w", err)
	}
	return list, nil
}

func (r *ApprovalRepository) List(ctx context.Context, tenantID string, f usecase.ApprovalListFilter) ([]domain.Approval, string, error) {
	where := []string{"tenant_id = ?"}
	args := []any{tenantID}
	if f.RequestID != "" {
		where, args = append(where, "request_id = ?"), append(args, f.RequestID)
	}
	if f.SubjectType != "" {
		where, args = append(where, "subject_type = ?"), append(args, string(f.SubjectType))
	}
	if f.Status != "" {
		where, args = append(where, "status = ?"), append(args, string(f.Status))
	}
	if f.PageToken != "" {
		at, id, err := usecase.DecodeApprovalCursor(f.PageToken)
		if err != nil {
			return nil, "", err
		}
		where, args = append(where, "(created_at < ? OR (created_at = ? AND id < ?))"), append(args, at, at, id)
	}
	args = append(args, f.PageSize+1)
	var list []domain.Approval
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return nil
		}
		rows, err := db.QueryContext(ctx, `SELECT `+approvalColumns+` FROM approvals WHERE `+strings.Join(where, " AND ")+
			` ORDER BY created_at DESC, id DESC LIMIT ?`, args...)
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
		return nil, "", fmt.Errorf("mysql: list approvals: %w", err)
	}
	next := ""
	if len(list) > f.PageSize {
		list = list[:f.PageSize]
		last := list[len(list)-1]
		next = usecase.EncodeApprovalCursor(last.CreatedAt, last.ID)
	}
	return list, next, nil
}

func (r *ApprovalRepository) ListPendingForUser(ctx context.Context, tenantID string, f usecase.PendingForUserFilter) ([]usecase.PendingApproval, string, error) {
	where := []string{"a.tenant_id = ?", "a.status = 'pending'", "(a.due_at IS NULL OR a.due_at > CURRENT_TIMESTAMP(6))",
		"(a.self_approval_allowed = 1 OR (rq.reporter_id <> ? AND NOT (a.requested_by = ? AND a.requested_by <> 'system')))"}
	args := []any{tenantID, f.UserID, f.UserID}
	if f.Role != "admin" {
		branches := []string{"(p.principal_kind = 'user' AND p.principal_id = ?)", "(p.principal_kind = 'role' AND p.principal_id = ?)", "(p.principal_kind = 'reporter' AND rq.reporter_id = ?)"}
		args = append(args, f.UserID, f.Role, f.UserID)
		if len(f.TeamIDs) > 0 {
			branches = append(branches, "(p.principal_kind = 'team' AND p.principal_id IN ("+strings.Repeat("?,", len(f.TeamIDs)-1)+"?))")
			for _, t := range f.TeamIDs {
				args = append(args, t)
			}
		}
		where = append(where, `EXISTS (SELECT 1 FROM approval_approvers p WHERE p.approval_id = a.id AND p.tenant_id = a.tenant_id AND (`+strings.Join(branches, " OR ")+`))`)
	}
	if f.SubjectType != "" {
		where, args = append(where, "a.subject_type = ?"), append(args, string(f.SubjectType))
	}
	if f.PageToken != "" {
		at, id, err := usecase.DecodeApprovalCursor(f.PageToken)
		if err != nil {
			return nil, "", err
		}
		where, args = append(where, "(a.created_at < ? OR (a.created_at = ? AND a.id < ?))"), append(args, at, at, id)
	}
	args = append(args, f.PageSize+1)
	var out []usecase.PendingApproval
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return nil
		}
		cols := "a." + strings.ReplaceAll(approvalColumns, ", ", ", a.")
		rows, err := db.QueryContext(ctx, `SELECT `+cols+`, rq.title, COALESCE(rq.type, ''), rq.number
			FROM approvals a JOIN requests rq ON rq.id = a.request_id AND rq.tenant_id = a.tenant_id
			WHERE `+strings.Join(where, " AND ")+` ORDER BY a.created_at DESC, a.id DESC LIMIT ?`, args...)
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
		return nil, "", fmt.Errorf("mysql: list pending approvals: %w", err)
	}
	next := ""
	if len(out) > f.PageSize {
		out = out[:f.PageSize]
		last := out[len(out)-1]
		next = usecase.EncodeApprovalCursor(last.CreatedAt, last.ID)
	}
	return out, next, nil
}

// claim scans every tenant (MySQL has no RLS); the per-candidate transaction re-checks under the row lock.
func (r *ApprovalRepository) claim(ctx context.Context, where string, batch int) ([]usecase.ApprovalClaim, error) {
	if batch <= 0 {
		batch = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT tenant_id, id, request_id FROM approvals
		WHERE status = 'pending' AND due_at IS NOT NULL AND `+where+` ORDER BY due_at LIMIT ?`, batch)
	if err != nil {
		return nil, fmt.Errorf("mysql: claim approvals: %w", err)
	}
	defer rows.Close()
	var claims []usecase.ApprovalClaim
	for rows.Next() {
		var c usecase.ApprovalClaim
		if err := rows.Scan(&c.TenantID, &c.ApprovalID, &c.RequestID); err != nil {
			return nil, err
		}
		claims = append(claims, c)
	}
	return claims, rows.Err()
}

func (r *ApprovalRepository) ClaimDue(ctx context.Context, batch int) ([]usecase.ApprovalClaim, error) {
	return r.claim(ctx, "due_at <= CURRENT_TIMESTAMP(6)", batch)
}

func (r *ApprovalRepository) ClaimDueForReminder(ctx context.Context, batch int) ([]usecase.ApprovalClaim, error) {
	return r.claim(ctx, `reminded_at IS NULL AND due_at > CURRENT_TIMESTAMP(6)
		AND TIMESTAMPADD(MICROSECOND, TIMESTAMPDIFF(MICROSECOND, created_at, due_at) * 3 DIV 4, created_at) <= CURRENT_TIMESTAMP(6)`, batch)
}

func (r *ApprovalRepository) NowDB(ctx context.Context) (time.Time, error) {
	var now time.Time
	err := r.exec(ctx).QueryRowContext(ctx, "SELECT CURRENT_TIMESTAMP(6)").Scan(&now)
	return now.UTC(), err
}
