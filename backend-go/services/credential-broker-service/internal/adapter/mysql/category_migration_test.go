//go:build integration

package mysql

import (
	"context"
	"database/sql"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/testutil"
)

func runMigrate(t *testing.T, dsn string, args ...string) {
	t.Helper()
	path, err := filepath.Abs("../../../migrations/mysql")
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("migrate", append([]string{"-path", path, "-database", dsn}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("migrate %v: %v\n%s", args, err, out)
	}
}

func insertCategory(ctx context.Context, db *sql.DB, category string) error {
	id := uuid.NewString()
	_, err := db.ExecContext(ctx, `INSERT INTO credential_metadata (id, tenant_id, owner_id, category, vault_path)
		VALUES (?, ?, 'mcp:s:env:T', ?, ?)`, id, uuid.NewString(), category, "credential/t/"+id)
	return err
}

func TestMigration0004_CategoryCheck_UpDownUp(t *testing.T) {
	raw := testutil.StartMySQL(t, "credential")
	ctx := context.Background()
	runMigrate(t, raw, "up")
	db, err := sql.Open("mysql", strings.TrimPrefix(raw, "mysql://")+"?parseTime=true")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, c := range []string{"mcp_external_secret", "dev_server_agent_token"} {
		if err := insertCategory(ctx, db, c); err != nil {
			t.Fatalf("%s should be accepted after up: %v", c, err)
		}
	}
	if err := insertCategory(ctx, db, "bogus"); err == nil {
		t.Fatal("unknown category must be rejected")
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM credential_metadata`); err != nil {
		t.Fatal(err)
	}
	runMigrate(t, raw, "down", "1")
	if err := insertCategory(ctx, db, "mcp_external_secret"); err == nil {
		t.Fatal("mcp_external_secret must be rejected after down")
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM credential_metadata`); err != nil {
		t.Fatal(err)
	}
	runMigrate(t, raw, "up")
	if err := insertCategory(ctx, db, "mcp_external_secret"); err != nil {
		t.Fatalf("accepted after second up: %v", err)
	}
}
