package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// StepApprovalStore implements usecase.StepApprovalRepository on workflow.step_approvals.
type StepApprovalStore struct {
	pool *pgxpool.Pool
}

func NewStepApprovalStore(pool *pgxpool.Pool) *StepApprovalStore {
	return &StepApprovalStore{pool: pool}
}

var _ usecase.StepApprovalRepository = (*StepApprovalStore)(nil)

const stepApprovalColumns = `id::text, tenant_id::text, execution_id::text, step_id, project_id, kind, subject, decision, decision_hash, status,
	COALESCE(resolved_by, ''), COALESCE(rationale, ''), created_at, resolved_at, submitted_at`

func scanStepApproval(row rowScanner) (domain.StepApproval, error) {
	var a domain.StepApproval
	var subject, decision []byte
	var status string
	err := row.Scan(&a.ID, &a.TenantID, &a.ExecutionID, &a.StepID, &a.ProjectID, &a.Kind, &subject, &decision, &a.DecisionHash, &status,
		&a.ResolvedBy, &a.Rationale, &a.CreatedAt, &a.ResolvedAt, &a.SubmittedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.StepApproval{}, fmt.Errorf("postgres: %w", domain.ErrStepApprovalNotFound)
	}
	if err != nil {
		return domain.StepApproval{}, fmt.Errorf("postgres: scan step approval: %w", err)
	}
	a.Subject, a.Decision, a.Status = json.RawMessage(subject), json.RawMessage(decision), domain.StepApprovalStatus(status)
	return a, nil
}

func (s *StepApprovalStore) UpsertPendingStepApproval(ctx context.Context, a domain.StepApproval) (domain.StepApproval, error) {
	if !isUUID(a.TenantID) || !isUUID(a.ExecutionID) {
		return domain.StepApproval{}, fmt.Errorf("postgres: %w", domain.ErrStepApprovalInvalid)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.StepApproval{}, fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	existing, err := scanStepApproval(tx.QueryRow(ctx, `SELECT `+stepApprovalColumns+` FROM workflow.step_approvals
		WHERE tenant_id = $1 AND execution_id = $2 AND step_id = $3 AND decision_hash = $4`, a.TenantID, a.ExecutionID, a.StepID, a.DecisionHash))
	if err == nil {
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, domain.ErrStepApprovalNotFound) {
		return domain.StepApproval{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE workflow.step_approvals SET status = 'superseded', resolved_at = now()
		WHERE tenant_id = $1 AND execution_id = $2 AND step_id = $3 AND status = 'pending'`, a.TenantID, a.ExecutionID, a.StepID); err != nil {
		return domain.StepApproval{}, fmt.Errorf("postgres: supersede step approvals: %w", err)
	}
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	created, err := scanStepApproval(tx.QueryRow(ctx, `INSERT INTO workflow.step_approvals
		(id, tenant_id, execution_id, step_id, project_id, kind, subject, decision, decision_hash, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending', $10)
		RETURNING `+stepApprovalColumns,
		a.ID, a.TenantID, a.ExecutionID, a.StepID, a.ProjectID, a.Kind, []byte(a.Subject), []byte(a.Decision), a.DecisionHash, a.CreatedAt))
	if err != nil {
		return domain.StepApproval{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.StepApproval{}, fmt.Errorf("postgres: commit tx: %w", err)
	}
	return created, nil
}

func (s *StepApprovalStore) GetStepApproval(ctx context.Context, tenantID, id string) (domain.StepApproval, error) {
	if !isUUID(tenantID) || !isUUID(id) {
		return domain.StepApproval{}, fmt.Errorf("postgres: %w", domain.ErrStepApprovalNotFound)
	}
	return scanStepApproval(s.pool.QueryRow(ctx, `SELECT `+stepApprovalColumns+` FROM workflow.step_approvals WHERE tenant_id = $1 AND id = $2`, tenantID, id))
}

func (s *StepApprovalStore) LatestStepApproval(ctx context.Context, tenantID, executionID, stepID string) (domain.StepApproval, error) {
	if !isUUID(tenantID) || !isUUID(executionID) {
		return domain.StepApproval{}, fmt.Errorf("postgres: %w", domain.ErrStepApprovalNotFound)
	}
	return scanStepApproval(s.pool.QueryRow(ctx, `SELECT `+stepApprovalColumns+` FROM workflow.step_approvals
		WHERE tenant_id = $1 AND execution_id = $2 AND step_id = $3 AND status <> 'superseded'
		ORDER BY created_at DESC LIMIT 1`, tenantID, executionID, stepID))
}

func (s *StepApprovalStore) ListPendingStepApprovals(ctx context.Context, tenantID, projectID, executionID string, limit int) ([]domain.StepApproval, error) {
	if !isUUID(tenantID) || (executionID != "" && !isUUID(executionID)) {
		return nil, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `SELECT `+stepApprovalColumns+` FROM workflow.step_approvals
		WHERE tenant_id = $1 AND status = 'pending' AND ($2 = '' OR project_id = $2) AND ($3 = '' OR execution_id::text = $3)
		ORDER BY created_at DESC LIMIT $4`, tenantID, projectID, executionID, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: list step approvals: %w", err)
	}
	defer rows.Close()
	var out []domain.StepApproval
	for rows.Next() {
		a, err := scanStepApproval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *StepApprovalStore) ResolveStepApproval(ctx context.Context, a domain.StepApproval) (bool, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE workflow.step_approvals SET status = $1, resolved_by = $2, rationale = $3, resolved_at = $4
		WHERE tenant_id = $5 AND id = $6 AND status = 'pending'`, string(a.Status), a.ResolvedBy, a.Rationale, a.ResolvedAt, a.TenantID, a.ID)
	if err != nil {
		return false, fmt.Errorf("postgres: resolve step approval: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (s *StepApprovalStore) MarkStepApprovalSubmitted(ctx context.Context, tenantID, id string, at time.Time) error {
	if _, err := s.pool.Exec(ctx, `UPDATE workflow.step_approvals SET submitted_at = $1 WHERE tenant_id = $2 AND id = $3`, at, tenantID, id); err != nil {
		return fmt.Errorf("postgres: mark step approval submitted: %w", err)
	}
	return nil
}

// isUUID guards uuid columns up front: a malformed id is "not found", not a 22P02 query error.
func isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}
