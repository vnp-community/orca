//go:build integration

package postgres

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
)

func runMigrate(t *testing.T, dsn string, args ...string) {
	t.Helper()
	path, err := filepath.Abs("../../../migrations/postgres")
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("migrate", append([]string{"-path", path, "-database", dsn}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("migrate %v: %v\n%s", args, err, out)
	}
}

func insertCategory(ctx context.Context, pool *pgxpool.Pool, category string) error {
	id := uuid.NewString()
	_, err := pool.Exec(ctx, `INSERT INTO credential.credential_metadata (id, tenant_id, owner_id, category, vault_path)
		VALUES ($1, $2, 'mcp:s:env:T', $3, $4)`, id, uuid.NewString(), category, "credential/t/"+id)
	return err
}

// 0004 must be reversible and must admit exactly the two new categories.
func TestMigration0004_CategoryCheck_UpDownUp(t *testing.T) {
	dsn := testutil.StartPostgres(t, "credential_broker")
	ctx := context.Background()
	runMigrate(t, dsn, "up")
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	for _, c := range []string{"mcp_external_secret", "dev_server_agent_token"} {
		if err := insertCategory(ctx, pool, c); err != nil {
			t.Fatalf("%s should be accepted after up: %v", c, err)
		}
	}
	if err := insertCategory(ctx, pool, "bogus"); err == nil {
		t.Fatal("unknown category must be rejected")
	}

	if _, err := pool.Exec(ctx, `DELETE FROM credential.credential_metadata`); err != nil {
		t.Fatal(err)
	}
	runMigrate(t, dsn, "down", "1")
	if err := insertCategory(ctx, pool, "mcp_external_secret"); err == nil {
		t.Fatal("mcp_external_secret must be rejected after down")
	}
	if err := insertCategory(ctx, pool, "scm_oauth"); err != nil {
		t.Fatalf("original categories stay valid after down: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM credential.credential_metadata`); err != nil {
		t.Fatal(err)
	}
	runMigrate(t, dsn, "up")
	if err := insertCategory(ctx, pool, "mcp_external_secret"); err != nil {
		t.Fatalf("mcp_external_secret accepted after second up: %v", err)
	}
}
