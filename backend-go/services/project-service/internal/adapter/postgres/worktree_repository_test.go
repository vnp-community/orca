//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

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

	worktrees, err := worktreeRepo.ListWorktrees(ctx, project.ID, nil, nil)
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

	worktrees, err := worktreeRepo.ListWorktrees(ctx, project.ID, nil, nil)
	if err != nil {
		t.Fatalf("list worktrees: %v", err)
	}
	if len(worktrees) != 0 {
		t.Errorf("expected worktrees to cascade-delete with project, got %+v", worktrees)
	}
}

func TestWorktreeRepository_ListWorktrees_FiltersByStatusAndOlderThan(t *testing.T) {
	pool := setupPool(t)
	projectRepo := New(pool)
	repoRepo := NewRepoRepository(pool)
	worktreeRepo := NewWorktreeRepository(pool)
	ctx := context.Background()

	project, repo := setupRepoForWorktree(t, projectRepo, repoRepo)

	_, err := worktreeRepo.RecordWorktreeCreated(ctx, domain.Worktree{
		ID: uuid.NewString(), ProjectID: project.ID, RepoID: repo.ID, Path: "/srv/w1", Branch: "main", Active: true, Status: domain.WorktreeStatusActive,
	})
	if err != nil {
		t.Fatalf("record active worktree: %v", err)
	}
	completed, err := worktreeRepo.RecordWorktreeCreated(ctx, domain.Worktree{
		ID: uuid.NewString(), ProjectID: project.ID, RepoID: repo.ID, Path: "/srv/w2", Branch: "feature", Active: true, Status: domain.WorktreeStatusCompleted,
	})
	if err != nil {
		t.Fatalf("record completed worktree: %v", err)
	}

	// Unfiltered call (both nil) sees every worktree — regression guard for
	// every pre-existing caller's behavior.
	all, err := worktreeRepo.ListWorktrees(ctx, project.ID, nil, nil)
	if err != nil {
		t.Fatalf("list unfiltered: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 worktrees unfiltered, got %d", len(all))
	}

	completedOnly, err := worktreeRepo.ListWorktrees(ctx, project.ID, []string{"completed"}, nil)
	if err != nil {
		t.Fatalf("list status_in=completed: %v", err)
	}
	if len(completedOnly) != 1 || completedOnly[0].ID != completed.ID {
		t.Fatalf("expected only the completed worktree, got %+v", completedOnly)
	}

	future := time.Now().UTC().Add(time.Hour)
	olderThanFuture, err := worktreeRepo.ListWorktrees(ctx, project.ID, nil, &future)
	if err != nil {
		t.Fatalf("list older_than=future: %v", err)
	}
	if len(olderThanFuture) != 2 {
		t.Fatalf("expected both worktrees to be older than a future cutoff, got %d", len(olderThanFuture))
	}

	past := time.Now().UTC().Add(-time.Hour)
	olderThanPast, err := worktreeRepo.ListWorktrees(ctx, project.ID, nil, &past)
	if err != nil {
		t.Fatalf("list older_than=past: %v", err)
	}
	if len(olderThanPast) != 0 {
		t.Fatalf("expected no worktrees older than a past cutoff, got %d", len(olderThanPast))
	}
}
