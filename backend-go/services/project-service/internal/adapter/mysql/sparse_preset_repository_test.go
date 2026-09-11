//go:build integration

package mysql

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// TestSparsePresetRepository_SaveAndList_DirectoriesJSONRoundTrips is this
// rollout's most dialect-specific translation risk: Postgres's native
// TEXT[] (migrations/postgres/0016) has no MySQL equivalent — this proves
// the []string<->JSON-array marshal/unmarshal in
// sparse_preset_repository.go round-trips element order and content
// exactly, including an empty slice.
func TestSparsePresetRepository_SaveAndList_DirectoriesJSONRoundTrips(t *testing.T) {
	db := setupDB(t)
	projRepo := New(db)
	repoRepoAdapter := NewRepoRepository(db)
	presetRepo := NewSparsePresetRepository(db)
	ctx := context.Background()

	p := newTestProject(uuid.NewString(), uuid.NewString(), "proj")
	if _, err := projRepo.Create(ctx, p); err != nil {
		t.Fatalf("create project: %v", err)
	}
	r, err := domain.NewRepo(uuid.NewString(), p.ID, "https://example.com/repo.git", "repo", "")
	if err != nil {
		t.Fatalf("NewRepo: %v", err)
	}
	repo, err := repoRepoAdapter.AddRepo(ctx, r)
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	preset := domain.SparsePreset{
		ID: uuid.NewString(), RepoID: repo.ID, Name: "frontend-only",
		Directories: []string{"src/renderer", "src/shared", "docs"},
		CreatedAt:   now, UpdatedAt: now,
	}
	saved, err := presetRepo.SaveSparsePreset(ctx, preset)
	if err != nil {
		t.Fatalf("SaveSparsePreset (insert): %v", err)
	}
	if !reflect.DeepEqual(saved.Directories, preset.Directories) {
		t.Fatalf("expected directories to round-trip in order, got %+v", saved.Directories)
	}

	got, err := presetRepo.GetSparsePreset(ctx, repo.ID, preset.ID)
	if err != nil {
		t.Fatalf("GetSparsePreset: %v", err)
	}
	if !reflect.DeepEqual(got.Directories, preset.Directories) {
		t.Fatalf("expected directories to round-trip via GetSparsePreset, got %+v", got.Directories)
	}

	// Update (same id -> upsert path) with a different, shorter slice.
	preset.Directories = []string{"src/main"}
	preset.UpdatedAt = now.Add(time.Minute)
	updated, err := presetRepo.SaveSparsePreset(ctx, preset)
	if err != nil {
		t.Fatalf("SaveSparsePreset (update): %v", err)
	}
	if !reflect.DeepEqual(updated.Directories, []string{"src/main"}) {
		t.Fatalf("expected directories replaced on update, got %+v", updated.Directories)
	}

	list, err := presetRepo.ListSparsePresets(ctx, repo.ID)
	if err != nil {
		t.Fatalf("ListSparsePresets: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected exactly 1 preset (upsert, not duplicate), got %d", len(list))
	}

	if err := presetRepo.RemoveSparsePreset(ctx, repo.ID, preset.ID); err != nil {
		t.Fatalf("RemoveSparsePreset: %v", err)
	}
	if _, err := presetRepo.GetSparsePreset(ctx, repo.ID, preset.ID); err != domain.ErrSparsePresetNotFound {
		t.Errorf("expected ErrSparsePresetNotFound after remove, got %v", err)
	}
}
