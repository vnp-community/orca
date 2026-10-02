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

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestSessionOrigin_RoundTripsTerminalRows(t *testing.T) {
	db := setupDB(t)
	store := NewTerminalSessionStore(db)
	ctx := context.Background()
	now := time.Now().UTC()
	origin := &domain.SessionOrigin{Type: "mcp", ClientName: "Claude Code", MCPSessionID: "sess-row-1", UserID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"}

	if _, err := store.Create(ctx, domain.TerminalSession{PtyID: "pty-mcp", TenantID: testTenant1, Cwd: "/w", CreatedAt: now, LastActiveAt: now, Origin: origin}); err != nil {
		t.Fatalf("create mcp terminal: %v", err)
	}
	if _, err := store.Create(ctx, domain.TerminalSession{PtyID: "pty-ui", TenantID: testTenant1, Cwd: "/w", CreatedAt: now, LastActiveAt: now}); err != nil {
		t.Fatalf("create ui terminal: %v", err)
	}
	_, got, err := store.Get(ctx, testTenant1, "pty-mcp")
	if err != nil || got.Origin == nil || *got.Origin != *origin {
		t.Fatalf("mcp terminal origin = %+v err=%v, want %+v", got.Origin, err, origin)
	}
	_, ui, _ := store.Get(ctx, testTenant1, "pty-ui")
	if ui.Origin != nil {
		t.Fatalf("UI-created terminal must have a nil origin, got %+v", ui.Origin)
	}
	list, err := store.List(ctx, testTenant1, "")
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %d err=%v", len(list), err)
	}
}

func TestMigration0038_DownUpRoundTrip(t *testing.T) {
	rawDSN := testutil.StartMySQL(t, "infra")
	migrationsPath, err := filepath.Abs("../../../migrations/mysql")
	if err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("migrate", append([]string{"-path", migrationsPath, "-database", rawDSN}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("migrate %v: %v\n%s", args, err, out)
		}
	}
	run("up")
	db, err := sql.Open("mysql", strings.TrimPrefix(rawDSN, "mysql://")+"?parseTime=true")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	hasColumn := func() bool {
		_, err := db.Exec(`SELECT origin_mcp_session_id FROM terminal_sessions LIMIT 1`)
		return err == nil
	}
	if !hasColumn() {
		t.Fatal("origin columns missing after up")
	}
	run("down", "1")
	if hasColumn() {
		t.Fatal("origin columns still present after down 1")
	}
	run("up", "1")
	if !hasColumn() {
		t.Fatal("origin columns missing after re-up")
	}
}
