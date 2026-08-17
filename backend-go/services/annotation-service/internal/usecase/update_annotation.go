package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/annotation-service/internal/domain"
)

// UpdateAnnotationInput mirrors the gRPC request 1:1. Author-only edit
// enforcement is deliberately not implemented here — per
// annotation-service.md §9 it's an OPA policy decision at the gateway, not
// an inline usecase check.
type UpdateAnnotationInput struct {
	ID       string
	Content  string
	Resolved bool
}

type UpdateAnnotation struct {
	repo Repository
}

func NewUpdateAnnotation(repo Repository) *UpdateAnnotation {
	return &UpdateAnnotation{repo: repo}
}

func (uc *UpdateAnnotation) Execute(ctx context.Context, in UpdateAnnotationInput) (domain.Annotation, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Annotation{}, apperrors.New(apperrors.KindUnauthenticated, "ANNOTATION_NO_TENANT", "no tenant in request context", err)
	}
	if in.ID == "" {
		return domain.Annotation{}, apperrors.New(apperrors.KindInvalidArgument, "ANNOTATION_ID_REQUIRED", "id is required", nil)
	}
	if in.Content == "" {
		return domain.Annotation{}, apperrors.New(apperrors.KindInvalidArgument, "ANNOTATION_INVALID", domain.ErrEmptyContent.Error(), domain.ErrEmptyContent)
	}

	updated, err := uc.repo.UpdateAnnotation(ctx, tenantID, in.ID, in.Content, in.Resolved)
	if err != nil {
		if errors.Is(err, domain.ErrAnnotationNotFound) {
			return domain.Annotation{}, apperrors.New(apperrors.KindNotFound, "ANNOTATION_NOT_FOUND", "annotation not found", err)
		}
		return domain.Annotation{}, apperrors.New(apperrors.KindInternal, "ANNOTATION_UPDATE_FAILED", "failed to update annotation", err)
	}
	return updated, nil
}
