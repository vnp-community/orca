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

	// Need a dev_server first due to foreign key
	devServerID := uuid.NewString()
	_, err := repo.pool.Exec(ctx, `
		INSERT INTO infra.dev_servers (id, tenant_id, host, connection_mode)
		VALUES ($1, $2, 'localhost', 'direct-websocket')
	`, devServerID, tenantID)
	if err != nil {
		t.Fatalf("insert dev_server: %v", err)
	}

	insertProfile := func() error {
		_, err := repo.pool.Exec(ctx, `
			INSERT INTO infra.dev_server_capability_profiles (dev_server_id, tenant_id, source, fingerprint, probed_at)
			VALUES ($1, $2, 'probe', 'fake-fingerprint', $3)
		`, devServerID, tenantID, time.Now())
		return err
	}

	if err := insertProfile(); err != nil {
		t.Fatalf("first insert into capability_profiles: %v", err)
	}

	migrationsPath, err := filepath.Abs("../../../migrations/postgres")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	dsn := repo.pool.Config().ConnString()

	// Down 1
	cmdDown := exec.Command("migrate", "-path", migrationsPath, "-database", dsn, "down", "1")
	if out, err := cmdDown.CombinedOutput(); err != nil {
		t.Fatalf("running migration down: %v\n%s", err, out)
	}

	// Verify table dropped
	_, err = repo.pool.Exec(ctx, `SELECT 1 FROM infra.dev_server_capability_profiles LIMIT 1`)
	if err == nil {
		t.Fatal("expected table to be dropped")
	}

	// Up 1
	cmdUp := exec.Command("migrate", "-path", migrationsPath, "-database", dsn, "up", "1")
	if out, err := cmdUp.CombinedOutput(); err != nil {
		t.Fatalf("running migration up: %v\n%s", err, out)
	}

	// Verify table recreated and can insert
	if err := insertProfile(); err != nil {
		t.Fatalf("insert after up: %v", err)
	}
}
