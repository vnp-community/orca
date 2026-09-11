// Package mysql (this file): ApprovalStore implements
// usecase.ApprovalRepository against `approvals` — mirrors
// internal/adapter/postgres's ApprovalStore/approvalTx split (a separate
// Go type over the same *sql.DB Repository uses, not one God-Repository —
// same Get-name-collision reasoning as that file's doc comment).
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	mysqldriver "github.com/go-sql-driver/mysql"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// mysqlDuplicateEntry is MySQL/TiDB's error number for a unique-constraint
// violation (ER_DUP_ENTRY) — see
// https://dev.mysql.com/doc/mysql-errors/8.0/en/server-error-reference.html#error_er_dup_entry,
// the MySQL/TiDB analogue of internal/adapter/postgres's pgUniqueViolation
// SQLSTATE constant.
const mysqlDuplicateEntry = 1062

// ApprovalStore implements usecase.ApprovalRepository.
type ApprovalStore struct {
	db *sql.DB
}

// NewApprovalStore builds an ApprovalStore over the same *sql.DB Repository
// uses.
func NewApprovalStore(db *sql.DB) *ApprovalStore {
	return &ApprovalStore{db: db}
}

// WithTx implements usecase.ApprovalRepository.WithTx — see
// Repository.WithTx's doc comment for the identical commit/rollback
// contract.
func (s *ApprovalStore) WithTx(ctx context.Context, fn func(tx usecase.ApprovalRepositoryTx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(&approvalTx{tx: tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit tx: %w", err)
	}
	return nil
}

// ListPending keyset-paginates tenantID's pending approvals — see
// usecase.ApprovalRepository.ListPending's doc comment.
func (s *ApprovalStore) ListPending(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Approval, string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, template_id, requested_by, status, COALESCE(resolved_by, ''), resolved_at, created_at
		FROM approvals
		WHERE tenant_id = ? AND status = 'pending' AND id > ?
		ORDER BY id
		LIMIT ?
	`, tenantID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: query pending approvals: %w", err)
	}
	defer rows.Close()

	var out []domain.Approval
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, "", fmt.Errorf("mysql: scan approval row: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate pending approval rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

func scanApproval(row rowScanner) (domain.Approval, error) {
	var a domain.Approval
	var status string
	var resolvedAt sql.NullTime
	err := row.Scan(&a.ID, &a.TenantID, &a.TemplateID, &a.RequestedBy, &status, &a.ResolvedBy, &resolvedAt, &a.CreatedAt)
	if err != nil {
		return domain.Approval{}, err
	}
	a.Status = domain.ApprovalStatus(status)
	if resolvedAt.Valid {
		a.ResolvedAt = &resolvedAt.Time
	}
	return a, nil
}

// approvalTx implements usecase.ApprovalRepositoryTx — every method
// participates in the transaction tx belongs to.
type approvalTx struct {
	tx *sql.Tx
}

// Get returns domain.ErrApprovalNotFound (wrapped) if no matching row
// exists for approvalID.
func (t *approvalTx) Get(ctx context.Context, approvalID string) (domain.Approval, error) {
	row := t.tx.QueryRowContext(ctx, `
		SELECT id, tenant_id, template_id, requested_by, status, COALESCE(resolved_by, ''), resolved_at, created_at
		FROM approvals
		WHERE id = ?
	`, approvalID)
	a, err := scanApproval(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Approval{}, domain.ErrApprovalNotFound
	}
	if err != nil {
		return domain.Approval{}, fmt.Errorf("mysql: query approval: %w", err)
	}
	return a, nil
}

// Update persists approval's mutable fields — SELECT ... FOR UPDATE
// existence check + UPDATE replaces the Postgres adapter's
// RowsAffected()==0 check, see repository.go's package doc comment.
func (t *approvalTx) Update(ctx context.Context, approval domain.Approval) error {
	var exists int
	err := t.tx.QueryRowContext(ctx, `SELECT 1 FROM approvals WHERE id = ? FOR UPDATE`, approval.ID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrApprovalNotFound
	}
	if err != nil {
		return fmt.Errorf("mysql: lock approval: %w", err)
	}

	if _, err := t.tx.ExecContext(ctx, `
		UPDATE approvals SET status = ?, resolved_by = ?, resolved_at = ? WHERE id = ?
	`, string(approval.Status), nullableString(approval.ResolvedBy), approval.ResolvedAt, approval.ID); err != nil {
		return fmt.Errorf("mysql: update approval: %w", err)
	}
	return nil
}

// Templates returns a TemplateRepositoryTx scoped to the SAME transaction
// as this approvalTx — see usecase.ApprovalRepositoryTx.Templates' doc
// comment.
func (t *approvalTx) Templates() usecase.TemplateRepositoryTx {
	return &templateTx{tx: t.tx}
}

// CreateTx inserts a new pending approval row — relies on
// migrations/mysql/0009_template_visibility_sharing.up.sql's
// idx_workflow_approvals_one_pending_per_template generated-column unique
// index to reject a second concurrent pending row for the same template at
// the constraint level, same as the Postgres adapter's partial unique
// index. Maps MySQL error 1062 (ER_DUP_ENTRY) to
// domain.ErrApprovalAlreadyPending — the MySQL/TiDB analogue of the
// Postgres adapter's pgUniqueViolation (SQLSTATE 23505) check.
func (t *approvalTx) CreateTx(ctx context.Context, approval domain.Approval) error {
	_, err := t.tx.ExecContext(ctx, `
		INSERT INTO approvals (id, tenant_id, template_id, requested_by, status)
		VALUES (?, ?, ?, ?, ?)
	`, approval.ID, approval.TenantID, approval.TemplateID, approval.RequestedBy, string(approval.Status))
	if err != nil {
		var mysqlErr *mysqldriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlDuplicateEntry {
			return fmt.Errorf("mysql: insert approval: %w", domain.ErrApprovalAlreadyPending)
		}
		return fmt.Errorf("mysql: insert approval: %w", err)
	}
	return nil
}
