package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ListRequestTypeHistory struct {
	repo    RequestRepository
	history RequestTypeHistoryRepository
}

func NewListRequestTypeHistory(repo RequestRepository, history RequestTypeHistoryRepository) *ListRequestTypeHistory {
	return &ListRequestTypeHistory{repo: repo, history: history}
}

func (uc *ListRequestTypeHistory) Execute(ctx context.Context, requestID string) ([]domain.RequestTypeChange, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return nil, domain.ErrRequestTenantRequired()
	}
	if _, err := uc.repo.Get(ctx, requestID); err != nil {
		return nil, err
	}
	return uc.history.List(ctx, requestID)
}
