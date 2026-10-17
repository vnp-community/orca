package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// StepApprovalStore implements usecase.StepApprovalRepository on step_approvals (MySQL/TiDB).
type StepApprovalStore struct {
	db *sql.DB
}

func NewStepApprovalStore(db *sql.DB) *StepApprovalStore { return &StepApprovalStore{db: db} }

var _ usecase.StepApprovalRepository = (*StepApprovalStore)(nil)

const stepApprovalColumns = `id, tenant_id, execution_id, step_id, project_id, kind, subject, decision, decision_hash, status,
	COALESCE(resolved_by, ''), COALESCE(rationale, ''), created_at, resolved_at, submitted_at`

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func scanStepApproval(row rowScanner) (domain.StepApproval, error) {
	var a domain.StepApproval
	var subject, decision []byte
	var status string
	var resolvedAt, submittedAt sql.NullTime
	err := row.Scan(&a.ID, &a.TenantID, &a.ExecutionID, &a.StepID, &a.ProjectID, &a.Kind, &subject, &decision, &a.DecisionHash, &status,
		&a.ResolvedBy, &a.Rationale, &a.CreatedAt, &resolvedAt, &submittedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.StepApproval{}, fmt.Errorf("mysql: %w", domain.ErrStepApprovalNotFound)
	}
	if err != nil {
		return domain.StepApproval{}, fmt.Errorf("mysql: scan step approval: %w", err)
	}
	if resolvedAt.Valid {
		a.ResolvedAt = &resolvedAt.Time
	}
	if submittedAt.Valid {
		a.SubmittedAt = &submittedAt.Time
	}
	a.Subject, a.Decision, a.Status = json.RawMessage(subject), json.RawMessage(decision), domain.StepApprovalStatus(status)
	return a, nil
}

func getByHash(ctx context.Context, q queryRower, a domain.StepApproval) (domain.StepApproval, error) {
	return scanStepApproval(q.QueryRowContext(ctx, `SELECT `+stepApprovalColumns+` FROM step_approvals
		WHERE tenant_id = ? AND execution_id = ? AND step_id = ? AND decision_hash = ?`, a.TenantID, a.ExecutionID, a.StepID, a.DecisionHash))
}

func (s *StepApprovalStore) UpsertPendingStepApproval(ctx context.Context, a domain.StepApproval) (domain.StepApproval, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.StepApproval{}, fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	existing, err := getByHash(ctx, tx, a)
	if err == nil {
		return existing, tx.Commit()
	}
	if !errors.Is(err, domain.ErrStepApprovalNotFound) {
		return domain.StepApproval{}, err
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `UPDATE step_approvals SET status = 'superseded', resolved_at = ?
		WHERE tenant_id = ? AND execution_id = ? AND step_id = ? AND status = 'pending'`, now, a.TenantID, a.ExecutionID, a.StepID); err != nil {
		return domain.StepApproval{}, fmt.Errorf("mysql: supersede step approvals: %w", err)
	}
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO step_approvals
		(id, tenant_id, execution_id, step_id, project_id, kind, subject, decision, decision_hash, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?)`,
		a.ID, a.TenantID, a.ExecutionID, a.StepID, a.ProjectID, a.Kind, []byte(a.Subject), []byte(a.Decision), a.DecisionHash, a.CreatedAt); err != nil {
		return domain.StepApproval{}, fmt.Errorf("mysql: insert step approval: %w", err)
	}
	created, err := getByHash(ctx, tx, a)
	if err != nil {
		return domain.StepApproval{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.StepApproval{}, fmt.Errorf("mysql: commit tx: %w", err)
	}
	return created, nil
}

func (s *StepApprovalStore) GetStepApproval(ctx context.Context, tenantID, id string) (domain.StepApproval, error) {
	return scanStepApproval(s.db.QueryRowContext(ctx, `SELECT `+stepApprovalColumns+` FROM step_approvals WHERE tenant_id = ? AND id = ?`, tenantID, id))
}

func (s *StepApprovalStore) LatestStepApproval(ctx context.Context, tenantID, executionID, stepID string) (domain.StepApproval, error) {
	return scanStepApproval(s.db.QueryRowContext(ctx, `SELECT `+stepApprovalColumns+` FROM step_approvals
		WHERE tenant_id = ? AND execution_id = ? AND step_id = ? AND status <> 'superseded'
		ORDER BY created_at DESC LIMIT 1`, tenantID, executionID, stepID))
}

func (s *StepApprovalStore) ListPendingStepApprovals(ctx context.Context, tenantID, projectID, executionID string, limit int) ([]domain.StepApproval, error) {
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+stepApprovalColumns+` FROM step_approvals
		WHERE tenant_id = ? AND status = 'pending' AND (? = '' OR project_id = ?) AND (? = '' OR execution_id = ?)
		ORDER BY created_at DESC LIMIT ?`, tenantID, projectID, projectID, executionID, executionID, limit)
	if err != nil {
		return nil, fmt.Errorf("mysql: list step approvals: %w", err)
	}
	defer func() { _ = rows.Close() }()
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
	res, err := s.db.ExecContext(ctx, `UPDATE step_approvals SET status = ?, resolved_by = ?, rationale = ?, resolved_at = ?
		WHERE tenant_id = ? AND id = ? AND status = 'pending'`, string(a.Status), a.ResolvedBy, a.Rationale, a.ResolvedAt, a.TenantID, a.ID)
	if err != nil {
		return false, fmt.Errorf("mysql: resolve step approval: %w", err)
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (s *StepApprovalStore) MarkStepApprovalSubmitted(ctx context.Context, tenantID, id string, at time.Time) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE step_approvals SET submitted_at = ? WHERE tenant_id = ? AND id = ?`, at, tenantID, id); err != nil {
		return fmt.Errorf("mysql: mark step approval submitted: %w", err)
	}
	return nil
}
