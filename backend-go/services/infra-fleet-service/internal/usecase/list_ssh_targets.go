package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ListSshTargets is 🏠 always-local — a Postgres read against this
// service's own tables, backing ssh.listTargets/ssh.getUserAccount.
type ListSshTargets struct {
	repo SshTargetRepository
}

func NewListSshTargets(repo SshTargetRepository) *ListSshTargets {
	return &ListSshTargets{repo: repo}
}

func (uc *ListSshTargets) Execute(ctx context.Context) ([]domain.SshTarget, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	return uc.repo.List(ctx, tenantID)
}
