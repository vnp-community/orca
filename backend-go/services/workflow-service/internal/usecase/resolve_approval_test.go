package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func resolveTestCtx(userID string) context.Context {
	return tenant.WithUserID(tenant.WithTenantID(context.Background(), "tenant-1"), userID)
}

func seedPendingApproval(t *testing.T, approvals *fakeApprovalRepository, id, templateID, requestedBy string) domain.Approval {
	t.Helper()
	a, err := domain.NewApproval(id, "tenant-1", templateID, requestedBy)
	if err != nil {
		t.Fatalf("building approval: %v", err)
	}
	approvals.approvals[id] = a
	return a
}

func TestResolveApproval_NonAdmin_Rejected(t *testing.T) {
	templates := newFakeTemplateRepository()
	mustSeedTemplate(t, templates, "tmpl-1", "tenant-1", "owner-1", domain.VisibilityTeam)
	approvals := newFakeApprovalRepository(templates)
	seedPendingApproval(t, approvals, "appr-1", "tmpl-1", "owner-1")
	opa := newFakeOPAChecker() // no admins

	uc := NewResolveApproval(approvals, opa)
	_, err := uc.Execute(resolveTestCtx("owner-1"), ResolveApprovalInput{ApprovalID: "appr-1", Decision: domain.ApprovalApproved})

	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindPermissionDenied {
		t.Fatalf("expected KindPermissionDenied, got %v", err)
	}
}

func TestResolveApproval_Approve_AppliesVisibilityCompanyAtomically(t *testing.T) {
	templates := newFakeTemplateRepository()
	mustSeedTemplate(t, templates, "tmpl-1", "tenant-1", "owner-1", domain.VisibilityTeam)
	approvals := newFakeApprovalRepository(templates)
	seedPendingApproval(t, approvals, "appr-1", "tmpl-1", "owner-1")
	opa := newFakeOPAChecker("admin-1")

	uc := NewResolveApproval(approvals, opa)
	got, err := uc.Execute(resolveTestCtx("admin-1"), ResolveApprovalInput{ApprovalID: "appr-1", Decision: domain.ApprovalApproved})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != domain.ApprovalApproved {
		t.Errorf("expected approval status=approved, got %s", got.Status)
	}
	if got.ResolvedBy != "admin-1" {
		t.Errorf("expected resolved_by=admin-1, got %s", got.ResolvedBy)
	}
	if templates.templates["tmpl-1"].Visibility != domain.VisibilityCompany {
		t.Errorf("expected template visibility=company after approval, got %s", templates.templates["tmpl-1"].Visibility)
	}
}

func TestResolveApproval_Reject_LeavesVisibilityUnchanged(t *testing.T) {
	templates := newFakeTemplateRepository()
	mustSeedTemplate(t, templates, "tmpl-1", "tenant-1", "owner-1", domain.VisibilityTeam)
	approvals := newFakeApprovalRepository(templates)
	seedPendingApproval(t, approvals, "appr-1", "tmpl-1", "owner-1")
	opa := newFakeOPAChecker("admin-1")

	uc := NewResolveApproval(approvals, opa)
	got, err := uc.Execute(resolveTestCtx("admin-1"), ResolveApprovalInput{ApprovalID: "appr-1", Decision: domain.ApprovalRejected})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != domain.ApprovalRejected {
		t.Errorf("expected approval status=rejected, got %s", got.Status)
	}
	if templates.templates["tmpl-1"].Visibility != domain.VisibilityTeam {
		t.Errorf("expected template visibility to remain unchanged (team) on rejection, got %s", templates.templates["tmpl-1"].Visibility)
	}
}

// TestResolveApproval_FailureBetweenWrites_RollsBackBoth injects a failure
// on the SECOND write within the transaction (the template's SetVisibility,
// via templatesTxFailAfterWrites) after the FIRST write (the approval's
// Update) already staged successfully — asserting neither side effect
// landed once WithTx rolls back, per this task's Verify section.
func TestResolveApproval_FailureBetweenWrites_RollsBackBoth(t *testing.T) {
	templates := newFakeTemplateRepository()
	mustSeedTemplate(t, templates, "tmpl-1", "tenant-1", "owner-1", domain.VisibilityTeam)
	approvals := newFakeApprovalRepository(templates)
	seedPendingApproval(t, approvals, "appr-1", "tmpl-1", "owner-1")
	opa := newFakeOPAChecker("admin-1")

	// The approval's own Update is the 1st write in the tx; the template's
	// SetVisibility (via Templates()) is a separate counter — fail it on
	// its very first call so the approval Update has already staged.
	approvals.templatesTxFailAfterWrites = 1

	uc := NewResolveApproval(approvals, opa)
	_, err := uc.Execute(resolveTestCtx("admin-1"), ResolveApprovalInput{ApprovalID: "appr-1", Decision: domain.ApprovalApproved})
	if err == nil {
		t.Fatal("expected the injected mid-transaction failure to propagate")
	}

	if approvals.approvals["appr-1"].Status != domain.ApprovalPending {
		t.Errorf("expected approval status to remain pending after rollback, got %s", approvals.approvals["appr-1"].Status)
	}
	if templates.templates["tmpl-1"].Visibility != domain.VisibilityTeam {
		t.Errorf("expected template visibility to remain unchanged after rollback, got %s", templates.templates["tmpl-1"].Visibility)
	}
}

func TestResolveApproval_NotFound(t *testing.T) {
	templates := newFakeTemplateRepository()
	approvals := newFakeApprovalRepository(templates)
	opa := newFakeOPAChecker("admin-1")

	uc := NewResolveApproval(approvals, opa)
	_, err := uc.Execute(resolveTestCtx("admin-1"), ResolveApprovalInput{ApprovalID: "does-not-exist", Decision: domain.ApprovalApproved})

	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindNotFound {
		t.Fatalf("expected KindNotFound, got %v", err)
	}
}
