package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func publishTestCtx(tenantID, userID string) context.Context {
	return tenant.WithUserID(tenant.WithTenantID(context.Background(), tenantID), userID)
}

func mustSeedTemplate(t *testing.T, repo *fakeTemplateRepository, id, tenantID, ownerID string, visibility domain.Visibility) domain.WorkflowTemplate {
	t.Helper()
	tmpl, err := domain.NewWorkflowTemplate(id, tenantID, "t-"+id, `{"steps":[]}`, domain.ScopePersonal, "", ownerID, domain.WithVisibility(visibility))
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(context.Background(), tmpl); err != nil {
		t.Fatalf("seeding template: %v", err)
	}
	return tmpl
}

func TestPublishTemplate_NonOwnerNonAdmin_Rejected(t *testing.T) {
	templates := newFakeTemplateRepository()
	mustSeedTemplate(t, templates, "tmpl-1", "tenant-1", "owner-1", domain.VisibilityPrivate)
	approvals := newFakeApprovalRepository(templates)
	opa := newFakeOPAChecker() // no admins

	uc := NewPublishTemplate(templates, approvals, opa)
	_, err := uc.Execute(publishTestCtx("tenant-1", "intruder"), PublishTemplateInput{TemplateID: "tmpl-1", NewVisibility: domain.VisibilityTeam})

	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindPermissionDenied {
		t.Fatalf("expected KindPermissionDenied, got %v", err)
	}
}

func TestPublishTemplate_OwnerEscalatesPrivateToTeam_NoApproval(t *testing.T) {
	templates := newFakeTemplateRepository()
	mustSeedTemplate(t, templates, "tmpl-1", "tenant-1", "owner-1", domain.VisibilityPrivate)
	approvals := newFakeApprovalRepository(templates)
	opa := newFakeOPAChecker()

	uc := NewPublishTemplate(templates, approvals, opa)
	got, err := uc.Execute(publishTestCtx("tenant-1", "owner-1"), PublishTemplateInput{TemplateID: "tmpl-1", NewVisibility: domain.VisibilityTeam})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Visibility != domain.VisibilityTeam {
		t.Errorf("expected visibility=team, got %s", got.Visibility)
	}
	if len(approvals.approvals) != 0 {
		t.Errorf("expected no approval row for a non-company escalation, got %d", len(approvals.approvals))
	}
}

func TestPublishTemplate_LeadEscalatesToCompany_CreatesPendingApproval(t *testing.T) {
	templates := newFakeTemplateRepository()
	mustSeedTemplate(t, templates, "tmpl-1", "tenant-1", "owner-1", domain.VisibilityTeam)
	approvals := newFakeApprovalRepository(templates)
	opa := newFakeOPAChecker() // owner-1 is not an admin

	uc := NewPublishTemplate(templates, approvals, opa)
	got, err := uc.Execute(publishTestCtx("tenant-1", "owner-1"), PublishTemplateInput{TemplateID: "tmpl-1", NewVisibility: domain.VisibilityCompany})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Visibility != domain.VisibilityTeam {
		t.Errorf("expected visibility to remain unchanged (team) pending approval, got %s", got.Visibility)
	}
	if len(approvals.approvals) != 1 {
		t.Fatalf("expected exactly one pending approval, got %d", len(approvals.approvals))
	}
	for _, a := range approvals.approvals {
		if a.Status != domain.ApprovalPending {
			t.Errorf("expected approval status=pending, got %s", a.Status)
		}
		if a.RequestedBy != "owner-1" {
			t.Errorf("expected requested_by=owner-1, got %s", a.RequestedBy)
		}
	}
}

func TestPublishTemplate_AdminEscalatesToCompany_AppliesImmediately(t *testing.T) {
	templates := newFakeTemplateRepository()
	mustSeedTemplate(t, templates, "tmpl-1", "tenant-1", "owner-1", domain.VisibilityTeam)
	approvals := newFakeApprovalRepository(templates)
	opa := newFakeOPAChecker("admin-1")

	uc := NewPublishTemplate(templates, approvals, opa)
	got, err := uc.Execute(publishTestCtx("tenant-1", "admin-1"), PublishTemplateInput{TemplateID: "tmpl-1", NewVisibility: domain.VisibilityCompany})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Visibility != domain.VisibilityCompany {
		t.Errorf("expected visibility=company applied immediately, got %s", got.Visibility)
	}
	if len(approvals.approvals) != 0 {
		t.Errorf("expected no approval row for an admin's own publish, got %d", len(approvals.approvals))
	}
}

func TestPublishTemplate_SecondPendingRequest_ConflictError(t *testing.T) {
	templates := newFakeTemplateRepository()
	mustSeedTemplate(t, templates, "tmpl-1", "tenant-1", "owner-1", domain.VisibilityTeam)
	approvals := newFakeApprovalRepository(templates)
	opa := newFakeOPAChecker()

	uc := NewPublishTemplate(templates, approvals, opa)
	ctx := publishTestCtx("tenant-1", "owner-1")
	if _, err := uc.Execute(ctx, PublishTemplateInput{TemplateID: "tmpl-1", NewVisibility: domain.VisibilityCompany}); err != nil {
		t.Fatalf("first request: unexpected error: %v", err)
	}

	_, err := uc.Execute(ctx, PublishTemplateInput{TemplateID: "tmpl-1", NewVisibility: domain.VisibilityCompany})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindAlreadyExists {
		t.Fatalf("expected a clean typed conflict (KindAlreadyExists), got %v", err)
	}
	if !errors.Is(err, domain.ErrApprovalAlreadyPending) {
		t.Fatalf("expected the wrapped cause to be domain.ErrApprovalAlreadyPending, got %v", err)
	}
}
