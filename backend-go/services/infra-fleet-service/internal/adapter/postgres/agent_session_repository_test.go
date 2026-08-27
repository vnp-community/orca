//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

const (
	testWorktree1 = "88888888-8888-8888-8888-888888888888"
	testUser1     = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
)

// setupAgentSessionStore starts a fresh Postgres container migrated through
// every migration (including 0007/0008/0009's agent_sessions additions),
// and returns AgentSessionStore alongside enough scaffolding (a dev server +
// terminal session row) for agent_sessions' foreign keys to be satisfiable.
func setupAgentSessionStore(t *testing.T) (*AgentSessionStore, *Repository, *TerminalSessionStore) {
	t.Helper()
	repo := setupRepository(t)
	pool := repo.pool
	terminalStore := NewTerminalSessionStore(pool)
	return NewAgentSessionStore(pool), repo, terminalStore
}

// seedAgentSessionRow creates the dev_servers/terminal_sessions rows
// agent_sessions.pty_id/dev_server_id foreign keys require, then returns a
// ready-to-Create domain.AgentSession referencing them. id is a fresh UUID
// (agent_sessions.id is typed UUID, unlike pty_id which is agent-assigned
// TEXT — see migrations/0007_agent_sessions.up.sql's doc comment).
func seedAgentSessionRow(t *testing.T, repo *Repository, terminalStore *TerminalSessionStore, tenantID, worktreeID, userID, ptyID string) domain.AgentSession {
	t.Helper()
	ctx := context.Background()

	ds, err := domain.NewDevServer(testDevServer1, tenantID, "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	if _, err := repo.Register(ctx, ds); err != nil {
		t.Fatalf("registering dev server: %v", err)
	}

	now := time.Now().UTC()
	if _, err := terminalStore.Create(ctx, domain.TerminalSession{
		PtyID: ptyID, TenantID: tenantID, Cwd: "/repo", CreatedAt: now, LastActiveAt: now,
	}); err != nil {
		t.Fatalf("seeding terminal session: %v", err)
	}

	return domain.AgentSession{
		ID: uuid.NewString(), TenantID: tenantID, PtyID: ptyID, WorktreeID: worktreeID, DevServerID: testDevServer1,
		UserID: userID, ModelID: "claude", Status: domain.AgentStatusSpawning, StartedAt: now, LastActiveAt: now,
	}
}

