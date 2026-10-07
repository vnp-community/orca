package mysql

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

var _ usecase.ApprovalRepository = (*Repository)(nil)

func (r *Repository) Insert(ctx context.Context, a domain.Approval) error {
	query := `
		INSERT INTO approvals (
			id, tenant_id, request_id, subject_type, subject_id, stage, status,
			requested_by, decided_by, decided_at, comment, due_at, version,
			subject_digest, self_approval_allowed, idempotency_key, reminded_at,
			created_at, updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?
		)
	`
	_, err := r.exec(ctx).ExecContext(ctx, query,
		a.ID, a.TenantID, a.RequestID, a.SubjectType, a.SubjectID, a.Stage, a.Status,
		a.RequestedBy, a.DecidedBy, a.DecidedAt, a.Comment, a.DueAt, a.Version,
		a.SubjectDigest, a.SelfApprovalAllowed, a.IdempotencyKey, a.RemindedAt,
		a.CreatedAt, a.UpdatedAt,
	)
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
		return fmt.Errorf("mysql insert approval: %w", err)
	}
	return nil
}

func (r *Repository) GetForUpdate(ctx context.Context, tenantID, id string) (domain.Approval, error) {
	query := `
		SELECT
			id, tenant_id, request_id, subject_type, subject_id, stage, status,
			requested_by, decided_by, decided_at, comment, due_at, version,
			subject_digest, self_approval_allowed, idempotency_key, reminded_at,
			created_at, updated_at
		FROM approvals
		WHERE tenant_id = ? AND id = ?
		FOR UPDATE
	`
	return r.scanApproval(r.exec(ctx).QueryRowContext(ctx, query, tenantID, id))
}

