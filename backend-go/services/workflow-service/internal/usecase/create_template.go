package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// CreateTemplateInput mirrors the gRPC request 1:1 by design — see
// architecture/03's note that usecase granularity mirrors today's RPC
// methods so the TS->Go mapping stays traceable.
//
// Note: CreateTemplateRequest's proto shape carries a tenant_id field, but
// (matching usage-service's convention and
// architecture/05-data-architecture.md's "tenant scoping is never trusted
// from the request body" rule) the tenant actually used here comes from
// the validated request context (common/tenant, populated by
// common/grpcmw's interceptor), not from that field.
type CreateTemplateInput struct {
	Name    string
	DAGJSON string
	Scope   string
}

// CreateTemplate is workflow-service's template authoring path.
type CreateTemplate struct {
	repo TemplateRepository
}

func NewCreateTemplate(repo TemplateRepository) *CreateTemplate {
	return &CreateTemplate{repo: repo}
}

func (uc *CreateTemplate) Execute(ctx context.Context, in CreateTemplateInput) (domain.WorkflowTemplate, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}

	tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, in.Name, in.DAGJSON, domain.Scope(in.Scope))
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_TEMPLATE", err.Error(), err)
	}

	if err := uc.repo.CreateTemplate(ctx, tmpl); err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_SAVE_FAILED", "failed to persist workflow template", err)
	}

	return tmpl, nil
}
