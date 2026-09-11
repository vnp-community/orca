//go:build integration

package mysql

import (
	"context"
	"database/sql"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// setupShareLinkStore mirrors setupRepository but also returns the plain
// *Repository (needed to create a task first — task_share_links.task_id has
// a foreign key into tasks).
func setupShareLinkStore(t *testing.T) (*ShareLinkStore, *Repository) {
	t.Helper()
	rawDSN := testutil.StartMySQL(t, "task")
	driverDSN := strings.TrimPrefix(rawDSN, "mysql://") + "?parseTime=true"

	migrationsPath, err := filepath.Abs("../../../migrations/mysql")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	cmd := exec.Command("migrate", "-path", migrationsPath, "-database", rawDSN, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running migrations: %v\n%s", err, out)
	}

	db, err := sql.Open("mysql", driverDSN)
	if err != nil {
		t.Fatalf("connecting to mysql: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("pinging mysql: %v", err)
	}

	return NewShareLinkStore(db), New(db)
}

func TestShareLinks_Create_And_ResolveActive(t *testing.T) {
	links, repo := setupShareLinkStore(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	task, _ := domain.NewTask(uuid.NewString(), tenantID, "task", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, task); err != nil {
		t.Fatalf("creating task: %v", err)
	}

	id, err := links.Create(ctx, tenantID, task.ID, "hash-of-token", uuid.NewString())
	if err != nil {
		t.Fatalf("creating share link: %v", err)
	}
	if id == "" {
		t.Fatal("expected a non-empty share link id")
	}

	resolved, err := links.ResolveActive(ctx, tenantID, "hash-of-token")
	if err != nil {
		t.Fatalf("resolving active link: %v", err)
	}
	if resolved != task.ID {
		t.Errorf("expected resolved task_id=%s, got %s", task.ID, resolved)
	}
}

func TestShareLinks_ResolveActive_RevokedLink_NotFound(t *testing.T) {
	links, repo := setupShareLinkStore(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	task, _ := domain.NewTask(uuid.NewString(), tenantID, "task", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, task); err != nil {
		t.Fatalf("creating task: %v", err)
	}
	id, err := links.Create(ctx, tenantID, task.ID, "hash-of-token", uuid.NewString())
	if err != nil {
		t.Fatalf("creating share link: %v", err)
	}

	if err := links.Revoke(ctx, tenantID, id); err != nil {
		t.Fatalf("revoking: %v", err)
	}

	if _, err := links.ResolveActive(ctx, tenantID, "hash-of-token"); err == nil {
		t.Fatal("expected a not-found error resolving a revoked link")
	}
}

func TestShareLinks_TaskIDFor(t *testing.T) {
	links, repo := setupShareLinkStore(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	task, _ := domain.NewTask(uuid.NewString(), tenantID, "task", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, task); err != nil {
		t.Fatalf("creating task: %v", err)
	}
	id, err := links.Create(ctx, tenantID, task.ID, "hash-of-token", uuid.NewString())
	if err != nil {
		t.Fatalf("creating share link: %v", err)
	}

	got, err := links.TaskIDFor(ctx, tenantID, id)
	if err != nil {
		t.Fatalf("TaskIDFor: %v", err)
	}
	if got != task.ID {
		t.Errorf("expected task_id=%s, got %s", task.ID, got)
	}
}

func TestShareLinks_Revoke_NonexistentLink_Fails(t *testing.T) {
	links, _ := setupShareLinkStore(t)
	ctx := context.Background()

	if err := links.Revoke(ctx, uuid.NewString(), uuid.NewString()); err == nil {
		t.Fatal("expected an error revoking a nonexistent share link")
	}
}

// TestShareLinks_TwoTasksCannotShareATokenHash exercises
// migrations/mysql/0004's task_share_links_token_hash_unique constraint —
// the MySQL translation of Postgres's `token_hash TEXT NOT NULL UNIQUE`.
func TestShareLinks_TwoTasksCannotShareATokenHash(t *testing.T) {
	links, repo := setupShareLinkStore(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	taskA, _ := domain.NewTask(uuid.NewString(), tenantID, "a", domain.StatusOpen, "", "")
	taskB, _ := domain.NewTask(uuid.NewString(), tenantID, "b", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, taskA); err != nil {
		t.Fatalf("creating task a: %v", err)
	}
	if _, err := repo.Create(ctx, taskB); err != nil {
		t.Fatalf("creating task b: %v", err)
	}

	if _, err := links.Create(ctx, tenantID, taskA.ID, "shared-hash", uuid.NewString()); err != nil {
		t.Fatalf("creating first link: %v", err)
	}
	if _, err := links.Create(ctx, tenantID, taskB.ID, "shared-hash", uuid.NewString()); err == nil {
		t.Fatal("expected a UNIQUE violation creating a second link with the same token_hash")
	}
}

// TestShareLinks_GetByShareToken_ResolvesPublicly mirrors
// internal/adapter/postgres/share_link.go's GetByShareToken (a separate
// TaskRepository method, deliberately unauthenticated/no tenant filter).
func TestShareLinks_GetByShareToken_ResolvesPublicly(t *testing.T) {
	_, repo := setupShareLinkStore(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	task, _ := domain.NewTask(uuid.NewString(), tenantID, "task", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, task); err != nil {
		t.Fatalf("creating task: %v", err)
	}
	task.ShareToken = "a-public-share-token"
	if err := repo.Update(ctx, tenantID, task, nil); err != nil {
		t.Fatalf("setting share token: %v", err)
	}

	got, err := repo.GetByShareToken(ctx, "a-public-share-token")
	if err != nil {
		t.Fatalf("GetByShareToken: %v", err)
	}
	if got.ID != task.ID {
		t.Errorf("expected task %s, got %s", task.ID, got.ID)
	}

	if _, err := repo.GetByShareToken(ctx, "no-such-token"); err == nil {
		t.Fatal("expected an error resolving an unknown share token")
	}
}
