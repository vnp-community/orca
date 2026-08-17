package usecase

import (
	"context"
	"testing"
)

func TestDeleteAnnotation_RequiresTenantContext(t *testing.T) {
	uc := NewDeleteAnnotation(newFakeRepository())
	err := uc.Execute(context.Background(), DeleteAnnotationInput{ID: "a1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestDeleteAnnotation_RequiresID(t *testing.T) {
	uc := NewDeleteAnnotation(newFakeRepository())
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	err := uc.Execute(ctx, DeleteAnnotationInput{})
	if err == nil {
		t.Fatal("expected an error when id is empty")
	}
}

func TestDeleteAnnotation_NotFoundPropagates(t *testing.T) {
	uc := NewDeleteAnnotation(newFakeRepository())
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	err := uc.Execute(ctx, DeleteAnnotationInput{ID: "missing"})
	if err == nil {
		t.Fatal("expected an error for an annotation that doesn't exist")
	}
}

func TestDeleteAnnotation_DeletesExisting(t *testing.T) {
	repo := newFakeRepository()
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	created, err := NewCreateAnnotation(repo).Execute(ctx, CreateAnnotationInput{
		RepoID: "repo-1", FilePath: "main.go", Line: 1, Content: "original",
	})
	if err != nil {
		t.Fatalf("seed create: %v", err)
	}

	if err := NewDeleteAnnotation(repo).Execute(ctx, DeleteAnnotationInput{ID: created.ID}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.byID[created.ID]; ok {
		t.Error("expected annotation to be removed from repository")
	}
}
