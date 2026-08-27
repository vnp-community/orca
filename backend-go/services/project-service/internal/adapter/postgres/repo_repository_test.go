//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func setupProjectForRepo(t *testing.T, repo *Repository) domain.Project {
	t.Helper()
	p := newTestProject(uuid.NewString(), uuid.NewString(), "proj")
	created, err := repo.Create(context.Background(), p)
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}
	return created
}

func TestRepoRepository_AddRepo_AssignsSequentialPositions(t *testing.T) {
	pool := setupPool(t)
	projectRepo := New(pool)
	repoRepo := NewRepoRepository(pool)
	ctx := context.Background()

	project := setupProjectForRepo(t, projectRepo)

	first, err := repoRepo.AddRepo(ctx, domain.Repo{ID: uuid.NewString(), ProjectID: project.ID, URL: "https://github.com/org/one"})
	if err != nil {
		t.Fatalf("add first repo: %v", err)
	}
	if first.Position != 0 {
		t.Errorf("expected first repo at position 0, got %d", first.Position)
	}

	second, err := repoRepo.AddRepo(ctx, domain.Repo{ID: uuid.NewString(), ProjectID: project.ID, URL: "https://github.com/org/two"})
	if err != nil {
		t.Fatalf("add second repo: %v", err)
	}
	if second.Position != 1 {
		t.Errorf("expected second repo at position 1, got %d", second.Position)
	}
}

func TestRepoRepository_ListRepos_OrderedByPosition(t *testing.T) {
	pool := setupPool(t)
	projectRepo := New(pool)
	repoRepo := NewRepoRepository(pool)
	ctx := context.Background()

	project := setupProjectForRepo(t, projectRepo)
	_, _ = repoRepo.AddRepo(ctx, domain.Repo{ID: uuid.NewString(), ProjectID: project.ID, URL: "https://github.com/org/one"})
	_, _ = repoRepo.AddRepo(ctx, domain.Repo{ID: uuid.NewString(), ProjectID: project.ID, URL: "https://github.com/org/two"})

	repos, err := repoRepo.ListRepos(ctx, project.ID)
	if err != nil {
		t.Fatalf("list repos: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	if repos[0].Position > repos[1].Position {
		t.Errorf("expected repos ordered by position, got %+v", repos)
	}
}

func TestRepoRepository_ReorderRepos_RewritesPositions(t *testing.T) {
	pool := setupPool(t)
	projectRepo := New(pool)
	repoRepo := NewRepoRepository(pool)
	ctx := context.Background()

	project := setupProjectForRepo(t, projectRepo)
	r1, _ := repoRepo.AddRepo(ctx, domain.Repo{ID: uuid.NewString(), ProjectID: project.ID, URL: "https://github.com/org/one"})
	r2, _ := repoRepo.AddRepo(ctx, domain.Repo{ID: uuid.NewString(), ProjectID: project.ID, URL: "https://github.com/org/two"})

	if err := repoRepo.ReorderRepos(ctx, project.ID, []string{r2.ID, r1.ID}); err != nil {
		t.Fatalf("reorder repos: %v", err)
	}

	repos, err := repoRepo.ListRepos(ctx, project.ID)
	if err != nil {
		t.Fatalf("list repos: %v", err)
	}
	if repos[0].ID != r2.ID {
		t.Errorf("expected r2 first after reorder, got %+v", repos)
	}
}

func TestRepoRepository_RemoveRepo_LeavesGapNotContiguous(t *testing.T) {
	pool := setupPool(t)
	projectRepo := New(pool)
	repoRepo := NewRepoRepository(pool)
	ctx := context.Background()

	project := setupProjectForRepo(t, projectRepo)
	r1, _ := repoRepo.AddRepo(ctx, domain.Repo{ID: uuid.NewString(), ProjectID: project.ID, URL: "https://github.com/org/one"})
	r2, _ := repoRepo.AddRepo(ctx, domain.Repo{ID: uuid.NewString(), ProjectID: project.ID, URL: "https://github.com/org/two"})

	if err := repoRepo.RemoveRepo(ctx, r1.ID); err != nil {
		t.Fatalf("remove repo: %v", err)
	}

	repos, err := repoRepo.ListRepos(ctx, project.ID)
	if err != nil {
		t.Fatalf("list repos: %v", err)
	}
	if len(repos) != 1 || repos[0].ID != r2.ID {
		t.Errorf("expected only r2 to remain, got %+v", repos)
	}
	if repos[0].Position != 1 {
		t.Errorf("expected r2's position to remain 1 (gap left, not renumbered), got %d", repos[0].Position)
	}
}

func TestRepoRepository_RemoveRepo_NotFound(t *testing.T) {
	pool := setupPool(t)
	repoRepo := NewRepoRepository(pool)

	if err := repoRepo.RemoveRepo(context.Background(), uuid.NewString()); err != domain.ErrRepoNotFound {
		t.Errorf("expected ErrRepoNotFound, got %v", err)
	}
}
