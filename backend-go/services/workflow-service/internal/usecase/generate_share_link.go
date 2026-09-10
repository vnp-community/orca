package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// GenerateShareLink mints (or returns the existing) share_token for a
// template once its Visibility has reached VisibilityPublic — the token IS
// the access-control boundary for a public template (see
// PreviewSharedTemplate/ImportSharedTemplate, which look a template up by
// token alone, with no tenant/auth context).
type GenerateShareLink struct {
	templates TemplateRepository
}

func NewGenerateShareLink(templates TemplateRepository) *GenerateShareLink {
	return &GenerateShareLink{templates: templates}
}

func (uc *GenerateShareLink) Execute(ctx context.Context, templateID string) (string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}
	if templateID == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_TEMPLATE_ID_REQUIRED", "template_id is required", nil)
	}

	tmpl, err := uc.templates.GetTemplate(ctx, tenantID, templateID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return "", apperrors.New(apperrors.KindNotFound, "WORKFLOW_TEMPLATE_NOT_FOUND", "workflow template not found", err)
		}
		return "", apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_FETCH_FAILED", "failed to fetch workflow template", err)
	}
	if tmpl.Visibility != domain.VisibilityPublic {
		return "", apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_TEMPLATE_NOT_PUBLIC", "template must be public to generate a share link", nil)
	}
	if tmpl.ShareToken != "" {
		// Idempotent — re-requesting a share link for an already-public
		// template must not rotate (and so invalidate) an already-shared
		// token.
		return tmpl.ShareToken, nil
	}

	token, err := generateOpaqueToken()
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "WORKFLOW_SHARE_TOKEN_GENERATE_FAILED", "failed to generate share token", err)
	}
	if err := uc.templates.SetShareToken(ctx, templateID, token); err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return "", apperrors.New(apperrors.KindNotFound, "WORKFLOW_TEMPLATE_NOT_FOUND", "workflow template not found", err)
		}
		return "", apperrors.New(apperrors.KindInternal, "WORKFLOW_SHARE_TOKEN_SAVE_FAILED", "failed to persist share token", err)
	}
	return token, nil
}

// generateOpaqueToken returns a cryptographically random, base64url
// (unpadded) opaque token — 32 bytes of entropy, unguessable by design
// since PreviewSharedTemplate/ImportSharedTemplate use possession of this
// token AS the access-control check.
func generateOpaqueToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
