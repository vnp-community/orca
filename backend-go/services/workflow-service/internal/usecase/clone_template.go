package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// CloneTemplateInput is CloneTemplate's request shape — see
// CloneTemplate's doc comment for why this is a separate usecase rather
// than a flag on CreateTemplateInput.
type CloneTemplateInput struct {
	SourceTemplateID  string
	Name, Description string
	Tags              []string
}

// CloneTemplate snapshots a RESOLVED (post-inheritance) template into a
// brand-new, disconnected root template. BUG-WF-01 found no
// disconnected-copy creation path at all: CreateTemplate always takes
// caller-supplied dag_json, which for an Inherit-mode source may be empty
// or override-only and meaningless standalone — Clone instead runs the
// source through ResolveTemplate and persists ITS resulting dag_json,
// so the clone stands alone even if the source template is later edited,
// re-parented, or deleted.
type CloneTemplate struct {
	resolve *ResolveTemplate // reused as-is — Clone is a thin usecase on top
	repo    TemplateRepository
}

func NewCloneTemplate(resolve *ResolveTemplate, repo TemplateRepository) *CloneTemplate {
	return &CloneTemplate{resolve: resolve, repo: repo}
}

func (uc *CloneTemplate) Execute(ctx context.Context, in CloneTemplateInput) (domain.WorkflowTemplate, error) {
	if in.SourceTemplateID == "" {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_CLONE_SOURCE_REQUIRED", "source_template_id is required", nil)
	}
	if in.Name == "" {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_CLONE_NAME_REQUIRED", "name is required", nil)
	}

	resolved, err := uc.resolve.Execute(ctx, ResolveTemplateInput{TemplateID: in.SourceTemplateID})
	if err != nil {
		return domain.WorkflowTemplate{}, err
	}

	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}
	ownerID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_USER", "no user in request context", nil)
	}

	tmpl, err := domain.NewWorkflowTemplate(
		uuid.NewString(), tenantID, in.Name, resolved.Template.DAGJSON,
		resolved.Template.Scope, "" /* no parent — Clone is disconnected */, ownerID,
		domain.WithDescription(in.Description),
		domain.WithTags(in.Tags),
		domain.WithClonedFrom(in.SourceTemplateID),
	)
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_TEMPLATE", err.Error(), err)
	}

	if err := uc.repo.CreateTemplate(ctx, tmpl); err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_SAVE_FAILED", "failed to persist cloned workflow template", err)
	}
	return tmpl, nil
}
