//go:build integration

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func setupRepoForWorktree(t *testing.T, projectRepo *Repository, repoRepo *RepoRepository) (domain.Project, domain.Repo) {
	t.Helper()
	ctx := context.Background()
	project := setupProjectForRepo(t, projectRepo)
	repo, err := repoRepo.AddRepo(ctx, domain.Repo{ID: uuid.NewString(), ProjectID: project.ID, URL: "https://github.com/org/repo"})
	if err != nil {
		t.Fatalf("adding repo: %v", err)
	}
	return project, repo
}

func TestWorktreeRepository_RecordWorktreeCreated_RoundTrips(t *testing.T) {
	pool := setupPool(t)
	projectRepo := New(pool)
	repoRepo := NewRepoRepository(pool)
	worktreeRepo := NewWorktreeRepository(pool)
	ctx := context.Background()

	project, repo := setupRepoForWorktree(t, projectRepo, repoRepo)

	wt, err := worktreeRepo.RecordWorktreeCreated(ctx, domain.Worktree{
		ID: uuid.NewString(), ProjectID: project.ID, RepoID: repo.ID, Path: "/srv/w1", Branch: "main", Active: true,
	})
	if err != nil {
		t.Fatalf("record worktree created: %v", err)
	}
	if !wt.Active {
		t.Error("expected worktree to be active")
	}
}

func TestWorktreeRepository_SetWorktreeActivation(t *testing.T) {
	pool := setupPool(t)
	projectRepo := New(pool)
	repoRepo := NewRepoRepository(pool)
	worktreeRepo := NewWorktreeRepository(pool)
	ctx := context.Background()

	project, repo := setupRepoForWorktree(t, projectRepo, repoRepo)
	wt, _ := worktreeRepo.RecordWorktreeCreated(ctx, domain.Worktree{
		ID: uuid.NewString(), ProjectID: project.ID, RepoID: repo.ID, Path: "/srv/w1", Branch: "main", Active: true,
	})

	updated, err := worktreeRepo.SetWorktreeActivation(ctx, wt.ID, false)
	if err != nil {
		t.Fatalf("set worktree activation: %v", err)
	}
	if updated.Active {
		t.Error("expected Active=false")
	}
}

func TestWorktreeRepository_RenameWorktree(t *testing.T) {
	pool := setupPool(t)
	projectRepo := New(pool)
	repoRepo := NewRepoRepository(pool)
	worktreeRepo := NewWorktreeRepository(pool)
	ctx := context.Background()

	project, repo := setupRepoForWorktree(t, projectRepo, repoRepo)
	wt, _ := worktreeRepo.RecordWorktreeCreated(ctx, domain.Worktree{
		ID: uuid.NewString(), ProjectID: project.ID, RepoID: repo.ID, Path: "/srv/w1", Branch: "old", Active: true,
	})

	updated, err := worktreeRepo.RenameWorktree(ctx, wt.ID, "new")
	if err != nil {
		t.Fatalf("rename worktree: %v", err)
	}
	if updated.Branch != "new" {
		t.Errorf("expected Branch=new, got %q", updated.Branch)
	}
}

func TestWorktreeRepository_RecordWorktreeRemoved_HardDeletes(t *testing.T) {
	pool := setupPool(t)
	projectRepo := New(pool)
	repoRepo := NewRepoRepository(pool)
	worktreeRepo := NewWorktreeRepository(pool)
	ctx := context.Background()

	project, repo := setupRepoForWorktree(t, projectRepo, repoRepo)
	wt, _ := worktreeRepo.RecordWorktreeCreated(ctx, domain.Worktree{
		ID: uuid.NewString(), ProjectID: project.ID, RepoID: repo.ID, Path: "/srv/w1", Branch: "main", Active: true,
	})

	if err := worktreeRepo.RecordWorktreeRemoved(ctx, wt.ID); err != nil {
		t.Fatalf("record worktree removed: %v", err)
	}

	worktrees, err := worktreeRepo.ListWorktrees(ctx, project.ID)
	if err != nil {
		t.Fatalf("list worktrees: %v", err)
	}
	if len(worktrees) != 0 {
		t.Errorf("expected worktree to be hard-deleted, got %+v", worktrees)
	}
}

