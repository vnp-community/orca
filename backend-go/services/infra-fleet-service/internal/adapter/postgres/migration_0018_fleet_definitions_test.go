//go:build integration

package postgres

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

// TestMigration0018_CreatesTable confirms infra.fleet_definitions exists
// with the expected columns by round-tripping a row through it.
func TestMigration0018_CreatesTable(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	id := uuid.NewString()
	tenantID := uuid.NewString()
	createdBy := uuid.NewString()
	_, err := repo.pool.Exec(ctx, `
		INSERT INTO infra.fleet_definitions (id, tenant_id, name, version, servers, provision, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, id, tenantID, "my-fleet", 1, `[{"host":"h1"}]`, `{"iac":"terraform"}`, createdBy)
	if err != nil {
		t.Fatalf("insert into infra.fleet_definitions: %v", err)
	}

	var name string
	var version int
	row := repo.pool.QueryRow(ctx, `SELECT name, version FROM infra.fleet_definitions WHERE id = $1`, id)
	if err := row.Scan(&name, &version); err != nil {
		t.Fatalf("select from infra.fleet_definitions: %v", err)
	}
	if name != "my-fleet" || version != 1 {
		t.Errorf("unexpected row: name=%q version=%d", name, version)
	}
}

// TestMigration0018_UniqueConstraintRejectsSameTenantNameDuplicate confirms
// the (tenant_id, name) unique constraint.
func TestMigration0018_UniqueConstraintRejectsSameTenantNameDuplicate(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	insert := func() error {
		_, err := repo.pool.Exec(ctx, `
			INSERT INTO infra.fleet_definitions (id, tenant_id, name, servers, created_by)
			VALUES ($1, $2, $3, $4, $5)
		`, uuid.NewString(), tenantID, "my-fleet", `[]`, uuid.NewString())
		return err
	}
	if err := insert(); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := insert(); err == nil {
		t.Fatal("expected a unique_violation on a duplicate (tenant_id, name)")
	}
}

// TestMigration0018_DownDropsTable runs `down 1` after `up` and confirms
// infra.fleet_definitions no longer exists — clean rollback.
func TestMigration0018_DownDropsTable(t *testing.T) {
	repo := setupRepository(t) // already ran `up` through the latest migration

	migrationsPath, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	dsn := repo.pool.Config().ConnString()
	cmd := exec.Command("migrate", "-path", migrationsPath, "-database", dsn, "down", "1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running migration down: %v\n%s", err, out)
	}

	_, err = repo.pool.Exec(context.Background(), `SELECT 1 FROM infra.fleet_definitions LIMIT 1`)
	if err == nil {
		t.Fatal("expected infra.fleet_definitions to no longer exist after `down 1`")
	}
}
