package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/annotation-service/internal/domain"
)

// DeleteAnnotationInput mirrors the gRPC request 1:1. Author-only delete
// enforcement is deliberately not implemented here — per
// annotation-service.md §9 it's an OPA policy decision at the gateway, not
// an inline usecase check.
type DeleteAnnotationInput struct {
	ID string
}

type DeleteAnnotation struct {
	repo Repository
}

func NewDeleteAnnotation(repo Repository) *DeleteAnnotation {
	return &DeleteAnnotation{repo: repo}
}

func (uc *DeleteAnnotation) Execute(ctx context.Context, in DeleteAnnotationInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "ANNOTATION_NO_TENANT", "no tenant in request context", err)
	}
	if in.ID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "ANNOTATION_ID_REQUIRED", "id is required", nil)
	}

	if err := uc.repo.DeleteAnnotation(ctx, tenantID, in.ID); err != nil {
		if errors.Is(err, domain.ErrAnnotationNotFound) {
			return apperrors.New(apperrors.KindNotFound, "ANNOTATION_NOT_FOUND", "annotation not found", err)
		}
		return apperrors.New(apperrors.KindInternal, "ANNOTATION_DELETE_FAILED", "failed to delete annotation", err)
	}
	return nil
}