func TestWorktreeRepository_ListLineage_ReturnsOnlyWorktreesWithCapturedParent(t *testing.T) {
	pool := setupPool(t)
	projectRepo := New(pool)
	repoRepo := NewRepoRepository(pool)
	worktreeRepo := NewWorktreeRepository(pool)
	ctx := context.Background()

	project, repo := setupRepoForWorktree(t, projectRepo, repoRepo)
	parent, err := worktreeRepo.RecordWorktreeCreated(ctx, domain.Worktree{
		ID: uuid.NewString(), ProjectID: project.ID, RepoID: repo.ID, Path: "/srv/w1", Branch: "main", Active: true,
	})
	if err != nil {
		t.Fatalf("record parent worktree: %v", err)
	}

	origin := "cli"
	explicit := "explicit"
	child, err := worktreeRepo.RecordWorktreeCreated(ctx, domain.Worktree{
		ID: uuid.NewString(), ProjectID: project.ID, RepoID: repo.ID, Path: "/srv/w2", Branch: "feature", Active: true,
		ParentWorktreeID: &parent.ID, Origin: &origin, CaptureConfidence: &explicit,
	})
	if err != nil {
		t.Fatalf("record child worktree: %v", err)
	}

	lineage, err := worktreeRepo.ListLineage(ctx)
	if err != nil {
		t.Fatalf("list lineage: %v", err)
	}
	if len(lineage) != 1 || lineage[0].ID != child.ID {
		t.Fatalf("expected only the child worktree, got %+v", lineage)
	}
	if lineage[0].ParentWorktreeID == nil || *lineage[0].ParentWorktreeID != parent.ID {
		t.Errorf("expected ParentWorktreeID=%q, got %v", parent.ID, lineage[0].ParentWorktreeID)
	}
	if lineage[0].CaptureConfidence == nil || *lineage[0].CaptureConfidence != "explicit" {
		t.Errorf("expected CaptureConfidence=explicit, got %v", lineage[0].CaptureConfidence)
	}
}

func TestWorktreeRepository_CascadesOnProjectDelete(t *testing.T) {
	pool := setupPool(t)
	projectRepo := New(pool)
	repoRepo := NewRepoRepository(pool)
	worktreeRepo := NewWorktreeRepository(pool)
	ctx := context.Background()

	project, repo := setupRepoForWorktree(t, projectRepo, repoRepo)
	_, _ = worktreeRepo.RecordWorktreeCreated(ctx, domain.Worktree{
		ID: uuid.NewString(), ProjectID: project.ID, RepoID: repo.ID, Path: "/srv/w1", Branch: "main", Active: true,
	})

	if err := projectRepo.DeleteProject(ctx, project.TenantID, project.ID); err != nil {
		t.Fatalf("delete project: %v", err)
	}

	worktrees, err := worktreeRepo.ListWorktrees(ctx, project.ID)
	if err != nil {
		t.Fatalf("list worktrees: %v", err)
	}
	if len(worktrees) != 0 {
		t.Errorf("expected worktrees to cascade-delete with project, got %+v", worktrees)
	}
}

// TestWorktreeRepository_UpdateWorktreeMeta_DefaultsToEmptyObject verifies
// the metadata column defaults to "{}" (never NULL) for a worktree recorded
// before UpdateWorktreeMeta was ever called for it.
func TestWorktreeRepository_UpdateWorktreeMeta_DefaultsToEmptyObject(t *testing.T) {
	pool := setupPool(t)
	projectRepo := New(pool)
	repoRepo := NewRepoRepository(pool)
	worktreeRepo := NewWorktreeRepository(pool)
	ctx := context.Background()

	project, repo := setupRepoForWorktree(t, projectRepo, repoRepo)
	wt, err := worktreeRepo.RecordWorktreeCreated(ctx, domain.Worktree{
		ID: uuid.NewString(), ProjectID: project.ID, RepoID: repo.ID, Path: "/srv/w1", Branch: "main", Active: true,
	})
	if err != nil {
		t.Fatalf("record worktree created: %v", err)
	}
	if string(wt.Metadata) != "{}" {
		t.Errorf("expected metadata to default to {}, got %q", string(wt.Metadata))
	}
}

