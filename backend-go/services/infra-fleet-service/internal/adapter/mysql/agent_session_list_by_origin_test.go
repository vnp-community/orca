//go:build integration

package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

func TestAgentSessionStore_ListByOrigin(t *testing.T) {
	db := setupDB(t)
	repo := New(db)
	terminalStore := NewTerminalSessionStore(db)
	store := NewAgentSessionStore(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	ds1, _ := domain.NewDevServer(uuid.NewString(), testTenant1, "10.0.0.41", domain.ConnectionModeRelayWebSocket, "", nil)
	ds2, _ := domain.NewDevServer(uuid.NewString(), testTenant2, "10.0.0.42", domain.ConnectionModeRelayWebSocket, "", nil)
	for _, ds := range []domain.DevServer{ds1, ds2} {
		if _, err := repo.Register(ctx, ds); err != nil {
			t.Fatal(err)
		}
	}
	mk := func(tenant, devServer, pty string, origin *domain.SessionOrigin, status domain.AgentStatus) domain.AgentSession {
		t.Helper()
		if _, err := terminalStore.Create(ctx, domain.TerminalSession{PtyID: pty, TenantID: tenant, CreatedAt: now, LastActiveAt: now}); err != nil {
			t.Fatal(err)
		}
		s := domain.AgentSession{ID: uuid.NewString(), TenantID: tenant, PtyID: pty, WorktreeID: uuid.NewString(), DevServerID: devServer,
			UserID: uuid.NewString(), ModelID: "claude", Status: status, StartedAt: now, LastActiveAt: now, Origin: origin}
		if _, err := store.Create(ctx, s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	userID := uuid.NewString()
	s1 := &domain.SessionOrigin{Type: "mcp", ClientName: "c", MCPSessionID: "sess-1", UserID: userID}
	s2 := &domain.SessionOrigin{Type: "mcp", ClientName: "c", MCPSessionID: "sess-2", UserID: userID}
	a1 := mk(testTenant1, ds1.ID, "p1", s1, domain.AgentStatusRunning)
	a2 := mk(testTenant1, ds1.ID, "p2", s1, domain.AgentStatusStopped)
	mk(testTenant1, ds1.ID, "p3", s2, domain.AgentStatusRunning)
	mk(testTenant1, ds1.ID, "p4", nil, domain.AgentStatusRunning)
	mk(testTenant2, ds2.ID, "p5", s1, domain.AgentStatusRunning)

	ids := func(f usecase.AgentSessionOriginFilter) map[string]bool {
		t.Helper()
		if f.Limit == 0 {
			f.Limit = 100
		}
		got, err := store.ListByOrigin(ctx, testTenant1, f)
		if err != nil {
			t.Fatal(err)
		}
		m := map[string]bool{}
		for _, s := range got {
			m[s.ID] = true
			if s.TenantID != testTenant1 {
				t.Fatalf("leaked tenant %s", s.TenantID)
			}
		}
		return m
	}
	if got := ids(usecase.AgentSessionOriginFilter{OriginSessionID: "sess-1"}); len(got) != 2 || !got[a1.ID] || !got[a2.ID] {
		t.Fatalf("by mcp session: %v", got)
	}
	if got := ids(usecase.AgentSessionOriginFilter{OriginSessionID: "sess-1", ActiveOnly: true}); len(got) != 1 || !got[a1.ID] {
		t.Fatalf("active only: %v", got)
	}
	if got := ids(usecase.AgentSessionOriginFilter{OriginType: "mcp"}); len(got) != 3 {
		t.Fatalf("by type must exclude UI-created and other tenants: %v", got)
	}
	if got := ids(usecase.AgentSessionOriginFilter{OriginType: "mcp", OriginSessionID: "nope"}); len(got) != 0 {
		t.Fatalf("unknown session: %v", got)
	}
	if got := ids(usecase.AgentSessionOriginFilter{OriginType: "mcp", Limit: 1}); len(got) != 1 {
		t.Fatalf("limit: %v", got)
	}
}
