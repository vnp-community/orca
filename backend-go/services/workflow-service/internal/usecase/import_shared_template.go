package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// ImportSharedTemplate requires a real, authenticated, tenant-scoped
// caller (unlike PreviewSharedTemplate). Reuses ResolveTemplate's
// resolution logic — a personal-scope, no-parent copy of the RESOLVED
// (post-inheritance) source, same shape CloneTemplate builds for a
// same-tenant clone — since the only difference from CloneTemplate is that
// the SOURCE may belong to a different tenant, so the lookup goes through
// GetByShareToken rather than a tenant-scoped GetTemplate, and the chain
// resolve goes through ResolveTemplate.Resolve (the source's OWN tenant_id)
// rather than Execute (the caller's ctx tenant_id).
type ImportSharedTemplate struct {
	templates TemplateRepository
	resolve   *ResolveTemplate
}

func NewImportSharedTemplate(templates TemplateRepository, resolve *ResolveTemplate) *ImportSharedTemplate {
	return &ImportSharedTemplate{templates: templates, resolve: resolve}
}

func (uc *ImportSharedTemplate) Execute(ctx context.Context, shareToken string) (domain.WorkflowTemplate, error) {
	if shareToken == "" {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_SHARE_TOKEN_REQUIRED", "share_token is required", nil)
	}
	source, err := uc.templates.GetByShareToken(ctx, shareToken)
	if err != nil || source.Visibility != domain.VisibilityPublic {
		// Same identical error PreviewSharedTemplate returns for the same
		// two cases (unknown token / since-unpublished) — see that
		// usecase's doc comment for why the two aren't distinguished.
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_SHARE_LINK_INVALID", "share link is invalid or expired", nil)
	}

	// Cross-tenant resolve: ResolveTemplate.Resolve called directly with
	// the SOURCE template's own tenant_id (read off the row above), NOT
	// through the ctx-tenant-scoped Execute — a deliberate usecase-boundary
	// decision (see ResolveTemplate.Resolve's doc comment), not a
	// workaround.
	resolved, err := uc.resolve.Resolve(ctx, source.TenantID, source.ID)
	if err != nil {
		return domain.WorkflowTemplate{}, err
	}

	importerTenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}
	importerUserID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_USER", "no user in request context", nil)
	}

	tmpl, err := domain.NewWorkflowTemplate(
		uuid.NewString(), importerTenantID, source.Name+" (imported)", resolved.Template.DAGJSON,
		domain.ScopePersonal, "" /* no parent — a cross-tenant import is disconnected, like Clone */, importerUserID,
		domain.WithDescription(source.Description),
		domain.WithTags(source.Tags),
		domain.WithClonedFrom(source.ID), // provenance across the tenant boundary
	)
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_TEMPLATE", err.Error(), err)
	}
	if err := uc.templates.CreateTemplate(ctx, tmpl); err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_SAVE_FAILED", "failed to persist imported workflow template", err)
	}
	return tmpl, nil
}
