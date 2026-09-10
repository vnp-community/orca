package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func TestGenerateShareLink_NonPublicTemplateRejected(t *testing.T) {
	repo := newFakeTemplateRepository()
	tmpl := mustNewTemplate(t, repo, "tmpl-1", "tenant-1", "deploy", `{"steps":[]}`, "")

	uc := NewGenerateShareLink(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, tmpl.ID)
	if err == nil {
		t.Fatal("expected an error for a non-public template")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKFLOW_TEMPLATE_NOT_PUBLIC" {
		t.Errorf("expected WORKFLOW_TEMPLATE_NOT_PUBLIC, got %v", err)
	}
}

func TestGenerateShareLink_PublicTemplate_ReturnsToken(t *testing.T) {
	repo := newFakeTemplateRepository()
	tmpl, err := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1", domain.WithVisibility(domain.VisibilityPublic))
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	_ = repo.CreateTemplate(context.Background(), tmpl)

	uc := NewGenerateShareLink(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	token, err := uc.Execute(ctx, tmpl.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected a non-empty share token")
	}

	got, err := repo.GetByShareToken(ctx, token)
	if err != nil {
		t.Fatalf("expected the token to resolve back to the template: %v", err)
	}
	if got.ID != tmpl.ID {
		t.Errorf("expected the resolved template to be %s, got %s", tmpl.ID, got.ID)
	}
}

func TestGenerateShareLink_RepeatedCalls_ReturnSameToken(t *testing.T) {
	repo := newFakeTemplateRepository()
	tmpl, err := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1", domain.WithVisibility(domain.VisibilityPublic))
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	_ = repo.CreateTemplate(context.Background(), tmpl)

	uc := NewGenerateShareLink(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	first, err := uc.Execute(ctx, tmpl.ID)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	second, err := uc.Execute(ctx, tmpl.ID)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if first != second {
		t.Errorf("expected an already-public template's repeated GenerateShareLink calls to return the SAME token, got %q then %q", first, second)
	}
}

func TestGenerateShareLink_UnknownTemplate_NotFound(t *testing.T) {
	repo := newFakeTemplateRepository()
	uc := NewGenerateShareLink(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, "does-not-exist")
	if err == nil {
		t.Fatal("expected a not-found error")
	}
}
