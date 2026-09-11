//go:build integration

package mysql

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// newTestProjectDB seeds a minimal project via the shared Repository so
// repo/worktree/etc. tests have a valid project_id FK target — every table
// in this schema ultimately FKs back to projects, per migrations/mysql/0001.
func newTestProjectDB(t *testing.T, projRepo *Repository, tenantID string) domain.Project {
	t.Helper()
	ctx := context.Background()
	p := newTestProject(uuid.NewString(), tenantID, "seed-project")
	if _, err := projRepo.Create(ctx, p); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return p
}

func TestRepoRepository_AddListReorderRemove_RoundTrip(t *testing.T) {
	db := setupDB(t)
	projRepo := New(db)
	repoRepo := NewRepoRepository(db)
	ctx := context.Background()

	p := newTestProjectDB(t, projRepo, uuid.NewString())

	r1, err := domain.NewRepo(uuid.NewString(), p.ID, "https://example.com/one.git", "one", "")
	if err != nil {
		t.Fatalf("NewRepo: %v", err)
	}
	added1, err := repoRepo.AddRepo(ctx, r1)
	if err != nil {
		t.Fatalf("AddRepo 1: %v", err)
	}
	if added1.Position != 0 {
		t.Errorf("expected first repo position 0, got %d", added1.Position)
	}

	r2, err := domain.NewRepo(uuid.NewString(), p.ID, "https://example.com/two.git", "two", "")
	if err != nil {
		t.Fatalf("NewRepo: %v", err)
	}
	added2, err := repoRepo.AddRepo(ctx, r2)
	if err != nil {
		t.Fatalf("AddRepo 2: %v", err)
	}
	if added2.Position != 1 {
		t.Errorf("expected second repo position 1 (MAX(position)+1), got %d", added2.Position)
	}

	list, err := repoRepo.ListRepos(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(list))
	}

	if err := repoRepo.ReorderRepos(ctx, p.ID, []string{added2.ID, added1.ID}); err != nil {
		t.Fatalf("ReorderRepos: %v", err)
	}
	reordered, err := repoRepo.ListRepos(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListRepos after reorder: %v", err)
	}
	if reordered[0].ID != added2.ID || reordered[0].Position != 0 {
		t.Errorf("expected repo two first after reorder, got %+v", reordered)
	}

	if err := repoRepo.RemoveRepo(ctx, added1.ID); err != nil {
		t.Fatalf("RemoveRepo: %v", err)
	}
	if _, err := repoRepo.GetRepo(ctx, added1.ID); err != domain.ErrRepoNotFound {
		t.Errorf("expected ErrRepoNotFound after remove, got %v", err)
	}
}

// TestRepoRepository_Update_HookSettingsJSONRoundTrips exercises the
// JSONB->JSON hook_settings translation (migrations/mysql/0018) — an
// opaque JSON blob this service never parses, stored/returned verbatim.
func TestRepoRepository_Update_HookSettingsJSONRoundTrips(t *testing.T) {
	db := setupDB(t)
	projRepo := New(db)
	repoRepo := NewRepoRepository(db)
	ctx := context.Background()

	p := newTestProjectDB(t, projRepo, uuid.NewString())
	r, err := domain.NewRepo(uuid.NewString(), p.ID, "https://example.com/repo.git", "repo", "")
	if err != nil {
		t.Fatalf("NewRepo: %v", err)
	}
	added, err := repoRepo.AddRepo(ctx, r)
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}

	added.HookSettings = `{"setupScript":"npm install"}`
	updated, err := repoRepo.Update(ctx, added)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.HookSettings != `{"setupScript": "npm install"}` && updated.HookSettings != `{"setupScript":"npm install"}` {
		t.Errorf("expected hook_settings to round-trip (allowing MySQL JSON re-serialization whitespace), got %q", updated.HookSettings)
	}
}

// TestRepoRepository_ReassignProject_ClearsRepoMembersAndDetectsRace mirrors
// postgres.RepoRepository.ReassignProject's TOCTOU guard: a stale
// fromProjectID must return ErrRepoProjectChanged (repo exists, just moved
// already), not ErrRepoNotFound.
func TestRepoRepository_ReassignProject_ClearsRepoMembersAndDetectsRace(t *testing.T) {
	db := setupDB(t)
	projRepo := New(db)
	repoRepo := NewRepoRepository(db)
	ctx := context.Background()

	tenantID := uuid.NewString()
	src := newTestProjectDB(t, projRepo, tenantID)
	dst := newTestProjectDB(t, projRepo, tenantID)

	r, err := domain.NewRepo(uuid.NewString(), src.ID, "https://example.com/repo.git", "repo", "")
	if err != nil {
		t.Fatalf("NewRepo: %v", err)
	}
	added, err := repoRepo.AddRepo(ctx, r)
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}

	member, err := domain.NewRepoMember(added.ID, uuid.NewString(), domain.RepoRoleDeveloper)
	if err != nil {
		t.Fatalf("NewRepoMember: %v", err)
	}
	if err := repoRepo.AddRepoMember(ctx, member); err != nil {
		t.Fatalf("AddRepoMember: %v", err)
	}

	moved, err := repoRepo.ReassignProject(ctx, added.ID, src.ID, dst.ID)
	if err != nil {
		t.Fatalf("ReassignProject: %v", err)
	}
	if moved.ProjectID != dst.ID {
		t.Errorf("expected repo moved to dst project, got %q", moved.ProjectID)
	}

	if _, err := repoRepo.GetRepoMembership(ctx, added.ID, member.UserID); err != domain.ErrRepoMembershipNotFound {
		t.Errorf("expected repo_members cleared on reassign, got %v", err)
	}

	// Stale fromProjectID (src) — repo already moved to dst.
	if _, err := repoRepo.ReassignProject(ctx, added.ID, src.ID, dst.ID); err != domain.ErrRepoProjectChanged {
		t.Errorf("expected ErrRepoProjectChanged for a stale fromProjectID, got %v", err)
	}
}
