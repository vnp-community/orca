package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func TestCloneTemplate_ClonesOwnStepsWithNoParent(t *testing.T) {
	repo := newFakeTemplateRepository()
	createUC := NewCreateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	source, err := createUC.Execute(ctx, CreateTemplateInput{Name: "source", Scope: "personal", DAGJSON: `{"steps":[{"id":"s1","type":"webhook"}]}`})
	if err != nil {
		t.Fatalf("creating source: %v", err)
	}

	uc := NewCloneTemplate(NewResolveTemplate(repo), repo)
	clone, err := uc.Execute(ctx, CloneTemplateInput{SourceTemplateID: source.ID, NewName: "clone", Scope: "personal"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clone.ID == source.ID {
		t.Error("expected the clone to have a different id from its source")
	}
	if clone.ParentTemplateID != "" {
		t.Errorf("expected the clone to have no parent, got %q", clone.ParentTemplateID)
	}
	if clone.Name != "clone" {
		t.Errorf("expected the clone's name to be the requested new name, got %q", clone.Name)
	}
	dag, _ := domain.ParseDAG(clone.DAGJSON)
	if len(dag.Steps) != 1 || dag.Steps[0].ID != "s1" {
		t.Errorf("expected the clone to carry the source's own steps, got %+v", dag.Steps)
	}

	stored, err := repo.GetTemplate(ctx, "tenant-1", clone.ID)
	if err != nil {
		t.Fatalf("expected the clone to be persisted: %v", err)
	}
	if stored.ID != clone.ID {
		t.Errorf("expected the persisted clone to match the returned value, got %+v", stored)
	}
}

// TestCloneTemplate_UsesResolvedDAGNotRawSourceDAG proves Clone uses
// ResolveTemplate's effective output, not the source row's raw dag_json —
// a personal source that only inherits from a parent (empty own dag_json)
// must clone the PARENT's resolved steps.
func TestCloneTemplate_UsesResolvedDAGNotRawSourceDAG(t *testing.T) {
	repo := newFakeTemplateRepository()
	createUC := NewCreateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	parent, err := createUC.Execute(ctx, CreateTemplateInput{Name: "parent", Scope: "company", DAGJSON: `{"steps":[{"id":"s1","type":"webhook"}]}`})
	if err != nil {
		t.Fatalf("creating parent: %v", err)
	}
	source, err := createUC.Execute(ctx, CreateTemplateInput{Name: "source", Scope: "personal", ParentTemplateID: parent.ID, DAGJSON: `{"steps":[]}`})
	if err != nil {
		t.Fatalf("creating source: %v", err)
	}

	uc := NewCloneTemplate(NewResolveTemplate(repo), repo)
	clone, err := uc.Execute(ctx, CloneTemplateInput{SourceTemplateID: source.ID, NewName: "clone", Scope: "personal"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dag, _ := domain.ParseDAG(clone.DAGJSON)
	if len(dag.Steps) != 1 || dag.Steps[0].ID != "s1" {
		t.Fatalf("expected the clone to carry the PARENT's resolved steps, got %+v", dag.Steps)
	}
}

// TestCloneTemplate_MutateSourceAfterCloneLeavesCloneUnaffected proves the
// severed-parent, copy-not-reference semantics: mutating the source after
// cloning must not affect the already-created clone.
func TestCloneTemplate_MutateSourceAfterCloneLeavesCloneUnaffected(t *testing.T) {
	repo := newFakeTemplateRepository()
	createUC := NewCreateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	source, err := createUC.Execute(ctx, CreateTemplateInput{Name: "source", Scope: "personal", DAGJSON: `{"steps":[{"id":"s1","type":"webhook"}]}`})
	if err != nil {
		t.Fatalf("creating source: %v", err)
	}

	uc := NewCloneTemplate(NewResolveTemplate(repo), repo)
	clone, err := uc.Execute(ctx, CloneTemplateInput{SourceTemplateID: source.ID, NewName: "clone", Scope: "personal"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Mutate the source's stored row directly (simulating a later
	// UpdateTemplate on the source).
	source.DAGJSON = `{"steps":[{"id":"s1","type":"webhook"},{"id":"s2","type":"webhook"}]}`
	if _, err := repo.Update(ctx, source, source.Version, true); err != nil {
		t.Fatalf("updating source: %v", err)
	}

	stillStoredClone, err := repo.GetTemplate(ctx, "tenant-1", clone.ID)
	if err != nil {
		t.Fatalf("fetching clone: %v", err)
	}
	dag, _ := domain.ParseDAG(stillStoredClone.DAGJSON)
	if len(dag.Steps) != 1 {
		t.Errorf("expected the clone to be unaffected by the source's later mutation, got %+v", dag.Steps)
	}
}

func TestCloneTemplate_NonexistentSourcePropagatesNotFound(t *testing.T) {
	repo := newFakeTemplateRepository()
	uc := NewCloneTemplate(NewResolveTemplate(repo), repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, CloneTemplateInput{SourceTemplateID: "does-not-exist", NewName: "clone", Scope: "personal"})
	if err == nil {
		t.Fatal("expected a not-found error propagated from ResolveTemplate")
	}
}

func TestCloneTemplate_RequiresTenantContext(t *testing.T) {
	repo := newFakeTemplateRepository()
	uc := NewCloneTemplate(NewResolveTemplate(repo), repo)

	_, err := uc.Execute(context.Background(), CloneTemplateInput{SourceTemplateID: "x", NewName: "clone", Scope: "personal"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