// TestWorktreeRepository_UpdateWorktreeMeta_ShallowMergesAndClearsWithNull
// verifies the real Postgres jsonb `||` merge: a second patch adds a new
// key, overwrites an existing key, leaves an untouched key alone, and an
// explicit JSON null overwrites (clears) rather than removes the key.
func TestWorktreeRepository_UpdateWorktreeMeta_ShallowMergesAndClearsWithNull(t *testing.T) {
	pool := setupPool(t)
	projectRepo := New(pool)
	repoRepo := NewRepoRepository(pool)
	worktreeRepo := NewWorktreeRepository(pool)
	ctx := context.Background()

	project, repo := setupRepoForWorktree(t, projectRepo, repoRepo)
	wt, _ := worktreeRepo.RecordWorktreeCreated(ctx, domain.Worktree{
		ID: uuid.NewString(), ProjectID: project.ID, RepoID: repo.ID, Path: "/srv/w1", Branch: "main", Active: true,
	})

	first, err := worktreeRepo.UpdateWorktreeMeta(ctx, wt.ID, []byte(`{"displayName":"My Worktree","isPinned":true}`))
	if err != nil {
		t.Fatalf("first UpdateWorktreeMeta: %v", err)
	}
	var m1 map[string]any
	if err := json.Unmarshal(first.Metadata, &m1); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if m1["displayName"] != "My Worktree" || m1["isPinned"] != true {
		t.Fatalf("unexpected metadata after first patch: %+v", m1)
	}

	second, err := worktreeRepo.UpdateWorktreeMeta(ctx, wt.ID, []byte(`{"comment":"hello","isPinned":null}`))
	if err != nil {
		t.Fatalf("second UpdateWorktreeMeta: %v", err)
	}
	var m2 map[string]any
	if err := json.Unmarshal(second.Metadata, &m2); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if m2["displayName"] != "My Worktree" {
		t.Errorf("expected displayName untouched by the second patch, got %+v", m2)
	}
	if m2["comment"] != "hello" {
		t.Errorf("expected comment added by the second patch, got %+v", m2)
	}
	if v, ok := m2["isPinned"]; !ok || v != nil {
		t.Errorf("expected isPinned cleared to JSON null (key still present, value nil), got %+v (present=%v)", v, ok)
	}
}

func TestWorktreeRepository_UpdateWorktreeMeta_NotFound(t *testing.T) {
	pool := setupPool(t)
	worktreeRepo := NewWorktreeRepository(pool)

	_, err := worktreeRepo.UpdateWorktreeMeta(context.Background(), uuid.NewString(), []byte(`{}`))
	if !errors.Is(err, domain.ErrWorktreeNotFound) {
		t.Fatalf("expected ErrWorktreeNotFound, got %v", err)
	}
}

func TestWorktreeRepository_SetWorktreeLineage_SetsAndClearsParent(t *testing.T) {
	pool := setupPool(t)
	projectRepo := New(pool)
	repoRepo := NewRepoRepository(pool)
	worktreeRepo := NewWorktreeRepository(pool)
	ctx := context.Background()

	project, repo := setupRepoForWorktree(t, projectRepo, repoRepo)
	parent, _ := worktreeRepo.RecordWorktreeCreated(ctx, domain.Worktree{
		ID: uuid.NewString(), ProjectID: project.ID, RepoID: repo.ID, Path: "/srv/parent", Branch: "main", Active: true,
	})
	child, _ := worktreeRepo.RecordWorktreeCreated(ctx, domain.Worktree{
		ID: uuid.NewString(), ProjectID: project.ID, RepoID: repo.ID, Path: "/srv/child", Branch: "feature", Active: true,
	})

	updated, err := worktreeRepo.SetWorktreeLineage(ctx, child.ID, &parent.ID)
	if err != nil {
		t.Fatalf("set worktree lineage: %v", err)
	}
	if updated.ParentWorktreeID == nil || *updated.ParentWorktreeID != parent.ID {
		t.Fatalf("expected ParentWorktreeID=%q, got %v", parent.ID, updated.ParentWorktreeID)
	}
	if updated.CaptureConfidence == nil || *updated.CaptureConfidence != "explicit" {
		t.Errorf("expected CaptureConfidence=explicit after setting a parent, got %v", updated.CaptureConfidence)
	}

	cleared, err := worktreeRepo.SetWorktreeLineage(ctx, child.ID, nil)
	if err != nil {
		t.Fatalf("clear worktree lineage: %v", err)
	}
	if cleared.ParentWorktreeID != nil {
		t.Errorf("expected ParentWorktreeID cleared, got %v", cleared.ParentWorktreeID)
	}
	if cleared.CaptureConfidence != nil {
		t.Errorf("expected CaptureConfidence cleared alongside the parent, got %v", cleared.CaptureConfidence)
	}
}
