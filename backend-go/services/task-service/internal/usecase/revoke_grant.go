package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type RevokeGrantInput struct {
	TaskID    string
	SubjectID string
	Level     domain.GrantLevel
}

// RevokeGrant is task-service's grant-removal usecase, deleting by the
// (task_id, subject_id, level) composite key — see
// usecase.GrantRepository.Revoke's doc comment. Idempotent: revoking a
// grant that doesn't (or no longer) exist is not an error.
type RevokeGrant struct {
	grants GrantRepository
}

func NewRevokeGrant(grants GrantRepository) *RevokeGrant {
	return &RevokeGrant{grants: grants}
}

func (uc *RevokeGrant) Execute(ctx context.Context, in RevokeGrantInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if err := uc.grants.Revoke(ctx, tenantID, in.TaskID, in.SubjectID, in.Level); err != nil {
		return apperrors.New(apperrors.KindInternal, "TASK_REVOKE_GRANT_FAILED", "failed to revoke grant", err)
	}
	return nil
}
