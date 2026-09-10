package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// DeleteSshTarget removes an SshTarget by ID, scoped to the caller's
// tenant. Used as a compensating action by BulkProvisionFleet when
// RegisterDevServer fails after CreateSshTarget already committed —
// see CR-FLEET-001's "Rollback semantics" (no cross-usecase DB
// transaction exists, so orphaned rows are cleaned up explicitly).
type DeleteSshTarget struct {
	repo SshTargetRepository
}

func NewDeleteSshTarget(repo SshTargetRepository) *DeleteSshTarget {
	return &DeleteSshTarget{repo: repo}
}

func (uc *DeleteSshTarget) Execute(ctx context.Context, id string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if err := uc.repo.Delete(ctx, tenantID, id); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_DELETE_SSH_TARGET_FAILED", "failed to delete ssh target", err)
	}
	return nil
}
