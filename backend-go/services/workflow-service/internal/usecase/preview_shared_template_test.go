package usecase

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func TestPreviewSharedTemplate_UnknownToken_InvalidLink(t *testing.T) {
	repo := newFakeTemplateRepository()
	uc := NewPreviewSharedTemplate(repo)

	_, err := uc.Execute(context.Background(), "unknown-token")
	if err == nil {
		t.Fatal("expected an error for an unknown token")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKFLOW_SHARE_LINK_INVALID" {
		t.Errorf("expected WORKFLOW_SHARE_LINK_INVALID, got %v", err)
	}
}

func TestPreviewSharedTemplate_SinceUnpublished_SameInvalidLinkError(t *testing.T) {
	repo := newFakeTemplateRepository()
	tmpl, err := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1",
		domain.WithVisibility(domain.VisibilityPrivate), domain.WithShareToken("stale-token"))
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	_ = repo.CreateTemplate(context.Background(), tmpl)

	uc := NewPreviewSharedTemplate(repo)
	_, unknownErr := uc.Execute(context.Background(), "unknown-token")
	_, staleErr := uc.Execute(context.Background(), "stale-token")

	var unknownAE, staleAE *apperrors.AppError
	if !errors.As(unknownErr, &unknownAE) || !errors.As(staleErr, &staleAE) {
		t.Fatalf("expected both to be *apperrors.AppError, got %v / %v", unknownErr, staleErr)
	}
	if unknownAE.Code != staleAE.Code || unknownAE.Message != staleAE.Message {
		t.Errorf("expected an unknown token and a since-unpublished token to return the IDENTICAL error (don't leak which), got %+v vs %+v", unknownAE, staleAE)
	}
}

func TestPreviewSharedTemplate_ValidPublicToken_ReturnsProjection(t *testing.T) {
	repo := newFakeTemplateRepository()
	tmpl, err := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "deploy", `{"steps":[{"id":"s1","type":"shell"}]}`, domain.ScopePersonal, "", "owner-1",
		domain.WithVisibility(domain.VisibilityPublic), domain.WithShareToken("valid-token"),
		domain.WithDescription("a deploy template"), domain.WithTags([]string{"deploy", "ci"}), domain.WithRating(12, 3))
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	_ = repo.CreateTemplate(context.Background(), tmpl)

	uc := NewPreviewSharedTemplate(repo)
	preview, err := uc.Execute(context.Background(), "valid-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if preview.Name != "deploy" || preview.Description != "a deploy template" || preview.DAGJSON == "" {
		t.Errorf("expected the projection populated from the template, got %+v", preview)
	}
	if preview.RatingSum != 12 || preview.RatingCount != 3 {
		t.Errorf("expected rating_sum=12 rating_count=3, got %+v", preview)
	}
	if preview.AverageRating != 4 {
		t.Errorf("expected AverageRating=4, got %v", preview.AverageRating)
	}
}

// TestPreviewSharedTemplate_ProjectionNeverLeaksOwnerOrTenant asserts, via
// reflection over SharedTemplatePreview's field set, that it structurally
// cannot carry owner_id/tenant_id — the task's own required assertion
// mechanism ("assert via reflection over the returned struct's field set").
func TestPreviewSharedTemplate_ProjectionNeverLeaksOwnerOrTenant(t *testing.T) {
	fields := reflect.TypeOf(SharedTemplatePreview{})
	for i := 0; i < fields.NumField(); i++ {
		name := strings.ToLower(fields.Field(i).Name)
		if strings.Contains(name, "owner") || strings.Contains(name, "tenant") {
			t.Errorf("SharedTemplatePreview must never carry an owner/tenant-identifying field, found %q", fields.Field(i).Name)
		}
	}
}
