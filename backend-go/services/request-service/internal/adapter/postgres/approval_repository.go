package postgres

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

var _ usecase.ApprovalRepository = (*Repository)(nil)

func (r *Repository) Insert(ctx context.Context, a domain.Approval) error {
	query := `
		INSERT INTO request.approvals (
			id, tenant_id, request_id, subject_type, subject_id, stage, status,
			requested_by, decided_by, decided_at, comment, due_at, version,
			subject_digest, self_approval_allowed, idempotency_key, reminded_at,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17,
			$18, $19
		)
	`
	_, err := r.exec(ctx).Exec(ctx, query,
		a.ID, a.TenantID, a.RequestID, a.SubjectType, a.SubjectID, a.Stage, a.Status,
		a.RequestedBy, a.DecidedBy, a.DecidedAt, a.Comment, a.DueAt, a.Version,
		a.SubjectDigest, a.SelfApprovalAllowed, a.IdempotencyKey, a.RemindedAt,
		a.CreatedAt, a.UpdatedAt,
	)
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
		return fmt.Errorf("postgres insert approval: %w", err)
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
		FROM request.approvals
		WHERE tenant_id = $1 AND id = $2
		FOR UPDATE
	`
	return r.scanApproval(r.exec(ctx).QueryRow(ctx, query, tenantID, id))
}

func (r *Repository) FindPendingBySubject(ctx context.Context, tenantID string, st domain.SubjectType, subjectID string) (*domain.Approval, error) {
	query := `
		SELECT
			id, tenant_id, request_id, subject_type, subject_id, stage, status,
			requested_by, decided_by, decided_at, comment, due_at, version,
			subject_digest, self_approval_allowed, idempotency_key, reminded_at,
			created_at, updated_at
		FROM request.approvals
		WHERE tenant_id = $1 AND subject_type = $2 AND subject_id = $3 AND status = 'pending'
	`
	a, err := r.scanApproval(r.exec(ctx).QueryRow(ctx, query, tenantID, st, subjectID))
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
		UPDATE request.approvals
		SET status = $1, decided_by = $2, decided_at = $3, comment = $4,
		    version = version + 1, updated_at = $5
		WHERE id = $6 AND tenant_id = $7 AND status = 'pending' AND version = $8
	`
	res, err := r.exec(ctx).Exec(ctx, query,
		a.Status, a.DecidedBy, a.DecidedAt, a.Comment, a.UpdatedAt,
		a.ID, a.TenantID, expectedVersion,
	)
	if err != nil {
		return false, fmt.Errorf("postgres update approval decision: %w", err)
	}
	return res.RowsAffected() > 0, nil
}

func (r *Repository) UpdatePendingDigest(ctx context.Context, tenantID string, st domain.SubjectType, subjectID, digest string) (bool, error) {
	query := `
		UPDATE request.approvals
		SET subject_digest = $1, updated_at = now()
		WHERE tenant_id = $2 AND subject_type = $3 AND subject_id = $4 AND status = 'pending'
	`
	res, err := r.exec(ctx).Exec(ctx, query, digest, tenantID, st, subjectID)
	if err != nil {
		return false, fmt.Errorf("postgres update approval digest: %w", err)
	}
	return res.RowsAffected() > 0, nil
}

