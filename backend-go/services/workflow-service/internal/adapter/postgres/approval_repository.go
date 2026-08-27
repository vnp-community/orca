// Package postgres (this file): ApprovalStore implements
// usecase.ApprovalRepository against workflow.approvals — see
// migrations/0008_template_visibility_sharing.up.sql's table comment
// ("mirrors orchestration-service.md §5's decision_gates shape
// deliberately"). Split into its own type over the same pool Repository
// uses, matching SshTargetStore's precedent in infra-fleet-service (a
// separate Go type per table-cluster sharing one *pgxpool.Pool, not one
// God-Repository) — see this package's Repository doc comment for the
// identical reasoning (Get's method-name collision risk once
// TemplateRepository/ApprovalRepository are both live).
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// pgUniqueViolation is Postgres's SQLSTATE for a unique-constraint
// violation — see https://www.postgresql.org/docs/current/errcodes-appendix.html,
// same convention as auth-service/project-service's own postgres adapters.
const pgUniqueViolation = "23505"

// ApprovalStore implements usecase.ApprovalRepository.
type ApprovalStore struct {
	pool *pgxpool.Pool
}

// NewApprovalStore builds an ApprovalStore over the same pool Repository
// uses — both are thin wrappers over one PostgreSQL connection pool, per
// architecture/05-data-architecture.md's database-per-service rule.
func NewApprovalStore(pool *pgxpool.Pool) *ApprovalStore {
	return &ApprovalStore{pool: pool}
}

// WithTx implements usecase.ApprovalRepository.WithTx — see
// Repository.WithTx's doc comment for the identical commit/rollback
// contract.
func (s *ApprovalStore) WithTx(ctx context.Context, fn func(tx usecase.ApprovalRepositoryTx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(&approvalTx{tx: tx}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit tx: %w", err)
	}
	return nil
}

// ListPending keyset-paginates tenantID's pending approvals — see
// usecase.ApprovalRepository.ListPending's doc comment.
func (s *ApprovalStore) ListPending(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Approval, string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, template_id, requested_by, status, COALESCE(resolved_by::text, ''), resolved_at, created_at
		FROM workflow.approvals
		WHERE tenant_id = $1 AND status = 'pending' AND id::text > $2
		ORDER BY id
		LIMIT $3
	`, tenantID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query pending approvals: %w", err)
	}
	defer rows.Close()

	var out []domain.Approval
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, "", fmt.Errorf("postgres: scan approval row: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate pending approval rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

// rowScanner is the common subset of pgx.Row/pgx.Rows scanApproval needs —
// lets one scan function serve both QueryRow (approvalTx.Get) and Query
// (ListPending) call sites.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanApproval(row rowScanner) (domain.Approval, error) {
	var a domain.Approval
	var status string
	err := row.Scan(&a.ID, &a.TenantID, &a.TemplateID, &a.RequestedBy, &status, &a.ResolvedBy, &a.ResolvedAt, &a.CreatedAt)
	if err != nil {
		return domain.Approval{}, err
	}
	a.Status = domain.ApprovalStatus(status)
	return a, nil
}

// approvalTx implements usecase.ApprovalRepositoryTx — every method
// participates in the transaction tx belongs to.
type approvalTx struct {
	tx pgx.Tx
}

// Get returns domain.ErrApprovalNotFound (wrapped) if no matching row
// exists for approvalID.
func (t *approvalTx) Get(ctx context.Context, approvalID string) (domain.Approval, error) {
	row := t.tx.QueryRow(ctx, `
		SELECT id, tenant_id, template_id, requested_by, status, COALESCE(resolved_by::text, ''), resolved_at, created_at
		FROM workflow.approvals
		WHERE id = $1
	`, approvalID)
	a, err := scanApproval(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Approval{}, domain.ErrApprovalNotFound
	}
	if err != nil {
		return domain.Approval{}, fmt.Errorf("postgres: query approval: %w", err)
	}
	return a, nil
}

// Update persists approval's mutable fields (status, resolved_by,
// resolved_at) — called after Approve/Reject.
func (t *approvalTx) Update(ctx context.Context, approval domain.Approval) error {
	tag, err := t.tx.Exec(ctx, `
		UPDATE workflow.approvals
		SET status = $1, resolved_by = NULLIF($2, '')::uuid, resolved_at = $3
		WHERE id = $4
	`, string(approval.Status), approval.ResolvedBy, approval.ResolvedAt, approval.ID)
	if err != nil {
		return fmt.Errorf("postgres: update approval: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrApprovalNotFound
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
// idx_workflow_approvals_one_pending_per_template (migrations/0008) to
// reject a second concurrent pending row for the same template at the
// constraint level; the caller (usecase.PublishTemplate) maps that
// constraint violation to a clean, typed conflict error rather than
// leaking the raw Postgres error.
func (t *approvalTx) CreateTx(ctx context.Context, approval domain.Approval) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO workflow.approvals (id, tenant_id, template_id, requested_by, status)
		VALUES ($1, $2, $3, $4, $5)
	`, approval.ID, approval.TenantID, approval.TemplateID, approval.RequestedBy, string(approval.Status))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return fmt.Errorf("postgres: insert approval: %w", domain.ErrApprovalAlreadyPending)
		}
		return fmt.Errorf("postgres: insert approval: %w", err)
	}
	return nil
}
