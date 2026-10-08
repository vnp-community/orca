package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type GetRequest struct {
	repo RequestRepository
}

func NewGetRequest(repo RequestRepository) *GetRequest {
	return &GetRequest{repo: repo}
}

func (uc *GetRequest) Execute(ctx context.Context, id string) (domain.Request, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Request{}, domain.ErrRequestTenantRequired()
	}
	// Malformed ids look the same as missing ones so callers cannot probe id formats.
	if _, err := uuid.Parse(id); err != nil {
		return domain.Request{}, domain.ErrRequestNotFound(id)
	}
	return uc.repo.Get(ctx, id)
}
