package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

type ListTemplatesInput struct {
	Scope string // optional filter, empty = all scopes
	// Query is a full-text filter against name/description
	// (idx_templates_fts) — empty means no query filter.
	Query string
	// Tags is an AND-filter: every listed tag must be present on a
	// matching template, not any (empty means no tag filter).
	Tags []string
	// Sort selects the ordering: "" (default, id order) | "trending"
	// (usage_count DESC, rating_sum DESC) | "recent" (updated_at DESC).
	Sort      string
	PageToken string
	PageSize  int32
}

type ListTemplatesOutput struct {
	Templates     []domain.WorkflowTemplate
	NextPageToken string
}

// ListTemplates is the other item Epic C left deferred alongside
// ResolveTemplate (docs/execution-plan.md §2/§10) — implemented together
// with it, 2026-08-17. Keyset-paginated, matching annotation-service's
// ListAnnotations convention exactly (opaque page_token = last-seen id).
type ListTemplates struct {
	repo TemplateRepository
}

func NewListTemplates(repo TemplateRepository) *ListTemplates {
	return &ListTemplates{repo: repo}
}

func (uc *ListTemplates) Execute(ctx context.Context, in ListTemplatesInput) (ListTemplatesOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ListTemplatesOutput{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}
	if in.Scope != "" && !domain.Scope(in.Scope).Valid() {
		return ListTemplatesOutput{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_SCOPE", "invalid scope filter", nil)
	}
	switch in.Sort {
	case "", "trending", "recent":
	default:
		return ListTemplatesOutput{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_SORT", `sort must be "trending", "recent", or empty`, nil)
	}

	pageSize := in.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}

	templates, next, err := uc.repo.ListTemplates(ctx, tenantID, in.Scope, in.Query, in.Tags, in.Sort, in.PageToken, pageSize)
	if err != nil {
		return ListTemplatesOutput{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_LIST_FAILED", "failed to list workflow templates", err)
	}
	return ListTemplatesOutput{Templates: templates, NextPageToken: next}, nil
}
