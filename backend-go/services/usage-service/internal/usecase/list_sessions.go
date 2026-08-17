package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/usage-service/internal/domain"
)

type ListSessionsInput struct {
	UserID    string
	PageToken string
	PageSize  int32
}

type ListSessionsOutput struct {
	Sessions      []domain.UsageSession
	NextPageToken string
}

type ListSessions struct {
	repo Repository
}

func NewListSessions(repo Repository) *ListSessions {
	return &ListSessions{repo: repo}
}

func (uc *ListSessions) Execute(ctx context.Context, in ListSessionsInput) (ListSessionsOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ListSessionsOutput{}, apperrors.New(apperrors.KindUnauthenticated, "USAGE_NO_TENANT", "no tenant in request context", err)
	}

	pageSize := in.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}

	sessions, next, err := uc.repo.ListSessions(ctx, tenantID, in.UserID, in.PageToken, pageSize)
	if err != nil {
		return ListSessionsOutput{}, apperrors.New(apperrors.KindInternal, "USAGE_LIST_FAILED", "failed to list usage sessions", err)
	}
	return ListSessionsOutput{Sessions: sessions, NextPageToken: next}, nil
}
