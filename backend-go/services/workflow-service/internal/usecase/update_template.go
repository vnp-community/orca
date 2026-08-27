package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

type UpdateTemplateInput struct {
	ID, Name, DAGJSON, ParentTemplateID string
	Scope                               domain.Scope
	ExpectedVersion                     int32
}

type UpdateTemplate struct {
	templates TemplateRepository
}

func NewUpdateTemplate(templates TemplateRepository) *UpdateTemplate {
	return &UpdateTemplate{templates: templates}
}

func (uc *UpdateTemplate) Execute(ctx context.Context, in UpdateTemplateInput) (domain.WorkflowTemplate, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}
	existing, err := uc.templates.GetTemplate(ctx, tenantID, in.ID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_TEMPLATE_NOT_FOUND", "template does not exist", nil)
		}
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_LOOKUP_FAILED", "failed to look up template", err)
	}

	// OwnerID is authoring provenance, not editable via update — it stays
	// pinned to whoever originally created the template regardless of who is
	// updating it now.
	next, err := domain.NewWorkflowTemplate(in.ID, tenantID, in.Name, in.DAGJSON, in.Scope, in.ParentTemplateID, existing.OwnerID)
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_TEMPLATE", err.Error(), err)
	}

	// Cycle re-validation — template.go's ErrTemplateSelfParent doc comment
	// used to reason this was unreachable because no UpdateTemplate existed
	// to rewire a parent after creation. That premise no longer holds: walk
	// the NEW parent's own chain and reject if in.ID appears in it (a
	// multi-hop cycle), not just the direct self-parent case
	// NewWorkflowTemplate already checks.
	if in.ParentTemplateID != "" {
		chain, err := uc.templates.ResolveChain(ctx, tenantID, in.ParentTemplateID, maxTemplateChainDepth)
		if err != nil {
			return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_CHAIN_FAILED", "failed to resolve parent chain", err)
		}
		for _, ancestor := range chain {
			if ancestor.ID == in.ID {
				return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_TEMPLATE_CYCLE", "update would create a cyclic parent chain", nil)
			}
		}
	}

	updated, err := uc.templates.Update(ctx, next, in.ExpectedVersion)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateVersionConflict) {
			return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_TEMPLATE_VERSION_CONFLICT", "template was modified by another request", err)
		}
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_UPDATE_TEMPLATE_FAILED", "failed to update template", err)
	}
	// No HasActiveExecutions-style guard needed — DefinitionSnapshot
	// freezes at Execute time (workflow-service.md §4), so this update can
	// never retroactively change a running execution's behavior.
	return updated, nil
}
