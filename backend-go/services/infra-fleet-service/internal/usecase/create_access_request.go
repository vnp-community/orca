package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// CreateAccessRequestInput's GranteeKind/GranteeID are supplied by the
// caller (api-gateway) — see the proto CreateAccessRequestRequest message's
// doc comment.
type CreateAccessRequestInput struct {
	DevServerGroupID string
	Message          string
	GranteeKind      domain.GranteeKind
	GranteeID        string
	NowUnixMs        int64
}

// CreateAccessRequest is NOT admin-gated — any authenticated tenant user
// may ask for access. See docs/crs/v2/dev-server/
// CR-DS-008-first-login-department-gate-and-access-request.md §2.3.
type CreateAccessRequest struct {
	repo DevServerAccessRequestRepository
}

func NewCreateAccessRequest(repo DevServerAccessRequestRepository) *CreateAccessRequest {
	return &CreateAccessRequest{repo: repo}
}

func (uc *CreateAccessRequest) Execute(ctx context.Context, in CreateAccessRequestInput) (domain.DevServerAccessRequest, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.DevServerAccessRequest{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.DevServerAccessRequest{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_USER", "no user in request context", nil)
	}

	req, err := domain.NewDevServerAccessRequest(uuid.NewString(), tenantID, userID, in.DevServerGroupID, in.Message, in.GranteeKind, in.GranteeID, in.NowUnixMs)
	if err != nil {
		return domain.DevServerAccessRequest{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_INVALID_ACCESS_REQUEST", err.Error(), err)
	}

	saved, err := uc.repo.Create(ctx, req)
	if err != nil {
		return domain.DevServerAccessRequest{}, apperrors.New(apperrors.KindInternal, "INFRA_CREATE_ACCESS_REQUEST_FAILED", "failed to create access request", err)
	}
	return saved, nil
}
