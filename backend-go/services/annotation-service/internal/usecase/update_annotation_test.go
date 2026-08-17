package usecase

import (
	"context"
	"testing"
)

func TestUpdateAnnotation_RequiresTenantContext(t *testing.T) {
	uc := NewUpdateAnnotation(newFakeRepository())
	_, err := uc.Execute(context.Background(), UpdateAnnotationInput{ID: "a1", Content: "edited"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestUpdateAnnotation_RequiresID(t *testing.T) {
	uc := NewUpdateAnnotation(newFakeRepository())
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, UpdateAnnotationInput{Content: "edited"})
	if err == nil {
		t.Fatal("expected an error when id is empty")
	}
}

func TestUpdateAnnotation_RequiresContent(t *testing.T) {
	uc := NewUpdateAnnotation(newFakeRepository())
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, UpdateAnnotationInput{ID: "a1"})
	if err == nil {
		t.Fatal("expected an error when content is empty")
	}
}

func TestUpdateAnnotation_NotFoundPropagates(t *testing.T) {
	uc := NewUpdateAnnotation(newFakeRepository())
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, UpdateAnnotationInput{ID: "missing", Content: "edited"})
	if err == nil {
		t.Fatal("expected an error for an annotation that doesn't exist")
	}
}

func TestUpdateAnnotation_UpdatesContentAndResolved(t *testing.T) {
	repo := newFakeRepository()
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	created, err := NewCreateAnnotation(repo).Execute(ctx, CreateAnnotationInput{
		RepoID: "repo-1", FilePath: "main.go", Line: 1, Content: "original",
	})
	if err != nil {
		t.Fatalf("seed create: %v", err)
	}

	updated, err := NewUpdateAnnotation(repo).Execute(ctx, UpdateAnnotationInput{
		ID: created.ID, Content: "edited", Resolved: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Content != "edited" || !updated.Resolved {
		t.Errorf("expected updated content/resolved, got %+v", updated)
	}
}
