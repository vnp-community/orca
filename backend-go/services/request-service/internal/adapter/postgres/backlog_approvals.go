package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
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
	idx := 2
	for _, id := range requestIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
		args = append(args, id)
		idx++
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, request_id, subject_type, subject_id, stage, status, requested_by, due_at, version, created_at, updated_at, subject_digest, self_approval_allowed
		FROM request.approvals
		WHERE tenant_id = $1 AND request_id IN (%s) AND subject_type IN ('plan', 'task_list', 'phase', 'pre_deploy')
	`, strings.Join(placeholders, ", "))

	rows, err := r.exec(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres ListGateApprovals: %w", err)
	}
	defer rows.(pgx.Rows).Close()

	var list []domain.Approval
	for rows.(pgx.Rows).Next() {
		var a domain.Approval
		if err := rows.(pgx.Rows).Scan(
			&a.ID, &a.TenantID, &a.RequestID, &a.SubjectType, &a.SubjectID, &a.Stage, &a.Status, &a.RequestedBy,
			&a.DueAt, &a.Version, &a.CreatedAt, &a.UpdatedAt, &a.SubjectDigest, &a.SelfApprovalAllowed,
		); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.(pgx.Rows).Err()
}