func TestAgentSessionStore_CreateThenGet_RoundTripsFields(t *testing.T) {
	store, repo, terminalStore := setupAgentSessionStore(t)
	ctx := context.Background()

	session := seedAgentSessionRow(t, repo, terminalStore, testTenant1, testWorktree1, testUser1, "agent-pty-1")
	session.AccountID = ""
	session.AgentVersion = "5.0.0"

	created, err := store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID != session.ID {
		t.Fatalf("expected the created session's ID to round-trip")
	}

	found, got, err := store.Get(ctx, testTenant1, session.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if got.PtyID != session.PtyID || got.WorktreeID != session.WorktreeID || got.UserID != session.UserID ||
		got.ModelID != session.ModelID || got.Status != session.Status || got.AgentVersion != session.AgentVersion {
		t.Fatalf("round-tripped session mismatch: got %+v, want fields from %+v", got, session)
	}
}

func TestAgentSessionStore_Create_ConcurrentActiveSession_ReturnsErrAgentAlreadyRunning(t *testing.T) {
	store, repo, terminalStore := setupAgentSessionStore(t)
	ctx := context.Background()

	first := seedAgentSessionRow(t, repo, terminalStore, testTenant1, testWorktree1, testUser1, "agent-pty-a")
	if _, err := store.Create(ctx, first); err != nil {
		t.Fatalf("Create (first): %v", err)
	}

	// A second terminal_sessions row is required for the second agent
	// session's own pty_id FK, but it targets the SAME (tenant, worktree,
	// user) tuple BR-AG-01's partial unique index guards.
	now := time.Now().UTC()
	if _, err := terminalStore.Create(ctx, domain.TerminalSession{
		PtyID: "agent-pty-b", TenantID: testTenant1, Cwd: "/repo", CreatedAt: now, LastActiveAt: now,
	}); err != nil {
		t.Fatalf("seeding second terminal session: %v", err)
	}
	second := domain.AgentSession{
		ID: uuid.NewString(), TenantID: testTenant1, PtyID: "agent-pty-b", WorktreeID: testWorktree1, DevServerID: testDevServer1,
		UserID: testUser1, ModelID: "claude", Status: domain.AgentStatusSpawning, StartedAt: now, LastActiveAt: now,
	}

	_, err := store.Create(ctx, second)
	if !errors.Is(err, domain.ErrAgentAlreadyRunning) {
		t.Fatalf("expected domain.ErrAgentAlreadyRunning, got %v", err)
	}

	// Once the first session is stopped, a fresh Create for the same tuple succeeds.
	if err := store.MarkStopped(ctx, testTenant1, first.ID, time.Now().UTC()); err != nil {
		t.Fatalf("MarkStopped: %v", err)
	}
	if _, err := store.Create(ctx, second); err != nil {
		t.Fatalf("expected Create to succeed once the prior session is stopped, got: %v", err)
	}
}

func TestAgentSessionStore_UpdateStatusAndMarkStopped(t *testing.T) {
	store, repo, terminalStore := setupAgentSessionStore(t)
	ctx := context.Background()

	session := seedAgentSessionRow(t, repo, terminalStore, testTenant1, testWorktree1, testUser1, "agent-pty-status")
	if _, err := store.Create(ctx, session); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.UpdateStatus(ctx, testTenant1, session.ID, domain.AgentStatusRunning, time.Now().UTC()); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	_, got, err := store.Get(ctx, testTenant1, session.ID)
	if err != nil || got.Status != domain.AgentStatusRunning {
		t.Fatalf("expected status=running after UpdateStatus, got %+v (err=%v)", got, err)
	}

	if err := store.MarkStopped(ctx, testTenant1, session.ID, time.Now().UTC()); err != nil {
		t.Fatalf("MarkStopped: %v", err)
	}
	_, got, err = store.Get(ctx, testTenant1, session.ID)
	if err != nil || got.Status != domain.AgentStatusStopped || got.StoppedAt == nil {
		t.Fatalf("expected status=stopped with StoppedAt set, got %+v (err=%v)", got, err)
	}
}

func TestAgentSessionStore_MarkStoppedWithStatus(t *testing.T) {
	store, repo, terminalStore := setupAgentSessionStore(t)
	ctx := context.Background()

	session := seedAgentSessionRow(t, repo, terminalStore, testTenant1, testWorktree1, testUser1, "agent-pty-mswerror")
	if _, err := store.Create(ctx, session); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.MarkStoppedWithStatus(ctx, testTenant1, session.ID, domain.AgentStatusError, time.Now().UTC()); err != nil {
		t.Fatalf("MarkStoppedWithStatus: %v", err)
	}
	_, got, err := store.Get(ctx, testTenant1, session.ID)
	if err != nil || got.Status != domain.AgentStatusError || got.StoppedAt == nil {
		t.Fatalf("expected status=error with StoppedAt set, got %+v (err=%v)", got, err)
	}
}

func TestAgentSessionStore_LatestAndMostRecentActiveForWorktree(t *testing.T) {
	store, repo, terminalStore := setupAgentSessionStore(t)
	ctx := context.Background()

	older := seedAgentSessionRow(t, repo, terminalStore, testTenant1, testWorktree1, testUser1, "agent-pty-older")
	older.StartedAt = time.Now().Add(-time.Hour).UTC()
	older.LastActiveAt = older.StartedAt
	if _, err := store.Create(ctx, older); err != nil {
		t.Fatalf("Create (older): %v", err)
	}
	if err := store.MarkStopped(ctx, testTenant1, older.ID, older.StartedAt); err != nil {
		t.Fatalf("MarkStopped (older): %v", err)
	}

	now := time.Now().UTC()
	if _, err := terminalStore.Create(ctx, domain.TerminalSession{
		PtyID: "agent-pty-newer", TenantID: testTenant1, Cwd: "/repo", CreatedAt: now, LastActiveAt: now,
	}); err != nil {
		t.Fatalf("seeding newer terminal session: %v", err)
	}
	newer := domain.AgentSession{
		ID: uuid.NewString(), TenantID: testTenant1, PtyID: "agent-pty-newer", WorktreeID: testWorktree1, DevServerID: testDevServer1,
		UserID: testUser1, ModelID: "claude", Status: domain.AgentStatusRunning, StartedAt: now, LastActiveAt: now,
	}
	if _, err := store.Create(ctx, newer); err != nil {
		t.Fatalf("Create (newer): %v", err)
	}

	found, latest, err := store.LatestForWorktree(ctx, testTenant1, testWorktree1)
	if err != nil || !found || latest.ID != newer.ID {
		t.Fatalf("expected LatestForWorktree to return the newer session, got %+v (found=%v err=%v)", latest, found, err)
	}

	found, active, err := store.MostRecentActiveForWorktree(ctx, testTenant1, testWorktree1)
	if err != nil || !found || active.ID != newer.ID {
		t.Fatalf("expected MostRecentActiveForWorktree to return the newer (non-terminal) session, got %+v (found=%v err=%v)", active, found, err)
	}
}

func TestAgentSessionStore_UpdateProviderSession(t *testing.T) {
	store, repo, terminalStore := setupAgentSessionStore(t)
	ctx := context.Background()

	session := seedAgentSessionRow(t, repo, terminalStore, testTenant1, testWorktree1, testUser1, "agent-pty-provider")
	if _, err := store.Create(ctx, session); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.UpdateProviderSession(ctx, testTenant1, session.ID, "session_id", "provider-sess-xyz"); err != nil {
		t.Fatalf("UpdateProviderSession: %v", err)
	}
	_, got, err := store.Get(ctx, testTenant1, session.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ResumeProviderSessionKey != "session_id" || got.ResumeProviderSessionID != "provider-sess-xyz" {
		t.Fatalf("expected provider session fields to round-trip, got %+v", got)
	}
}
