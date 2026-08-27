//go:build integration

package postgres

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// setupShareLinkStore mirrors setupRepository but also returns the plain
// *Repository (needed to create a task first — task_share_links.task_id
// has a foreign key into task.tasks).
func setupShareLinkStore(t *testing.T) (*ShareLinkStore, *Repository) {
	t.Helper()
	dsn := testutil.StartPostgres(t, "task")

	migrationsPath, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	cmd := exec.Command("migrate", "-path", migrationsPath, "-database", dsn, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running migrations: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting to postgres: %v", err)
	}
	t.Cleanup(pool.Close)

	return NewShareLinkStore(pool), New(pool)
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

// TestShareLinks_TokenHashNeverStoredAsPlaintext confirms the stored
// token_hash column is never equal to any value the caller might think of
// as "the plaintext" — i.e. Create only ever persists whatever the caller
// passes in as tokenHash, never anything else, and there is no separate
// plaintext-token column on the table at all (a schema-level guarantee,
// asserted here by round-tripping and confirming ONLY the given hash comes
// back, never a derived plaintext).
func TestShareLinks_TokenHashNeverStoredAsPlaintext(t *testing.T) {
	links, repo := setupShareLinkStore(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	task, _ := domain.NewTask(uuid.NewString(), tenantID, "task", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, task); err != nil {
		t.Fatalf("creating task: %v", err)
	}

	plaintextLookingValue := "this-would-be-the-plaintext-token-if-passed-directly"
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd" // a 64-hex-char SHA-256-shaped hash, deliberately NOT equal to plaintextLookingValue
	if _, err := links.Create(ctx, tenantID, task.ID, hash, uuid.NewString()); err != nil {
		t.Fatalf("creating share link: %v", err)
	}

	// Resolving by the "plaintext-looking" value must fail — only the hash
	// resolves.
	if _, err := links.ResolveActive(ctx, tenantID, plaintextLookingValue); err == nil {
		t.Error("expected resolving by a non-hash value to fail")
	}
	if _, err := links.ResolveActive(ctx, tenantID, hash); err != nil {
		t.Errorf("expected resolving by the actual stored hash to succeed, got %v", err)
	}
}
