package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// SharedTemplatePreview is the read-only projection PreviewSharedTemplate
// returns — deliberately narrow: never OwnerID, never TenantID, never any
// field that would leak who owns the template or let a caller pivot to
// that tenant's other data. See PreviewSharedTemplate's doc comment.
type SharedTemplatePreview struct {
	Name        string
	Description string
	Tags        []string
	DAGJSON     string
	// RatingSum/RatingCount mirror the proto SharedTemplatePreview message's
	// own fields exactly (TASK-WF-03-03) — raw sum+count, not a
	// pre-computed average, so the caller (or a UI averaging client-side)
	// isn't stuck with a lossy float->int round-trip through the wire.
	RatingSum   int32
	RatingCount int32
	// AverageRating is a convenience derived value for a Go caller that
	// wants it directly (e.g. a future CLI/log line) — see
	// domain.WorkflowTemplate.AverageRating's doc comment for why it's
	// derived, never stored.
	AverageRating float64
}

func toPreview(t domain.WorkflowTemplate) SharedTemplatePreview {
	return SharedTemplatePreview{
		Name:          t.Name,
		Description:   t.Description,
		Tags:          t.Tags,
		DAGJSON:       t.DAGJSON,
		RatingSum:     t.RatingSum,
		RatingCount:   t.RatingCount,
		AverageRating: t.AverageRating(),
	}
}

// PreviewSharedTemplate is the ONE usecase in this service with NO
// tenant/auth context at all — a deliberate, narrow exception to
// 05-data-architecture.md's tenant-scoping rule. Looked up by share_token
// (unguessable, not a template id an attacker could enumerate); returns
// only SharedTemplatePreview's read-only projection — never owner_id,
// never any other tenant's templates, never a list/search surface.
type PreviewSharedTemplate struct {
	templates TemplateRepository
}

func NewPreviewSharedTemplate(templates TemplateRepository) *PreviewSharedTemplate {
	return &PreviewSharedTemplate{templates: templates}
}

func (uc *PreviewSharedTemplate) Execute(ctx context.Context, shareToken string) (SharedTemplatePreview, error) {
	if shareToken == "" {
		return SharedTemplatePreview{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_SHARE_TOKEN_REQUIRED", "share_token is required", nil)
	}
	tmpl, err := uc.templates.GetByShareToken(ctx, shareToken) // no tenantID param — the token IS the lookup key
	if err != nil || tmpl.Visibility != domain.VisibilityPublic {
		// Same error for "no such token" and "template since unpublished" —
		// deliberately don't leak which, so an attacker can't use response
		// shape to probe token validity separately from publish state.
		return SharedTemplatePreview{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_SHARE_LINK_INVALID", "share link is invalid or expired", nil)
	}
	return toPreview(tmpl), nil
}
