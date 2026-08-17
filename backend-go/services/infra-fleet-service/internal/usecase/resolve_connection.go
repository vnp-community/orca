package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ResolveConnectionOutput is the resolved dispatch record: whether a dev
// server owns the given connectionId (Connected) and, if so, which one.
type ResolveConnectionOutput struct {
	Connected bool
	DevServer domain.DevServer
}

// ResolveConnection is THE core coordination/execution dispatch primitive of
// this service — see specs/backend-go/services/infra-fleet-service.md §7.
// git-gateway-service calls this on every git.* dispatch to decide
// local-exec vs. relay; project-service calls it to validate a dev-server
// binding; any connectionId-bound feature in the system reduces to this call.
type ResolveConnection struct {
	resolver ConnectionResolver
}

func NewResolveConnection(resolver ConnectionResolver) *ResolveConnection {
	return &ResolveConnection{resolver: resolver}
}

func (uc *ResolveConnection) Execute(ctx context.Context, connectionID string) (ResolveConnectionOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ResolveConnectionOutput{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	// No connectionId at all is not an error — it's the caller's own signal
	// that there's nothing to resolve (a connectionless, local-only worktree
	// or session). Short-circuit before the repository round-trip.
	if connectionID == "" {
		return ResolveConnectionOutput{Connected: false}, nil
	}

	connected, devServer, err := uc.resolver.ResolveConnection(ctx, tenantID, connectionID)
	if err != nil {
		return ResolveConnectionOutput{}, apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", err)
	}
	return ResolveConnectionOutput{Connected: connected, DevServer: devServer}, nil
}
