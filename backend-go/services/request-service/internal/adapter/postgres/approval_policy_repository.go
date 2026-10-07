package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type PolicyRepository struct {
	*Repository // embeds exec
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
		FROM request.approval_policies
		WHERE tenant_id = $1 AND subject_type = $2 AND enabled = true
		  AND (project_id IS NULL OR project_id = $3)
		  AND (request_type IS NULL OR request_type = $4)
		  AND (size IS NULL OR size = $5)
		  AND (urgency IS NULL OR urgency = $6)
	`
	rows, err := r.exec(ctx).Query(ctx, query, tenantID, string(subjectType), projectID, requestType, size, urgency)
	if err != nil {
		return nil, fmt.Errorf("postgres list policies: %w", err)
	}
	defer rows.(pgx.Rows).Close()

	var list []domain.ApprovalPolicy
	for rows.(pgx.Rows).Next() {
		var p domain.ApprovalPolicy
		var approversJSON []byte
		var dueSecs *int
		if err := rows.(pgx.Rows).Scan(
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
	return list, rows.(pgx.Rows).Err()
}

func (r *PolicyRepository) Upsert(ctx context.Context, p domain.ApprovalPolicy) error {
	return nil // Stub for now
}

func (r *PolicyRepository) Delete(ctx context.Context, tenantID, id string) error {
	return nil // Stub for now
}

func (r *PolicyRepository) NowDB(ctx context.Context) (time.Time, error) {
	var now time.Time
	err := r.exec(ctx).QueryRow(ctx, "SELECT now()").Scan(&now)
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
	query := `INSERT INTO request.approval_approvers (approval_id, tenant_id, principal_kind, principal_id) VALUES ($1, $2, $3, $4)`
	for _, p := range approvers {
		_, err := r.exec(ctx).Exec(ctx, query, approvalID, tenantID, string(p.Kind), p.ID)
		if err != nil {
			return fmt.Errorf("postgres insert approver snapshot: %w", err)
		}
	}
	return nil
}

func (r *ApproverRepository) ListForApproval(ctx context.Context, tenantID, approvalID string) ([]domain.Principal, error) {
	query := `SELECT principal_kind, principal_id FROM request.approval_approvers WHERE tenant_id = $1 AND approval_id = $2`
	rows, err := r.exec(ctx).Query(ctx, query, tenantID, approvalID)
	if err != nil {
		return nil, fmt.Errorf("postgres list approvers: %w", err)
	}
	defer rows.(pgx.Rows).Close()

	var list []domain.Principal
	for rows.(pgx.Rows).Next() {
		var p domain.Principal
		if err := rows.(pgx.Rows).Scan(&p.Kind, &p.ID); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.(pgx.Rows).Err()
}
