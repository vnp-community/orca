package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

type CloneTemplateInput struct {
	SourceTemplateID string
	NewName          string
	Scope            string
}

// CloneTemplate produces a fresh, parent-less copy of an effective
// (post-inheritance) template — the clone's dag_json is the SOURCE's
// fully-resolved DAG (parent chain already folded in by ResolveTemplate),
// not just the source row's own raw dag_json, so a clone of a
// steps-inheriting-from-parent template still has real steps. The clone
// has NO ParentTemplateID — this is a deliberate severing of inheritance:
// a clone is a fresh root, never a child of its source, so future edits to
// the source never propagate to the clone (or vice versa).
type CloneTemplate struct {
	resolveTemplate *ResolveTemplate
	templates       TemplateRepository
}

func NewCloneTemplate(resolveTemplate *ResolveTemplate, templates TemplateRepository) *CloneTemplate {
	return &CloneTemplate{resolveTemplate: resolveTemplate, templates: templates}
}

func (uc *CloneTemplate) Execute(ctx context.Context, in CloneTemplateInput) (domain.WorkflowTemplate, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}

	resolved, err := uc.resolveTemplate.Execute(ctx, ResolveTemplateInput{TemplateID: in.SourceTemplateID})
	if err != nil {
		return domain.WorkflowTemplate{}, err // ResolveTemplate already wraps NotFound/etc. in apperrors
	}

	scope := domain.Scope(in.Scope)
	clone, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, in.NewName, resolved.Template.DAGJSON, scope, "")
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_TEMPLATE", err.Error(), err)
	}

	if err := uc.templates.CreateTemplate(ctx, clone); err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_CLONE_TEMPLATE_FAILED", "failed to create cloned template", err)
	}
	return clone, nil
}
