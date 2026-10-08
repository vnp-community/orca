//go:build integration

package postgres

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMigration0039_UpDownUp(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	devServerID := seedDevServer(t, repo, tenantID)

	insertProfile := func() error {
		tx, err := repo.pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO infra.dev_server_capability_profiles (dev_server_id, tenant_id, source, fingerprint, probed_at)
			VALUES ($1, $2, 'probe', repeat('f', 64), $3)
		`, devServerID, tenantID, time.Now()); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if err := insertProfile(); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	migrationsPath, err := filepath.Abs("../../../migrations/postgres")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	dsn := repo.pool.Config().ConnString()

	if out, err := exec.Command("migrate", "-path", migrationsPath, "-database", dsn, "down", "1").CombinedOutput(); err != nil {
		t.Fatalf("migration down: %v\n%s", err, out)
	}
	if _, err := repo.pool.Exec(ctx, `SELECT 1 FROM infra.dev_server_capability_profiles LIMIT 1`); err == nil {
		t.Fatal("expected table to be dropped by down")
	}
	if out, err := exec.Command("migrate", "-path", migrationsPath, "-database", dsn, "up", "1").CombinedOutput(); err != nil {
		t.Fatalf("migration up: %v\n%s", err, out)
	}
	if err := insertProfile(); err != nil {
		t.Fatalf("insert after re-up: %v", err)
	}
}
