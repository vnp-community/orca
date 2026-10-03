//go:build integration

package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func seedMcpTerminal(t *testing.T, store *TerminalSessionStore, ptyID string) domain.TerminalSession {
	t.Helper()
	now := time.Now().UTC()
	s := domain.TerminalSession{PtyID: ptyID, TenantID: testTenant1, Cwd: "/w", CreatedAt: now, LastActiveAt: now,
		Origin: &domain.SessionOrigin{Type: "mcp", ClientName: "Cursor", MCPSessionID: "s1", UserID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"}}
	if _, err := store.Create(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	return s
}

func closedEventRows(t *testing.T, repo *Repository) int {
	t.Helper()
	var n int
	if err := repo.db.QueryRow(`SELECT count(*) FROM outbox_events WHERE subject = ?`, domain.SubjectTerminalClosed).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCloseWithEvent_TransitionEnqueuesOnceAndRepeatDoesNot(t *testing.T) {
	repo := setupRepository(t)
	store := NewTerminalSessionStore(repo.db)
	ctx := context.Background()
	s := seedMcpTerminal(t, store, "pty-1")
	ev, _ := domain.NewTerminalClosedEvent(s, "idle", "", time.Now())

	if tr, err := store.CloseWithEvent(ctx, testTenant1, "pty-1", time.Now().UTC(), ev); err != nil || !tr {
		t.Fatalf("first close: transitioned=%v err=%v", tr, err)
	}
	if tr, err := store.CloseWithEvent(ctx, testTenant1, "pty-1", time.Now().UTC(), ev); err != nil || tr {
		t.Fatalf("repeat close: transitioned=%v err=%v", tr, err)
	}
	if n := closedEventRows(t, repo); n != 1 {
		t.Fatalf("outbox rows = %d, want 1", n)
	}
	if _, err := store.CloseWithEvent(ctx, testTenant1, "missing", time.Now().UTC(), ev); err == nil {
		t.Fatal("closing an unknown session must fail")
	}
	recs, err := repo.FetchUnpublished(ctx, 10)
	if err != nil || len(recs) != 1 || recs[0].Subject != domain.SubjectTerminalClosed || recs[0].ID != ev.ID {
		t.Fatalf("relay store contract: recs=%+v err=%v", recs, err)
	}
	if err := repo.MarkPublished(ctx, []string{recs[0].ID}); err != nil {
		t.Fatal(err)
	}
	if recs, _ := repo.FetchUnpublished(ctx, 10); len(recs) != 0 {
		t.Fatalf("published row still fetched: %+v", recs)
	}
}

func TestCloseWithEvent_OutboxFailureRollsBackStateChange(t *testing.T) {
	repo := setupRepository(t)
	store := NewTerminalSessionStore(repo.db)
	ctx := context.Background()
	s := seedMcpTerminal(t, store, "pty-1")
	ev, _ := domain.NewTerminalClosedEvent(s, "idle", "", time.Now())
	ev.PayloadJSON = []byte("{not json")

	if _, err := store.CloseWithEvent(ctx, testTenant1, "pty-1", time.Now().UTC(), ev); err == nil {
		t.Fatal("want error from invalid outbox payload")
	}
	_, got, _ := store.Get(ctx, testTenant1, "pty-1")
	if got.ClosedAt != nil {
		t.Fatal("closed_at persisted although the outbox insert failed")
	}
	if n := closedEventRows(t, repo); n != 0 {
		t.Fatalf("outbox rows = %d, want 0", n)
	}
}
