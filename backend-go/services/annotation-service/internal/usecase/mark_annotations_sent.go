package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/annotation-service/internal/domain"
)

type MarkAnnotationsSentInput struct {
	IDs []string
}

// MarkAnnotationsSent bulk-transitions a set of annotations to sent-to-agent
// state — used exclusively by api-gateway's annotation.sendToAgent
// composition (SOL-CR-03), never a standalone user action.
type MarkAnnotationsSent struct {
	repo Repository
}

func NewMarkAnnotationsSent(repo Repository) *MarkAnnotationsSent {
	return &MarkAnnotationsSent{repo: repo}
}

func (uc *MarkAnnotationsSent) Execute(ctx context.Context, in MarkAnnotationsSentInput) ([]domain.Annotation, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ANNOTATION_NO_TENANT", "no tenant in request context", err)
	}
	if len(in.IDs) == 0 {
		return nil, nil
	}
	updated, err := uc.repo.MarkSent(ctx, tenantID, in.IDs, time.Now().UTC())
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "ANNOTATION_MARK_SENT_FAILED", "failed to mark annotations sent", err)
	}
	return updated, nil
}
