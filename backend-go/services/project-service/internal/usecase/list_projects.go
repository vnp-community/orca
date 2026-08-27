package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type ListProjectsInput struct {
	PageToken string
	PageSize  int32
}

type ListProjectsOutput struct {
	Projects      []domain.Project
	NextPageToken string
}

type ListProjects struct {
	repo ProjectRepository
}

func NewListProjects(repo ProjectRepository) *ListProjects {
	return &ListProjects{repo: repo}
}

func (uc *ListProjects) Execute(ctx context.Context, in ListProjectsInput) (ListProjectsOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ListProjectsOutput{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	pageSize := in.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}

	projects, next, err := uc.repo.List(ctx, tenantID, in.PageToken, pageSize)
	if err != nil {
		return ListProjectsOutput{}, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_FAILED", "failed to list projects", err)
	}
	return ListProjectsOutput{Projects: projects, NextPageToken: next}, nil
}
