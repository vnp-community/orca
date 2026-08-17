package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
)

func TestDeleteAnnotation_RequiresTenantContext(t *testing.T) {
	uc := NewDeleteAnnotation(newFakeRepository(), newFakeOPAClient(true))
	err := uc.Execute(context.Background(), DeleteAnnotationInput{ID: "a1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestDeleteAnnotation_RequiresUserContext(t *testing.T) {
	uc := NewDeleteAnnotation(newFakeRepository(), newFakeOPAClient(true))
	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	err := uc.Execute(ctx, DeleteAnnotationInput{ID: "a1"})
	if err == nil {
		t.Fatal("expected an error when no user is in context")
	}
}

func TestDeleteAnnotation_RequiresID(t *testing.T) {
	uc := NewDeleteAnnotation(newFakeRepository(), newFakeOPAClient(true))
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	err := uc.Execute(ctx, DeleteAnnotationInput{})
	if err == nil {
		t.Fatal("expected an error when id is empty")
	}
}

func TestDeleteAnnotation_NotFoundPropagates(t *testing.T) {
	uc := NewDeleteAnnotation(newFakeRepository(), newFakeOPAClient(true))
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
		RepoID: "repo-1", FilePath: "main.go", Line: 1, Content: "original", RequestID: "req-seed",
	})
	if err != nil {
		t.Fatalf("seed create: %v", err)
	}

	if err := NewDeleteAnnotation(repo, newFakeOPAClient(true)).Execute(ctx, DeleteAnnotationInput{ID: created.ID}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.byID[created.ID]; ok {
		t.Error("expected annotation to be removed from repository")
	}
}

// TestDeleteAnnotation_AuthorMayDelete exercises the author-matches-actor
// allow branch (via the fake OPA client) — the annotation's author
// deleting their own annotation.
func TestDeleteAnnotation_AuthorMayDelete(t *testing.T) {
	repo := newFakeRepository()
	createCtx := withIdentity(context.Background(), "tenant-1", "author-1")
	created, err := NewCreateAnnotation(repo).Execute(createCtx, CreateAnnotationInput{
		RepoID: "repo-1", FilePath: "main.go", Line: 1, Content: "original", RequestID: "req-seed",
	})
	if err != nil {
		t.Fatalf("seed create: %v", err)
	}

	opa := newFakeOPAClient(true)
	ctx := withIdentity(context.Background(), "tenant-1", "author-1")
	if err := NewDeleteAnnotation(repo, opa).Execute(ctx, DeleteAnnotationInput{ID: created.ID}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.byID[created.ID]; ok {
		t.Error("expected annotation to be removed from repository")
	}
	if len(opa.calls) != 1 || opa.calls[0].actorID != "author-1" || opa.calls[0].authorID != "author-1" {
		t.Errorf("expected OPA to be queried with actor==author, got %+v", opa.calls)
	}
}

// TestDeleteAnnotation_NonAuthorNonAdminDenied covers the deny path and
// asserts the mutation never reaches the repository — the annotation must
// still exist afterward.
func TestDeleteAnnotation_NonAuthorNonAdminDenied(t *testing.T) {
	repo := newFakeRepository()
	createCtx := withIdentity(context.Background(), "tenant-1", "author-1")
	created, err := NewCreateAnnotation(repo).Execute(createCtx, CreateAnnotationInput{
		RepoID: "repo-1", FilePath: "main.go", Line: 1, Content: "original", RequestID: "req-seed",
	})
	if err != nil {
		t.Fatalf("seed create: %v", err)
	}

	opa := newFakeOPAClient(false)
	ctx := withIdentity(context.Background(), "tenant-1", "someone-else")
	err = NewDeleteAnnotation(repo, opa).Execute(ctx, DeleteAnnotationInput{ID: created.ID})
	if err == nil {
		t.Fatal("expected a permission-denied error for a non-author, non-admin caller")
	}
	if len(opa.calls) != 1 || opa.calls[0].actorID != "someone-else" || opa.calls[0].authorID != "author-1" {
		t.Errorf("expected OPA to be queried with actor!=author, got %+v", opa.calls)
	}
	if _, ok := repo.byID[created.ID]; !ok {
		t.Error("expected the annotation to still exist — denied delete must not mutate")
	}
}

// TestDeleteAnnotation_OPAErrorFailsClosed asserts that an evaluator error
// denies the request rather than allowing it — fail closed, per
// common/policy.Evaluator's contract.
func TestDeleteAnnotation_OPAErrorFailsClosed(t *testing.T) {
	repo := newFakeRepository()
	createCtx := withIdentity(context.Background(), "tenant-1", "author-1")
	created, err := NewCreateAnnotation(repo).Execute(createCtx, CreateAnnotationInput{
		RepoID: "repo-1", FilePath: "main.go", Line: 1, Content: "original", RequestID: "req-seed",
	})
	if err != nil {
		t.Fatalf("seed create: %v", err)
	}

	opa := newFakeOPAClient(true)
	opa.err = errors.New("bundle unavailable")
	ctx := withIdentity(context.Background(), "tenant-1", "author-1")
	err = NewDeleteAnnotation(repo, opa).Execute(ctx, DeleteAnnotationInput{ID: created.ID})
	if err == nil {
		t.Fatal("expected an error when the policy evaluator fails")
	}
	if _, ok := repo.byID[created.ID]; !ok {
		t.Error("expected the annotation to still exist — evaluator error must fail closed")
	}
}
