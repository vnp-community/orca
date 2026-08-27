package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// PublishTemplateInput is PublishTemplate's request shape.
type PublishTemplateInput struct {
	TemplateID    string
	NewVisibility domain.Visibility
}

// PublishTemplate escalates or de-escalates a template's Visibility —
// BUG-WF-03's publish state machine (see domain.Visibility.CanEscalateTo).
// Escalating to VisibilityCompany requires admin approval unless the
// caller is already an admin: a non-admin (lead) request creates a
// pending domain.Approval and leaves Visibility unchanged until an admin
// resolves it (usecase.ResolveApproval); everything else (any unpublish,
// any non-company escalation, or an admin's own company/public escalation)
// applies immediately. Both the approval-creation and the direct-apply
// paths are atomic (TemplateRepository.WithTx/ApprovalRepository.WithTx) —
// orchestration-service.md §8's discipline: a torn write here could leave
// a template silently already-company-visible with no pending approval to
// gate it, or an orphaned approval nobody can act on.
type PublishTemplate struct {
	templates TemplateRepository
	approvals ApprovalRepository
	opa       OPAChecker
}

func NewPublishTemplate(templates TemplateRepository, approvals ApprovalRepository, opa OPAChecker) *PublishTemplate {
	return &PublishTemplate{templates: templates, approvals: approvals, opa: opa}
}

func (uc *PublishTemplate) Execute(ctx context.Context, in PublishTemplateInput) (domain.WorkflowTemplate, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}
	callerUserID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_USER", "no user in request context", nil)
	}
	if !in.NewVisibility.Valid() {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_VISIBILITY", "unknown visibility", nil)
	}

	tmpl, err := uc.templates.GetTemplate(ctx, tenantID, in.TemplateID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_TEMPLATE_NOT_FOUND", "workflow template not found", err)
		}
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_FETCH_FAILED", "failed to fetch workflow template", err)
	}

	isAdmin := uc.opa.IsAdmin(ctx, callerUserID)
	if tmpl.OwnerID != callerUserID && !isAdmin {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindPermissionDenied, "WORKFLOW_PUBLISH_NOT_OWNER", "only the template owner or an admin may publish", nil)
	}
	if !tmpl.Visibility.CanEscalateTo(in.NewVisibility) {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_VISIBILITY_TRANSITION",
			fmt.Sprintf("cannot move directly from %s to %s", tmpl.Visibility, in.NewVisibility), nil)
	}

	if in.NewVisibility == domain.VisibilityCompany && !isAdmin {
		// Lead (non-admin) requesting company scope: create a pending
		// approval, visibility STAYS AT ITS CURRENT TIER until resolved.
		var result domain.WorkflowTemplate
		err := uc.approvals.WithTx(ctx, func(tx ApprovalRepositoryTx) error {
			approval, aerr := domain.NewApproval(uuid.NewString(), tenantID, tmpl.ID, callerUserID)
			if aerr != nil {
				return aerr
			}
			if cerr := tx.CreateTx(ctx, approval); cerr != nil {
				return cerr
			}
			result = tmpl // unchanged visibility — pending, not yet published
			return nil
		})
		if err != nil {
			if errors.Is(err, domain.ErrApprovalAlreadyPending) {
				return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindAlreadyExists, "WORKFLOW_APPROVAL_ALREADY_PENDING", "a pending approval already exists for this template", err)
			}
			return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_APPROVAL_CREATE_FAILED", "failed to create pending approval", err)
		}
		return result, nil
	}

	// Admin publishing directly, OR any non-company tier escalation, or
	// any unpublish — no approval gate.
	tmpl.Visibility = in.NewVisibility
	var result domain.WorkflowTemplate
	err = uc.templates.WithTx(ctx, func(tx TemplateRepositoryTx) error {
		var uerr error
		result, uerr = tx.UpdateVisibility(ctx, tmpl)
		return uerr
	})
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_TEMPLATE_NOT_FOUND", "workflow template not found", err)
		}
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_PUBLISH_FAILED", "failed to publish template", err)
	}
	return result, nil
}
