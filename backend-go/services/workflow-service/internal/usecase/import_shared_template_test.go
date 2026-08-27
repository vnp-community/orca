package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func TestImportSharedTemplate_CrossTenant_CreatesPersonalScopeCopyUnderImporterTenant(t *testing.T) {
	repo := newFakeTemplateRepository()
	source, err := domain.NewWorkflowTemplate("tmpl-source", "tenant-source", "shared-deploy", `{"steps":[{"id":"s1","type":"shell"}]}`, domain.ScopeCompany, "", "owner-source",
		domain.WithVisibility(domain.VisibilityPublic), domain.WithShareToken("share-tok"), domain.WithDescription("desc"), domain.WithTags([]string{"a"}))
	if err != nil {
		t.Fatalf("building source template: %v", err)
	}
	if err := repo.CreateTemplate(context.Background(), source); err != nil {
		t.Fatalf("creating source template: %v", err)
	}

	resolveUC := NewResolveTemplate(repo)
	uc := NewImportSharedTemplate(repo, resolveUC)
	// The importing caller belongs to a DIFFERENT tenant than the source.
	ctx := withTenantContext(context.Background(), "tenant-importer")

	imported, err := uc.Execute(ctx, "share-tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if imported.TenantID != "tenant-importer" {
		t.Errorf("expected the imported row under the IMPORTER's tenant, got %q", imported.TenantID)
	}
	if imported.ParentTemplateID != "" {
		t.Errorf("expected an empty ParentTemplateID (disconnected copy), got %q", imported.ParentTemplateID)
	}
	if imported.ClonedFromTemplateID != source.ID {
		t.Errorf("expected ClonedFromTemplateID=%s, got %q", source.ID, imported.ClonedFromTemplateID)
	}
	if imported.Scope != domain.ScopePersonal {
		t.Errorf("expected ScopePersonal, got %q", imported.Scope)
	}
	if !jsonEqualSteps(t, imported.DAGJSON, source.DAGJSON) {
		t.Errorf("expected the imported dag_json to match the resolved source, got %s", imported.DAGJSON)
	}

	// Persisted for real, under the importer's tenant.
	persisted, err := repo.GetTemplate(ctx, "tenant-importer", imported.ID)
	if err != nil {
		t.Fatalf("expected the import to be persisted: %v", err)
	}
	if persisted.ID == source.ID {
		t.Error("expected the import to have its own id, distinct from the source")
	}
}

func TestImportSharedTemplate_SinceUnpublishedToken_FailsSameAsPreview(t *testing.T) {
	repo := newFakeTemplateRepository()
	source, err := domain.NewWorkflowTemplate("tmpl-source", "tenant-source", "shared-deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-source",
		domain.WithVisibility(domain.VisibilityPrivate), domain.WithShareToken("stale-tok"))
	if err != nil {
		t.Fatalf("building source template: %v", err)
	}
	_ = repo.CreateTemplate(context.Background(), source)

	resolveUC := NewResolveTemplate(repo)
	uc := NewImportSharedTemplate(repo, resolveUC)
	previewUC := NewPreviewSharedTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-importer")

	_, importErr := uc.Execute(ctx, "stale-tok")
	_, previewErr := previewUC.Execute(ctx, "stale-tok")

	var importAE, previewAE *apperrors.AppError
	if !errors.As(importErr, &importAE) || !errors.As(previewErr, &previewAE) {
		t.Fatalf("expected both to be *apperrors.AppError, got %v / %v", importErr, previewErr)
	}
	if importAE.Code != previewAE.Code {
		t.Errorf("expected ImportSharedTemplate to fail the same way PreviewSharedTemplate does for a since-unpublished token, got %q vs %q", importAE.Code, previewAE.Code)
	}
}

func TestImportSharedTemplate_UnknownToken_NotFound(t *testing.T) {
	repo := newFakeTemplateRepository()
	resolveUC := NewResolveTemplate(repo)
	uc := NewImportSharedTemplate(repo, resolveUC)
	ctx := withTenantContext(context.Background(), "tenant-importer")

	_, err := uc.Execute(ctx, "does-not-exist")
	if err == nil {
		t.Fatal("expected an error for an unknown token")
	}
}
