package usecase

import (
	"context"
	"errors"

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
	// ParentTemplateID is optional — empty means this template is a root
	// of its own inheritance chain. See domain.WorkflowTemplate's doc
	// comment and usecase.ResolveTemplate.
	ParentTemplateID string
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
	ownerID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_USER", "no user in request context", nil)
	}

	tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, in.Name, in.DAGJSON, domain.Scope(in.Scope), in.ParentTemplateID, ownerID)
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_TEMPLATE", err.Error(), err)
	}

	// Explicit existence check (rather than relying on the FK constraint
	// alone) so an unknown parent surfaces as a clean
	// KindFailedPrecondition, matching task-service's CreateTask/ParentID
	// convention — see that usecase for the identical pattern.
	if tmpl.ParentTemplateID != "" {
		if _, err := uc.repo.GetTemplate(ctx, tenantID, tmpl.ParentTemplateID); err != nil {
			if errors.Is(err, domain.ErrTemplateNotFound) {
				return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_PARENT_TEMPLATE_NOT_FOUND", "parent template does not exist", err)
			}
			return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_FETCH_FAILED", "failed to fetch parent template", err)
		}
	}

	if err := uc.repo.CreateTemplate(ctx, tmpl); err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_SAVE_FAILED", "failed to persist workflow template", err)
	}

	return tmpl, nil
}