func (r *Repository) CancelPendingForRequest(ctx context.Context, tenantID, requestID, why string, now time.Time) ([]domain.Approval, error) {
	query := `
		UPDATE request.approvals
		SET status = 'cancelled', comment = $1, version = version + 1, updated_at = $2
		WHERE tenant_id = $3 AND request_id = $4 AND status = 'pending'
		RETURNING
			id, tenant_id, request_id, subject_type, subject_id, stage, status,
			requested_by, decided_by, decided_at, comment, due_at, version,
			subject_digest, self_approval_allowed, idempotency_key, reminded_at,
			created_at, updated_at
	`
	rows, err := r.exec(ctx).Query(ctx, query, why, now, tenantID, requestID)
	if err != nil {
		return nil, fmt.Errorf("postgres cancel pending approvals: %w", err)
	}
	defer rows.(pgx.Rows).Close()

	var list []domain.Approval
	for rows.(pgx.Rows).Next() {
		var a domain.Approval
		if err := rows.(pgx.Rows).Scan(
			&a.ID, &a.TenantID, &a.RequestID, &a.SubjectType, &a.SubjectID, &a.Stage, &a.Status,
			&a.RequestedBy, &a.DecidedBy, &a.DecidedAt, &a.Comment, &a.DueAt, &a.Version,
			&a.SubjectDigest, &a.SelfApprovalAllowed, &a.IdempotencyKey, &a.RemindedAt,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.(pgx.Rows).Err()
}

func (r *Repository) List(ctx context.Context, tenantID string, f usecase.ApprovalListFilter) ([]domain.Approval, string, error) {
	query := `
		SELECT
			id, tenant_id, request_id, subject_type, subject_id, stage, status,
			requested_by, decided_by, decided_at, comment, due_at, version,
			subject_digest, self_approval_allowed, idempotency_key, reminded_at,
			created_at, updated_at
		FROM request.approvals
		WHERE tenant_id = $1
	`
	args := []any{tenantID}
	idx := 2

	if f.RequestID != "" {
		query += fmt.Sprintf(" AND request_id = $%d", idx)
		args = append(args, f.RequestID)
		idx++
	}
	if f.SubjectType != "" {
		query += fmt.Sprintf(" AND subject_type = $%d", idx)
		args = append(args, string(f.SubjectType))
		idx++
	}
	if f.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, string(f.Status))
		idx++
	}

	if f.PageToken != "" {
		dec, err := base64.URLEncoding.DecodeString(f.PageToken)
		if err == nil {
			parts := strings.SplitN(string(dec), "|", 2)
			if len(parts) == 2 {
				query += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", idx, idx+1)
				args = append(args, parts[0], parts[1])
				idx += 2
			}
		}
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", idx)
	args = append(args, f.PageSize+1)

	rows, err := r.exec(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("postgres list approvals: %w", err)
	}
	defer rows.(pgx.Rows).Close()

	var list []domain.Approval
	for rows.(pgx.Rows).Next() {
		var a domain.Approval
		if err := rows.(pgx.Rows).Scan(
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
		nextPageToken = base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("%s|%s", last.CreatedAt.Format(time.RFC3339Nano), last.ID)))
		list = list[:f.PageSize]
	}

	return list, nextPageToken, rows.(pgx.Rows).Err()
}

func (r *Repository) scanApproval(row any) (domain.Approval, error) {
	var a domain.Approval
	err := row.(pgx.Row).Scan(
		&a.ID, &a.TenantID, &a.RequestID, &a.SubjectType, &a.SubjectID, &a.Stage, &a.Status,
		&a.RequestedBy, &a.DecidedBy, &a.DecidedAt, &a.Comment, &a.DueAt, &a.Version,
		&a.SubjectDigest, &a.SelfApprovalAllowed, &a.IdempotencyKey, &a.RemindedAt,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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
		FROM request.approvals
		WHERE status = 'pending' AND due_at IS NOT NULL AND due_at <= now()
		ORDER BY due_at ASC
		LIMIT $1
	`
	rows, err := r.exec(ctx).Query(ctx, query, batch)
	if err != nil {
		return nil, fmt.Errorf("postgres claim due approvals: %w", err)
	}
	defer rows.(pgx.Rows).Close()

	var claims []usecase.ApprovalClaim
	for rows.(pgx.Rows).Next() {
		var c usecase.ApprovalClaim
		if err := rows.(pgx.Rows).Scan(&c.TenantID, &c.ApprovalID, &c.RequestID); err != nil {
			return nil, err
		}
		claims = append(claims, c)
	}
	return claims, rows.(pgx.Rows).Err()
}

func (r *Repository) ClaimDueForReminder(ctx context.Context, batch int) ([]usecase.ApprovalClaim, error) {
	if batch <= 0 {
		batch = 100
	}
	query := `
		SELECT tenant_id, id, request_id
		FROM request.approvals
		WHERE status = 'pending' AND reminded_at IS NULL AND due_at IS NOT NULL
		ORDER BY due_at ASC
		LIMIT $1
	`
	rows, err := r.exec(ctx).Query(ctx, query, batch)
	if err != nil {
		return nil, fmt.Errorf("postgres claim due reminder approvals: %w", err)
	}
	defer rows.(pgx.Rows).Close()

	var claims []usecase.ApprovalClaim
	for rows.(pgx.Rows).Next() {
		var c usecase.ApprovalClaim
		if err := rows.(pgx.Rows).Scan(&c.TenantID, &c.ApprovalID, &c.RequestID); err != nil {
			return nil, err
		}
		claims = append(claims, c)
	}
	return claims, rows.(pgx.Rows).Err()
}

func (r *Repository) NowDB(ctx context.Context) (time.Time, error) {
	var now time.Time
	err := r.exec(ctx).QueryRow(ctx, "SELECT now()").Scan(&now)
	return now, err
}
