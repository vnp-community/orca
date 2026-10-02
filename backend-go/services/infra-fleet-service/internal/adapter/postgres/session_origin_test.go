//go:build integration

package postgres

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

const testMcpUser = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

// TestSessionOrigin_RoundTripsTerminalAndAgentRows proves BE-MCP-SOL-009's
// origin_* columns survive Create/Get/List, and that UI-created rows keep a
// nil Origin.
func TestSessionOrigin_RoundTripsTerminalAndAgentRows(t *testing.T) {
	store, repo, terminalStore := setupAgentSessionStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	origin := &domain.SessionOrigin{Type: "mcp", ClientName: "Claude Code", MCPSessionID: "sess-row-1", UserID: testMcpUser}

	if _, err := terminalStore.Create(ctx, domain.TerminalSession{PtyID: "pty-mcp", TenantID: testTenant1, Cwd: "/w", CreatedAt: now, LastActiveAt: now, Origin: origin}); err != nil {
		t.Fatalf("create mcp terminal: %v", err)
	}
	if _, err := terminalStore.Create(ctx, domain.TerminalSession{PtyID: "pty-ui", TenantID: testTenant1, Cwd: "/w", CreatedAt: now, LastActiveAt: now}); err != nil {
		t.Fatalf("create ui terminal: %v", err)
	}
	_, got, err := terminalStore.Get(ctx, testTenant1, "pty-mcp")
	if err != nil || got.Origin == nil || *got.Origin != *origin {
		t.Fatalf("mcp terminal origin = %+v err=%v, want %+v", got.Origin, err, origin)
	}
	_, ui, _ := terminalStore.Get(ctx, testTenant1, "pty-ui")
	if ui.Origin != nil {
		t.Fatalf("UI-created terminal must have a nil origin, got %+v", ui.Origin)
	}
	list, err := terminalStore.List(ctx, testTenant1, "")
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %d err=%v", len(list), err)
	}
	withOrigin := 0
	for _, s := range list {
		if s.Origin != nil {
			withOrigin++
		}
	}
	if withOrigin != 1 {
		t.Fatalf("exactly one listed terminal carries an origin, got %d", withOrigin)
	}

	agent := seedAgentSessionRow(t, repo, terminalStore, testTenant1, testWorktree1, testUser1, "agent-pty-origin")
	agent.Origin = origin
	if _, err := store.Create(ctx, agent); err != nil {
		t.Fatalf("create agent session: %v", err)
	}
	_, ag, err := store.Get(ctx, testTenant1, agent.ID)
	if err != nil || ag.Origin == nil || *ag.Origin != *origin {
		t.Fatalf("agent origin = %+v err=%v, want %+v", ag.Origin, err, origin)
	}
}

// TestMigration0038_DownUpRoundTrip rolls the origin migration back and forward.
func TestMigration0038_DownUpRoundTrip(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	migrationsPath, err := filepath.Abs("../../../migrations/postgres")
	if err != nil {
		t.Fatal(err)
	}
	dsn := repo.pool.Config().ConnString()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("migrate", append([]string{"-path", migrationsPath, "-database", dsn}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("migrate %v: %v\n%s", args, err, out)
		}
	}
	hasColumn := func() bool {
		_, err := repo.pool.Exec(ctx, `SELECT origin_mcp_session_id FROM infra.terminal_sessions LIMIT 1`)
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
