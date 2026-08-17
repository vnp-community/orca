package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/annotation-service/internal/domain"
)

// CreateAnnotationInput mirrors the gRPC request 1:1 by design — see
// architecture/03's note that usecase granularity mirrors today's RPC
// methods so the TS->Go mapping stays traceable.
type CreateAnnotationInput struct {
	RepoID   string
	FilePath string
	Line     int32
	Ref      string
	Content  string
}

// CreateAnnotation is annotation-service's core write path. TenantID/
// AuthorID are NOT part of the input struct — they're pulled from context
// (see common/tenant), never trusted from the request body, per
// architecture/05-data-architecture.md's tenant-isolation rule.
type CreateAnnotation struct {
	repo Repository
}

func NewCreateAnnotation(repo Repository) *CreateAnnotation {
	return &CreateAnnotation{repo: repo}
}

func (uc *CreateAnnotation) Execute(ctx context.Context, in CreateAnnotationInput) (domain.Annotation, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Annotation{}, apperrors.New(apperrors.KindUnauthenticated, "ANNOTATION_NO_TENANT", "no tenant in request context", err)
	}
	authorID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.Annotation{}, apperrors.New(apperrors.KindUnauthenticated, "ANNOTATION_NO_USER", "no user in request context", nil)
	}

	anchor, err := domain.NewAnchor(in.RepoID, in.FilePath, in.Line, in.Ref)
	if err != nil {
		return domain.Annotation{}, apperrors.New(apperrors.KindInvalidArgument, "ANNOTATION_INVALID_ANCHOR", err.Error(), err)
	}

	now := time.Now().UTC()
	annotation, err := domain.NewAnnotation(uuid.NewString(), tenantID, authorID, anchor, in.Content, false, now, now)
	if err != nil {
		return domain.Annotation{}, apperrors.New(apperrors.KindInvalidArgument, "ANNOTATION_INVALID", err.Error(), err)
	}

	created, err := uc.repo.CreateAnnotation(ctx, annotation)
	if err != nil {
		return domain.Annotation{}, apperrors.New(apperrors.KindInternal, "ANNOTATION_CREATE_FAILED", "failed to persist annotation", err)
	}
	return created, nil
}
