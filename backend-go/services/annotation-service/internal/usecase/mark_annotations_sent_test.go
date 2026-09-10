package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
)

func TestMarkAnnotationsSent_MarksAllPresentIDs(t *testing.T) {
	repo := newFakeRepository()
	createCtx := withIdentity(context.Background(), "tenant-1", "author-1")

	a1, err := NewCreateAnnotation(repo).Execute(createCtx, CreateAnnotationInput{
		RepoID: "repo-1", FilePath: "main.go", Line: 1, Content: "one", RequestID: "req-1",
	})
	if err != nil {
		t.Fatalf("seed create a1: %v", err)
	}
	a2, err := NewCreateAnnotation(repo).Execute(createCtx, CreateAnnotationInput{
		RepoID: "repo-1", FilePath: "main.go", Line: 2, Content: "two", RequestID: "req-2",
	})
	if err != nil {
		t.Fatalf("seed create a2: %v", err)
	}

	uc := NewMarkAnnotationsSent(repo)
	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	updated, err := uc.Execute(ctx, MarkAnnotationsSentInput{IDs: []string{a1.ID, a2.ID}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updated) != 2 {
		t.Fatalf("expected 2 updated annotations, got %d", len(updated))
	}
	for _, a := range updated {
		if !a.SentToAgent || a.SentAt == nil {
			t.Errorf("expected SentToAgent=true and SentAt set, got %+v", a)
		}
	}
}

func TestMarkAnnotationsSent_EmptyIDsReturnsNoError(t *testing.T) {
	repo := newFakeRepository()
	uc := NewMarkAnnotationsSent(repo)
	ctx := tenant.WithTenantID(context.Background(), "tenant-1")

	updated, err := uc.Execute(ctx, MarkAnnotationsSentInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updated) != 0 {
		t.Errorf("expected no updated annotations, got %d", len(updated))
	}
	if repo.markSentCalls != 0 {
		t.Errorf("expected repository not to be called for an empty IDs slice, got %d calls", repo.markSentCalls)
	}
}

func TestMarkAnnotationsSent_RequiresTenantContext(t *testing.T) {
	repo := newFakeRepository()
	uc := NewMarkAnnotationsSent(repo)
	_, err := uc.Execute(context.Background(), MarkAnnotationsSentInput{IDs: []string{"a1"}})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
