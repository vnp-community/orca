package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/annotation-service/internal/domain"
)

type ListAnnotationsInput struct {
	RepoID    string
	FilePath  string // optional filter
	PageToken string
	PageSize  int32
}

type ListAnnotationsOutput struct {
	Annotations   []domain.Annotation
	NextPageToken string
}

type ListAnnotations struct {
	repo Repository
}

func NewListAnnotations(repo Repository) *ListAnnotations {
	return &ListAnnotations{repo: repo}
}

func (uc *ListAnnotations) Execute(ctx context.Context, in ListAnnotationsInput) (ListAnnotationsOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ListAnnotationsOutput{}, apperrors.New(apperrors.KindUnauthenticated, "ANNOTATION_NO_TENANT", "no tenant in request context", err)
	}
	if in.RepoID == "" {
		return ListAnnotationsOutput{}, apperrors.New(apperrors.KindInvalidArgument, "ANNOTATION_REPO_ID_REQUIRED", "repo_id is required", nil)
	}

	pageSize := in.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}

	annotations, next, err := uc.repo.ListAnnotations(ctx, tenantID, in.RepoID, in.FilePath, in.PageToken, pageSize)
	if err != nil {
		return ListAnnotationsOutput{}, apperrors.New(apperrors.KindInternal, "ANNOTATION_LIST_FAILED", "failed to list annotations", err)
	}
	return ListAnnotationsOutput{Annotations: annotations, NextPageToken: next}, nil
}
