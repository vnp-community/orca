package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// loadReadableRequest applies the same read rule as GetRequest (tenant scope, malformed ids look missing).
func loadReadableRequest(ctx context.Context, repo RequestRepository, id string) (domain.Request, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Request{}, domain.ErrRequestTenantRequired()
	}
	if _, err := uuid.Parse(id); err != nil {
		return domain.Request{}, domain.ErrRequestNotFound(id)
	}
	return repo.Get(ctx, id)
}

// callerIsAdmin and callerID read the identity the gRPC interceptor put in ctx; an empty role fails closed.
func callerIsAdmin(ctx context.Context) bool {
	role, _ := tenant.Role(ctx)
	return role == "admin"
}

func callerID(ctx context.Context) string {
	id, _ := tenant.UserID(ctx)
	return id
}

// callerIsMachine is true for agent and system actors; they never answer, confirm or choose for a person.
func callerIsMachine(ctx context.Context) bool {
	return tenant.ActorType(ctx) != tenant.ActorUser
}

func errRequestForbidden(reason string) error {
	return apperrors.New(apperrors.KindPermissionDenied, "REQUEST_FORBIDDEN", reason, nil)
}

// canWriteRequest is the interim write rule (reporter or admin) until README v6 section 8 settles request-level permissions.
func canWriteRequest(ctx context.Context, r domain.Request) bool {
	return callerIsAdmin(ctx) || (callerID(ctx) != "" && callerID(ctx) == r.ReporterID)
}

func tenantOf(ctx context.Context) (string, bool) { return tenant.TenantID(ctx) }