func (r *Repository) FindPendingBySubject(ctx context.Context, tenantID string, st domain.SubjectType, subjectID string) (*domain.Approval, error) {
	query := `
		SELECT
			id, tenant_id, request_id, subject_type, subject_id, stage, status,
			requested_by, decided_by, decided_at, comment, due_at, version,
			subject_digest, self_approval_allowed, idempotency_key, reminded_at,
			created_at, updated_at
		FROM approvals
		WHERE tenant_id = ? AND subject_type = ? AND subject_id = ? AND status = 'pending'
	`
	a, err := r.scanApproval(r.exec(ctx).QueryRowContext(ctx, query, tenantID, st, subjectID))
	if err != nil {
		if errors.Is(err, usecase.ErrApprovalNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

func (r *Repository) UpdateDecision(ctx context.Context, a domain.Approval, expectedVersion int64) (bool, error) {
	query := `
		UPDATE approvals
		SET status = ?, decided_by = ?, decided_at = ?, comment = ?,
		    version = version + 1, updated_at = ?
		WHERE id = ? AND tenant_id = ? AND status = 'pending' AND version = ?
	`
	res, err := r.exec(ctx).ExecContext(ctx, query,
		a.Status, a.DecidedBy, a.DecidedAt, a.Comment, a.UpdatedAt,
		a.ID, a.TenantID, expectedVersion,
	)
	if err != nil {
		return false, fmt.Errorf("mysql update approval decision: %w", err)
	}
	affected, err := res.RowsAffected()
	return affected > 0, err
}

func (r *Repository) UpdatePendingDigest(ctx context.Context, tenantID string, st domain.SubjectType, subjectID, digest string) (bool, error) {
	query := `
		UPDATE approvals
		SET subject_digest = ?, updated_at = CURRENT_TIMESTAMP(6)
		WHERE tenant_id = ? AND subject_type = ? AND subject_id = ? AND status = 'pending'
	`
	res, err := r.exec(ctx).ExecContext(ctx, query, digest, tenantID, st, subjectID)
	if err != nil {
		return false, fmt.Errorf("mysql update approval digest: %w", err)
	}
	affected, err := res.RowsAffected()
	return affected > 0, err
}

func (r *Repository) CancelPendingForRequest(ctx context.Context, tenantID, requestID, why string, now time.Time) ([]domain.Approval, error) {
	selQuery := `
		SELECT
			id, tenant_id, request_id, subject_type, subject_id, stage, status,
			requested_by, decided_by, decided_at, comment, due_at, version,
			subject_digest, self_approval_allowed, idempotency_key, reminded_at,
			created_at, updated_at
		FROM approvals
		WHERE tenant_id = ? AND request_id = ? AND status = 'pending'
		FOR UPDATE
	`
	rows, err := r.exec(ctx).QueryContext(ctx, selQuery, tenantID, requestID)
	if err != nil {
		return nil, fmt.Errorf("mysql select pending for cancel: %w", err)
	}
	defer rows.Close()

	var list []domain.Approval
	var ids []any
	for rows.Next() {
		var a domain.Approval
		if err := rows.Scan(
			&a.ID, &a.TenantID, &a.RequestID, &a.SubjectType, &a.SubjectID, &a.Stage, &a.Status,
			&a.RequestedBy, &a.DecidedBy, &a.DecidedAt, &a.Comment, &a.DueAt, &a.Version,
			&a.SubjectDigest, &a.SelfApprovalAllowed, &a.IdempotencyKey, &a.RemindedAt,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		a.Status = domain.ApprovalStatusCancelled
		a.Comment = why
		a.Version++
		a.UpdatedAt = now
		list = append(list, a)
		ids = append(ids, a.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	placeholders := strings.Repeat("?,", len(ids)-1) + "?"
	updQuery := fmt.Sprintf(`
		UPDATE approvals
		SET status = 'cancelled', comment = ?, version = version + 1, updated_at = ?
		WHERE id IN (%s) AND status = 'pending' AND tenant_id = ?
	`, placeholders)
	
	args := []any{why, now}
	args = append(args, ids...)
	args = append(args, tenantID)

	if _, err := r.exec(ctx).ExecContext(ctx, updQuery, args...); err != nil {
		return nil, fmt.Errorf("mysql update cancel pending: %w", err)
	}

	return list, nil
}

func (r *Repository) List(ctx context.Context, tenantID string, f usecase.ApprovalListFilter) ([]domain.Approval, string, error) {
	query := `
		SELECT
			id, tenant_id, request_id, subject_type, subject_id, stage, status,
			requested_by, decided_by, decided_at, comment, due_at, version,
			subject_digest, self_approval_allowed, idempotency_key, reminded_at,
			created_at, updated_at
		FROM approvals
		WHERE tenant_id = ?
	`
	args := []any{tenantID}

	if f.RequestID != "" {
		query += " AND request_id = ?"
		args = append(args, f.RequestID)
	}
	if f.SubjectType != "" {
		query += " AND subject_type = ?"
		args = append(args, string(f.SubjectType))
	}
	if f.Status != "" {
		query += " AND status = ?"
		args = append(args, string(f.Status))
	}

	if f.PageToken != "" {
		dec, err := base64.URLEncoding.DecodeString(f.PageToken)
		if err == nil {
			parts := strings.SplitN(string(dec), "|", 2)
			if len(parts) == 2 {
				query += " AND (created_at, id) < (?, ?)"
				args = append(args, parts[0], parts[1])
			}
		}
	}

	query += " ORDER BY created_at DESC, id DESC LIMIT ?"
	args = append(args, f.PageSize+1)

	rows, err := r.exec(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("mysql list approvals: %w", err)
	}
	defer rows.Close()

	var list []domain.Approval
	for rows.Next() {
		var a domain.Approval
		if err := rows.Scan(
			&a.ID, &a.TenantID, &a.RequestID, &a.SubjectType, &a.SubjectID, &a.Stage, &a.Status,
			&a.RequestedBy, &a.DecidedBy, &a.DecidedAt, &a.Comment, &a.DueAt, &a.Version,
			&a.SubjectDigest, &a.SelfApprovalAllowed, &a.IdempotencyKey, &a.RemindedAt,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, "", err
		}
		list = append(list, a)
	}

	nextPageToken := ""
	if len(list) > f.PageSize {
		last := list[f.PageSize-1]
		nextPageToken = base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("%s|%s", last.CreatedAt.Format("2006-01-02 15:04:05.999999"), last.ID)))
		list = list[:f.PageSize]
	}

	return list, nextPageToken, rows.Err()
}

func (r *Repository) scanApproval(row *sql.Row) (domain.Approval, error) {
	var a domain.Approval
	err := row.Scan(
		&a.ID, &a.TenantID, &a.RequestID, &a.SubjectType, &a.SubjectID, &a.Stage, &a.Status,
		&a.RequestedBy, &a.DecidedBy, &a.DecidedAt, &a.Comment, &a.DueAt, &a.Version,
		&a.SubjectDigest, &a.SelfApprovalAllowed, &a.IdempotencyKey, &a.RemindedAt,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return a, usecase.ErrApprovalNotFound
		}
		return a, fmt.Errorf("scan approval: %w", err)
	}
	return a, nil
}

func (r *Repository) ClaimDue(ctx context.Context, batch int) ([]usecase.ApprovalClaim, error) {
	if batch <= 0 {
		batch = 100
	}
	query := `
		SELECT tenant_id, id, request_id
		FROM approvals
		WHERE status = 'pending' AND due_at IS NOT NULL AND due_at <= CURRENT_TIMESTAMP(6)
		ORDER BY due_at ASC
		LIMIT ?
	`
	rows, err := r.exec(ctx).QueryContext(ctx, query, batch)
	if err != nil {
		return nil, fmt.Errorf("mysql claim due approvals: %w", err)
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

func (r *Repository) ClaimDueForReminder(ctx context.Context, batch int) ([]usecase.ApprovalClaim, error) {
	if batch <= 0 {
		batch = 100
	}
	query := `
		SELECT tenant_id, id, request_id
		FROM approvals
		WHERE status = 'pending' AND reminded_at IS NULL AND due_at IS NOT NULL
		ORDER BY due_at ASC
		LIMIT ?
	`
	rows, err := r.exec(ctx).QueryContext(ctx, query, batch)
	if err != nil {
		return nil, fmt.Errorf("mysql claim due reminder approvals: %w", err)
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

func (r *Repository) NowDB(ctx context.Context) (time.Time, error) {
	var now time.Time
	var tStr string
	err := r.exec(ctx).QueryRowContext(ctx, "SELECT CURRENT_TIMESTAMP(6)").Scan(&tStr)
	if err == nil {
		now, err = time.Parse("2006-01-02 15:04:05.999999", tStr)
	}
	return now, err
}
