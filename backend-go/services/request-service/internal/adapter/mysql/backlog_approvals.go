package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type ApprovalGateReader struct {
	*Repository
}

func NewApprovalGateReader(r *Repository) *ApprovalGateReader {
	return &ApprovalGateReader{Repository: r}
}

var _ usecase.ApprovalGateReader = (*ApprovalGateReader)(nil)

func (r *ApprovalGateReader) ListGateApprovals(ctx context.Context, tenantID string, requestIDs []string) ([]domain.Approval, error) {
	if len(requestIDs) == 0 {
		return nil, nil
	}

	var placeholders []string
	args := []any{tenantID}
	for _, id := range requestIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, request_id, subject_type, subject_id, stage, status, requested_by, due_at, version, created_at, updated_at, subject_digest, self_approval_allowed
		FROM approvals
		WHERE tenant_id = ? AND request_id IN (%s) AND subject_type IN ('plan', 'task_list', 'phase', 'pre_deploy')
	`, strings.Join(placeholders, ", "))

	rows, err := r.exec(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql ListGateApprovals: %w", err)
	}
	defer rows.Close()

	var list []domain.Approval
	for rows.Next() {
		var a domain.Approval
		if err := rows.Scan(
			&a.ID, &a.TenantID, &a.RequestID, &a.SubjectType, &a.SubjectID, &a.Stage, &a.Status, &a.RequestedBy,
			&a.DueAt, &a.Version, &a.CreatedAt, &a.UpdatedAt, &a.SubjectDigest, &a.SelfApprovalAllowed,
		); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}
