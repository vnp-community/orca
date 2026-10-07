//go:build integration

package mysql

import (
	"context"
	"database/sql"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/testutil"
)

func TestMigration0039_UpDownUp(t *testing.T) {
	rawDSN := testutil.StartMySQL(t, "infra")
	driverDSN := strings.TrimPrefix(rawDSN, "mysql://") + "?parseTime=true"

	migrationsPath, err := filepath.Abs("../../../migrations/mysql")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}

	// Up to latest
	cmdUpAll := exec.Command("migrate", "-path", migrationsPath, "-database", rawDSN, "up")
	if out, err := cmdUpAll.CombinedOutput(); err != nil {
		t.Fatalf("running migrations up: %v\n%s", err, out)
	}

	db, err := sql.Open("mysql", driverDSN)
	if err != nil {
		t.Fatalf("connecting to mysql: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	tenantID := uuid.NewString()
	devServerID := uuid.NewString()

	_, err = db.ExecContext(ctx, `
		INSERT INTO dev_servers (id, tenant_id, host, connection_mode)
		VALUES (?, ?, 'localhost', 'direct-websocket')
	`, devServerID, tenantID)
	if err != nil {
		t.Fatalf("insert dev_server: %v", err)
	}

	insertProfile := func() error {
		_, err := db.ExecContext(ctx, `
			INSERT INTO dev_server_capability_profiles (dev_server_id, tenant_id, source, agent_build_version, features, profile, fingerprint, probed_at)
			VALUES (?, ?, 'probe', '', '[]', '{}', 'fake-fingerprint', ?)
		`, devServerID, tenantID, time.Now())
		return err
	}

	if err := insertProfile(); err != nil {
		t.Fatalf("first insert into capability_profiles: %v", err)
	}

	// Down 1
	cmdDown := exec.Command("migrate", "-path", migrationsPath, "-database", rawDSN, "down", "1")
	if out, err := cmdDown.CombinedOutput(); err != nil {
		t.Fatalf("running migration down: %v\n%s", err, out)
	}

	_, err = db.ExecContext(ctx, `SELECT 1 FROM dev_server_capability_profiles LIMIT 1`)
	if err == nil {
		t.Fatal("expected table to be dropped")
	}

	// Up 1
	cmdUp := exec.Command("migrate", "-path", migrationsPath, "-database", rawDSN, "up", "1")
	if out, err := cmdUp.CombinedOutput(); err != nil {
		t.Fatalf("running migration up 1: %v\n%s", err, out)
	}

	if err := insertProfile(); err != nil {
		t.Fatalf("insert after up: %v", err)
	}
}
