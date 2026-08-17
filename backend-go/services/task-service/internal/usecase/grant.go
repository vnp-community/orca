package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type GrantInput struct {
	TaskID    string
	SubjectID string
	Level     domain.GrantLevel
	ApplyTree bool
}

// Grant is task-service's grant-mutation usecase. Per task-service.md §9,
// Grant/RevokeGrant should emit structured audit events — not wired in this
// scaffold (no EventPublisher port defined here); see this service's
// README.
type Grant struct {
	grants GrantRepository
}

func NewGrant(grants GrantRepository) *Grant {
	return &Grant{grants: grants}
}

func (uc *Grant) Execute(ctx context.Context, in GrantInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if in.TaskID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "TASK_GRANT_INVALID", "task_id is required", nil)
	}
	if in.SubjectID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "TASK_GRANT_INVALID", "subject_id is required", nil)
	}
	if !in.Level.Valid() {
		return apperrors.New(apperrors.KindInvalidArgument, "TASK_GRANT_INVALID", "level is not a recognized grant level", nil)
	}

	grant := domain.Grant{TaskID: in.TaskID, SubjectID: in.SubjectID, Level: in.Level, ApplyTree: in.ApplyTree}
	if err := uc.grants.Grant(ctx, tenantID, grant); err != nil {
		return apperrors.New(apperrors.KindInternal, "TASK_GRANT_FAILED", "failed to persist grant", err)
	}
	return nil
}
