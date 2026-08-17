package usecase

import (
	"context"
	"testing"
)

func TestListAnnotations_RequiresTenantContext(t *testing.T) {
	uc := NewListAnnotations(newFakeRepository())
	_, err := uc.Execute(context.Background(), ListAnnotationsInput{RepoID: "repo-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestListAnnotations_RequiresRepoID(t *testing.T) {
	uc := NewListAnnotations(newFakeRepository())
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, ListAnnotationsInput{})
	if err == nil {
		t.Fatal("expected an error when repo_id is empty")
	}
}

func TestListAnnotations_FiltersByTenantAndRepo(t *testing.T) {
	repo := newFakeRepository()
	createUC := NewCreateAnnotation(repo)
	ctx1 := withIdentity(context.Background(), "tenant-1", "user-1")
	ctx2 := withIdentity(context.Background(), "tenant-2", "user-1")

	if _, err := createUC.Execute(ctx1, CreateAnnotationInput{RepoID: "repo-1", FilePath: "main.go", Line: 1, Content: "a"}); err != nil {
		t.Fatalf("seed create (tenant-1): %v", err)
	}
	if _, err := createUC.Execute(ctx2, CreateAnnotationInput{RepoID: "repo-1", FilePath: "main.go", Line: 1, Content: "b"}); err != nil {
		t.Fatalf("seed create (tenant-2): %v", err)
	}

	listUC := NewListAnnotations(repo)
	out, err := listUC.Execute(ctx1, ListAnnotationsInput{RepoID: "repo-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Annotations) != 1 || out.Annotations[0].TenantID != "tenant-1" {
		t.Errorf("expected only tenant-1's annotation, got %+v", out.Annotations)
	}
}

func TestListAnnotations_ClampsPageSize(t *testing.T) {
	repo := newFakeRepository()
	uc := NewListAnnotations(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ListAnnotationsInput{RepoID: "repo-1", PageSize: -1}); err != nil {
		t.Fatalf("unexpected error with negative page size: %v", err)
	}
	if _, err := uc.Execute(ctx, ListAnnotationsInput{RepoID: "repo-1", PageSize: 10000}); err != nil {
		t.Fatalf("unexpected error with oversized page size: %v", err)
	}
}
