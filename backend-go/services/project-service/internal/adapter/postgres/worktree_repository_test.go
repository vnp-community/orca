//go:build integration

package postgres

import (
	"context"
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

func TestWorktreeRepository_FindWorktreeByIdempotencyKey_RoundTrips(t *testing.T) {
	pool := setupPool(t)
	projectRepo := New(pool)
	repoRepo := NewRepoRepository(pool)
	worktreeRepo := NewWorktreeRepository(pool)
	ctx := context.Background()

	project, repo := setupRepoForWorktree(t, projectRepo, repoRepo)
	key := "sha256-abc"
	created, err := worktreeRepo.RecordWorktreeCreated(ctx, domain.Worktree{
		ID: uuid.NewString(), ProjectID: project.ID, RepoID: repo.ID, Path: "/srv/w1", Branch: "main", Active: true,
		IdempotencyKey: &key,
	})
	if err != nil {
		t.Fatalf("record worktree created: %v", err)
	}

	found, ok, err := worktreeRepo.FindWorktreeByIdempotencyKey(ctx, project.ID, key)
	if err != nil {
		t.Fatalf("find worktree by idempotency key: %v", err)
	}
	if !ok {
		t.Fatal("expected found=true")
	}
	if found.ID != created.ID {
		t.Errorf("expected to find worktree %q, got %q", created.ID, found.ID)
	}
}

func TestWorktreeRepository_FindWorktreeByIdempotencyKey_NoMatchReturnsFoundFalse(t *testing.T) {
	pool := setupPool(t)
	projectRepo := New(pool)
	repoRepo := NewRepoRepository(pool)
	worktreeRepo := NewWorktreeRepository(pool)
	ctx := context.Background()

	project, _ := setupRepoForWorktree(t, projectRepo, repoRepo)

	_, ok, err := worktreeRepo.FindWorktreeByIdempotencyKey(ctx, project.ID, "no-such-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected found=false for a non-matching idempotency key")
	}
}
