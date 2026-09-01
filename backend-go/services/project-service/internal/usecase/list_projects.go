package usecase

import (
	"context"

	"github.com/google/uuid"

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

// Execute lists only projects the caller is a member of — NOT every
// project in the tenant. Found live during the "one private default
// project per user" pass (Phase 4b): the original tenant-only filter
// meant `project.list` (this RPC's wscompat channel) handed every tenant
// member the full tenant-wide project catalog, defeating the private-by-
// default design before it existed — ListMembers/GetMembership already
// gate per-project access correctly, but nothing gated the LIST itself.
func (uc *ListProjects) Execute(ctx context.Context, in ListProjectsInput) (ListProjectsOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ListProjectsOutput{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return ListProjectsOutput{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_USER", "no user in request context", nil)
	}

	if in.PageToken != "" {
		if _, err := uuid.Parse(in.PageToken); err != nil {
			return ListProjectsOutput{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID_PAGE_TOKEN", "page_token must be empty or a valid cursor", err)
		}
	}

	pageSize := in.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}

	projects, next, err := uc.repo.List(ctx, tenantID, userID, in.PageToken, pageSize)
	if err != nil {
		return ListProjectsOutput{}, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_FAILED", "failed to list projects", err)
	}
	return ListProjectsOutput{Projects: projects, NextPageToken: next}, nil
}
