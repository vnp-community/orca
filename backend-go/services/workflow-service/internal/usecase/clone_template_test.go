package usecase

import (
	"context"
	"testing"
)

func TestCloneTemplate_InheritModeSourceSnapshotsResolvedSteps(t *testing.T) {
	repo := newFakeTemplateRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")

	// Inherit-mode source: parent has the real steps, the source itself is
	// an empty passthrough that only inherits.
	parent := mustNewTemplate(t, repo, "parent", "tenant-1", "parent",
		`{"steps":[{"id":"s1","type":"webhook"}]}`, "")
	source := mustNewTemplate(t, repo, "source", "tenant-1", "source", `{"steps":[]}`, parent.ID)

	resolveUC := NewResolveTemplate(repo)
	cloneUC := NewCloneTemplate(resolveUC, repo)

	clone, err := cloneUC.Execute(ctx, CloneTemplateInput{
		SourceTemplateID: source.ID,
		Name:             "my clone",
		Description:      "a standalone copy",
		Tags:             []string{"cloned"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !jsonEqualSteps(t, clone.DAGJSON, `{"steps":[{"id":"s1","type":"webhook"}]}`) {
		t.Errorf("expected clone's dag_json to be the source's RESOLVED steps, got %s", clone.DAGJSON)
	}
	if clone.ParentTemplateID != "" {
		t.Errorf("expected a disconnected root template, got parent=%q", clone.ParentTemplateID)
	}
	if clone.ClonedFromTemplateID != source.ID {
		t.Errorf("expected ClonedFromTemplateID=%q, got %q", source.ID, clone.ClonedFromTemplateID)
	}
	if clone.Description != "a standalone copy" {
		t.Errorf("expected Description set, got %q", clone.Description)
	}
	if len(clone.Tags) != 1 || clone.Tags[0] != "cloned" {
		t.Errorf("expected Tags=[cloned], got %v", clone.Tags)
	}

	// Persisted for real, under a fresh id distinct from the source.
	persisted, err := repo.GetTemplate(ctx, "tenant-1", clone.ID)
	if err != nil {
		t.Fatalf("expected clone to be persisted: %v", err)
	}
	if persisted.ID == source.ID {
		t.Error("expected the clone to have its own id, distinct from the source")
	}
}

func TestCloneTemplate_UpdateOnCloneNeverTouchesOriginal(t *testing.T) {
	repo := newFakeTemplateRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")

	source := mustNewTemplate(t, repo, "source", "tenant-1", "source",
		`{"steps":[{"id":"s1","type":"webhook"}]}`, "")

	resolveUC := NewResolveTemplate(repo)
	cloneUC := NewCloneTemplate(resolveUC, repo)
	updateUC := NewUpdateTemplate(repo)

	clone, err := cloneUC.Execute(ctx, CloneTemplateInput{SourceTemplateID: source.ID, Name: "clone"})
	if err != nil {
		t.Fatalf("cloning: %v", err)
	}

	if _, err := updateUC.Execute(ctx, UpdateTemplateInput{
		ID:              clone.ID,
		Name:            "clone-renamed",
		DAGJSON:         `{"steps":[{"id":"s2","type":"shell"}]}`,
		Scope:           clone.Scope,
		ExpectedVersion: clone.Version,
	}); err != nil {
		t.Fatalf("updating clone: %v", err)
	}

	original, err := repo.GetTemplate(ctx, "tenant-1", source.ID)
	if err != nil {
		t.Fatalf("fetching original: %v", err)
	}
	if !jsonEqualSteps(t, original.DAGJSON, `{"steps":[{"id":"s1","type":"webhook"}]}`) {
		t.Errorf("expected original untouched by the clone's update, got %s", original.DAGJSON)
	}
	if original.Name != "source" {
		t.Errorf("expected original's name untouched, got %q", original.Name)
	}
}

func TestCloneTemplate_RequiresSourceAndName(t *testing.T) {
	repo := newFakeTemplateRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")
	resolveUC := NewResolveTemplate(repo)
	cloneUC := NewCloneTemplate(resolveUC, repo)

	if _, err := cloneUC.Execute(ctx, CloneTemplateInput{Name: "x"}); err == nil {
		t.Error("expected an error for an empty source_template_id")
	}

	source := mustNewTemplate(t, repo, "source", "tenant-1", "source", `{"steps":[]}`, "")
	if _, err := cloneUC.Execute(ctx, CloneTemplateInput{SourceTemplateID: source.ID}); err == nil {
		t.Error("expected an error for an empty name")
	}
}

func TestCloneTemplate_UnknownSourceNotFound(t *testing.T) {
	repo := newFakeTemplateRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")
	resolveUC := NewResolveTemplate(repo)
	cloneUC := NewCloneTemplate(resolveUC, repo)

	_, err := cloneUC.Execute(ctx, CloneTemplateInput{SourceTemplateID: "does-not-exist", Name: "x"})
	if err == nil {
		t.Fatal("expected a not-found error for an unknown source")
	}
}
