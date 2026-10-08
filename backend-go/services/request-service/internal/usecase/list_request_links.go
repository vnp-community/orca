package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ListRequestLinks struct {
	repo  RequestRepository
	links RequestLinkRepository
}

func NewListRequestLinks(repo RequestRepository, links RequestLinkRepository) *ListRequestLinks {
	return &ListRequestLinks{repo: repo, links: links}
}

type RequestLinksView struct {
	Parents  []domain.RequestLink
	Children []domain.RequestLink
}

func (uc *ListRequestLinks) Execute(ctx context.Context, requestID string) (RequestLinksView, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return RequestLinksView{}, domain.ErrRequestTenantRequired()
	}
	if _, err := uc.repo.Get(ctx, requestID); err != nil {
		return RequestLinksView{}, err
	}
	parents, err := uc.links.ListParents(ctx, requestID)
	if err != nil {
		return RequestLinksView{}, err
	}
	children, err := uc.links.ListChildren(ctx, requestID)
	if err != nil {
		return RequestLinksView{}, err
	}
	return RequestLinksView{Parents: parents, Children: children}, nil
}
