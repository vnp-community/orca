package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type ResolveAccessRequestInput struct {
	RequestID string
	Approve   bool // false = reject
}

type ResolveAccessRequestOutput struct {
	Request domain.DevServerAccessRequest
	// Grant is the zero value when Approve was false — a rejection creates
	// no grant.
	Grant domain.DevServerGroupGrant
}

// ResolveAccessRequest admin-gated. Approving creates a
// DevServerGroupGrant for exactly the (kind, id) the request captured at
// creation time (see DevServerAccessRequest.GranteeKind's doc comment) —
// never re-derives the requester's current department/team, so a
// department change after the request was filed doesn't retroactively
// change what gets granted.
type ResolveAccessRequest struct {
	requests DevServerAccessRequestRepository
	grants   DevServerGroupGrantRepository
}

func NewResolveAccessRequest(requests DevServerAccessRequestRepository, grants DevServerGroupGrantRepository) *ResolveAccessRequest {
	return &ResolveAccessRequest{requests: requests, grants: grants}
}

func (uc *ResolveAccessRequest) Execute(ctx context.Context, in ResolveAccessRequestInput) (ResolveAccessRequestOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ResolveAccessRequestOutput{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if err := requireAdmin(ctx); err != nil {
		return ResolveAccessRequestOutput{}, err
	}

	req, err := uc.requests.Get(ctx, tenantID, in.RequestID)
	if err != nil {
		return ResolveAccessRequestOutput{}, apperrors.New(apperrors.KindInternal, "INFRA_GET_ACCESS_REQUEST_FAILED", "failed to look up access request", err)
	}
	// Why: resolving twice must never double-grant, and a rejected request
	// re-approved later would silently grant access nobody re-reviewed —
	// require the caller to file a new request instead.
	if req.Status != domain.AccessRequestStatusPending {
		return ResolveAccessRequestOutput{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_ACCESS_REQUEST_ALREADY_RESOLVED", "access request is already resolved", nil)
	}

	newStatus := domain.AccessRequestStatusRejected
	if in.Approve {
		newStatus = domain.AccessRequestStatusApproved
	}
	updated, err := uc.requests.UpdateStatus(ctx, tenantID, in.RequestID, newStatus)
	if err != nil {
		return ResolveAccessRequestOutput{}, apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_ACCESS_REQUEST_FAILED", "failed to resolve access request", err)
	}

	if !in.Approve {
		return ResolveAccessRequestOutput{Request: updated}, nil
	}

	grant, err := domain.NewDevServerGroupGrant(uuid.NewString(), tenantID, req.DevServerGroupID, req.GranteeKind, req.GranteeID)
	if err != nil {
		return ResolveAccessRequestOutput{}, apperrors.New(apperrors.KindInternal, "INFRA_INVALID_GRANT", "approved request produced an invalid grant", err)
	}
	saved, err := uc.grants.Create(ctx, grant)
	if err != nil {
		return ResolveAccessRequestOutput{}, apperrors.New(apperrors.KindInternal, "INFRA_GRANT_FAILED", "failed to create grant for approved request", err)
	}
	return ResolveAccessRequestOutput{Request: updated, Grant: saved}, nil
}
