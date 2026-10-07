package mysql

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type PolicyRepository struct {
	*Repository
}

func NewPolicyRepository(r *Repository) *PolicyRepository {
	return &PolicyRepository{Repository: r}
}

var _ usecase.ApprovalPolicyRepository = (*PolicyRepository)(nil)

func (r *PolicyRepository) ListEnabledCandidates(ctx context.Context, tenantID string, subjectType domain.SubjectType, projectID, requestType, size, urgency string) ([]domain.ApprovalPolicy, error) {
	query := `
		SELECT
			id, tenant_id, project_id, subject_type, request_type, size, urgency,
			approvers, allow_requester_approve, due_after_seconds, priority,
			enabled, version, created_at
		FROM approval_policies
		WHERE tenant_id = ? AND subject_type = ? AND enabled = true
		  AND (project_id IS NULL OR project_id = ?)
		  AND (request_type IS NULL OR request_type = ?)
		  AND (size IS NULL OR size = ?)
		  AND (urgency IS NULL OR urgency = ?)
	`
	rows, err := r.exec(ctx).QueryContext(ctx, query, tenantID, string(subjectType), projectID, requestType, size, urgency)
	if err != nil {
		return nil, fmt.Errorf("mysql list policies: %w", err)
	}
	defer rows.Close()

	var list []domain.ApprovalPolicy
	for rows.Next() {
		var p domain.ApprovalPolicy
		var approversJSON []byte
		var dueSecs *int
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.ProjectID, &p.SubjectType, &p.RequestType, &p.Size, &p.Urgency,
			&approversJSON, &p.AllowRequesterApprove, &dueSecs, &p.Priority,
			&p.Enabled, &p.Version, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		if dueSecs != nil {
			d := time.Duration(*dueSecs) * time.Second
			p.DueAfter = &d
		}
		if err := json.Unmarshal(approversJSON, &p.Approvers); err != nil {
			return nil, fmt.Errorf("parse approvers: %w", err)
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *PolicyRepository) Upsert(ctx context.Context, p domain.ApprovalPolicy) error {
	return nil // Stub for now
}

func (r *PolicyRepository) Delete(ctx context.Context, tenantID, id string) error {
	return nil // Stub for now
}

func (r *PolicyRepository) NowDB(ctx context.Context) (time.Time, error) {
	var now time.Time
	var tStr string
	err := r.exec(ctx).QueryRowContext(ctx, "SELECT CURRENT_TIMESTAMP(6)").Scan(&tStr)
	if err == nil {
		now, err = time.Parse("2006-01-02 15:04:05.999999", tStr)
	}
	return now, err
}

type ApproverRepository struct {
	*Repository
}

func NewApproverRepository(r *Repository) *ApproverRepository {
	return &ApproverRepository{Repository: r}
}

var _ usecase.ApprovalApproverRepository = (*ApproverRepository)(nil)

func (r *ApproverRepository) InsertSnapshot(ctx context.Context, approvalID, tenantID string, approvers []domain.Principal) error {
	if len(approvers) == 0 {
		return nil
	}
	query := `INSERT INTO approval_approvers (approval_id, tenant_id, principal_kind, principal_id) VALUES (?, ?, ?, ?)`
	for _, p := range approvers {
		id := p.ID
		if p.Kind == domain.PrincipalKindReporter {
			id = ""
		}
		_, err := r.exec(ctx).ExecContext(ctx, query, approvalID, tenantID, string(p.Kind), id)
		if err != nil {
			return fmt.Errorf("mysql insert approver snapshot: %w", err)
		}
	}
	return nil
}

func (r *ApproverRepository) ListForApproval(ctx context.Context, tenantID, approvalID string) ([]domain.Principal, error) {
	query := `SELECT principal_kind, principal_id FROM approval_approvers WHERE tenant_id = ? AND approval_id = ?`
	rows, err := r.exec(ctx).QueryContext(ctx, query, tenantID, approvalID)
	if err != nil {
		return nil, fmt.Errorf("mysql list approvers: %w", err)
	}
	defer rows.Close()

	var list []domain.Principal
	for rows.Next() {
		var p domain.Principal
		if err := rows.Scan(&p.Kind, &p.ID); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}
