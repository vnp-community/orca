package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// RegisterDevServerInput mirrors the gRPC request 1:1 by design — see
// architecture/03's note that usecase granularity mirrors today's RPC
// methods so the TS->Go mapping stays traceable.
type RegisterDevServerInput struct {
	Host string
	Mode domain.ConnectionMode
}

// RegisterDevServer adds a new dev host to the registry. TenantID is NOT
// part of the input struct — it's pulled from context (see common/tenant),
// never trusted from the request body, per
// architecture/05-data-architecture.md's tenant-isolation rule.
type RegisterDevServer struct {
	repo DevServerRepository
}

func NewRegisterDevServer(repo DevServerRepository) *RegisterDevServer {
	return &RegisterDevServer{repo: repo}
}

func (uc *RegisterDevServer) Execute(ctx context.Context, in RegisterDevServerInput) (domain.DevServer, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.DevServer{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	devServer, err := domain.NewDevServer(uuid.NewString(), tenantID, in.Host, in.Mode)
	if err != nil {
		return domain.DevServer{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_INVALID_DEV_SERVER", err.Error(), err)
	}

	saved, err := uc.repo.Register(ctx, devServer)
	if err != nil {
		return domain.DevServer{}, apperrors.New(apperrors.KindInternal, "INFRA_REGISTER_FAILED", "failed to register dev server", err)
	}
	return saved, nil
}
