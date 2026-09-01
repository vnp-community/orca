package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// RejectDevServerInput carries the optional reason shown to whoever
// registered the dev server — not persisted as its own column (this
// service's DevServer has no audit-log table yet); Reason exists on the
// wire for a future notification/audit pass, not stored today.
type RejectDevServerInput struct {
	DevServerID string
	Reason      string
}

// RejectDevServer marks a dev server rejected — admin-gated. Rejecting is
// NOT enforced anywhere yet (same Phase-1/Phase-2 sequencing CR-DS-006
// documents for approval) — a rejected dev server still works exactly like
// an approved one until CR-DS-007's ListDevServersForUser gate is the
// caller's only path to it (RPCs like ResolveConnection still read
// DevServerRepository.Get/List directly, unaffected by approval_status).
type RejectDevServer struct {
	repo DevServerRepository
}

func NewRejectDevServer(repo DevServerRepository) *RejectDevServer {
	return &RejectDevServer{repo: repo}
}

func (uc *RejectDevServer) Execute(ctx context.Context, in RejectDevServerInput) (domain.DevServer, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.DevServer{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if err := requireAdmin(ctx); err != nil {
		return domain.DevServer{}, err
	}

	ds, err := uc.repo.UpdateApprovalStatus(ctx, tenantID, in.DevServerID, domain.DevServerStatusRejected)
	if err != nil {
		return domain.DevServer{}, apperrors.New(apperrors.KindInternal, "INFRA_REJECT_DEV_SERVER_FAILED", "failed to reject dev server", err)
	}
	return ds, nil
}
