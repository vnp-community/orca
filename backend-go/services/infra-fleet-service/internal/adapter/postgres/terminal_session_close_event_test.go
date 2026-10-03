//go:build integration

package postgres

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func seedMcpTerminal(t *testing.T, store *TerminalSessionStore, ptyID string) domain.TerminalSession {
	t.Helper()
	now := time.Now().UTC()
	s := domain.TerminalSession{PtyID: ptyID, TenantID: testTenant1, Cwd: "/w", CreatedAt: now, LastActiveAt: now,
		Origin: &domain.SessionOrigin{Type: "mcp", ClientName: "Cursor", MCPSessionID: "s1", UserID: testMcpUser}}
	if _, err := store.Create(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	return s
}

func outboxCount(t *testing.T, repo *Repository) int {
	t.Helper()
	var n int
	if err := repo.pool.QueryRow(context.Background(), `SELECT count(*) FROM infra.outbox_events WHERE subject = $1`, domain.SubjectTerminalClosed).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCloseWithEvent_TransitionEnqueuesOnceAndRepeatDoesNot(t *testing.T) {
	repo, _ := setupSshTargetStore(t)
	store := NewTerminalSessionStore(repo.pool)
	ctx := context.Background()
	s := seedMcpTerminal(t, store, "pty-1")
	ev, _ := domain.NewTerminalClosedEvent(s, "idle", "", time.Now())

	if tr, err := store.CloseWithEvent(ctx, testTenant1, "pty-1", time.Now().UTC(), ev); err != nil || !tr {
		t.Fatalf("first close: transitioned=%v err=%v", tr, err)
	}
	if tr, err := store.CloseWithEvent(ctx, testTenant1, "pty-1", time.Now().UTC(), ev); err != nil || tr {
		t.Fatalf("repeat close: transitioned=%v err=%v", tr, err)
	}
	if n := outboxCount(t, repo); n != 1 {
		t.Fatalf("outbox rows = %d, want 1", n)
	}
	if _, err := store.CloseWithEvent(ctx, testTenant1, "missing", time.Now().UTC(), ev); err == nil {
		t.Fatal("closing an unknown session must fail")
	}
}

// A failure after the state change (bad outbox payload) must roll the close back.
func TestCloseWithEvent_OutboxFailureRollsBackStateChange(t *testing.T) {
	repo, _ := setupSshTargetStore(t)
	store := NewTerminalSessionStore(repo.pool)
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
	if n := outboxCount(t, repo); n != 0 {
		t.Fatalf("outbox rows = %d, want 0", n)
	}
}

func TestCloseWithEvent_RelayPublishesClosedEvent(t *testing.T) {
	repo, _ := setupSshTargetStore(t)
	store := NewTerminalSessionStore(repo.pool)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := seedMcpTerminal(t, store, "pty-1")
	ev, _ := domain.NewTerminalClosedEvent(s, "idle", "", time.Now())
	if _, err := store.CloseWithEvent(ctx, testTenant1, "pty-1", time.Now().UTC(), ev); err != nil {
		t.Fatal(err)
	}

	url := testutil.StartNATS(t)
	pub, _, closeBus, err := commoneventbus.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closeBus() }()
	if err := pub.EnsureStream(ctx, "INFRAFLEET", []string{"orca.infrafleet.>"}); err != nil {
		t.Fatal(err)
	}
	relayCtx, stop := context.WithCancel(ctx)
	defer stop()
	go outbox.NewRelay(repo, pub, outbox.Config{PollInterval: 50 * time.Millisecond, BatchSize: 10}, slog.Default()).Run(relayCtx)

	nc, _ := nats.Connect(url)
	defer nc.Close()
	js, _ := jetstream.New(nc)
	stream, _ := js.Stream(ctx, "INFRAFLEET")
	deadline := time.Now().Add(20 * time.Second)
	for {
		info, err := stream.Info(ctx)
		if err == nil && info.State.Msgs == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("relay did not publish the closed event (info=%v err=%v)", info, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	var unpublished int
	for i := 0; i < 50 && unpublished == 0; i++ {
		_ = repo.pool.QueryRow(ctx, `SELECT count(*) FROM infra.outbox_events WHERE published_at IS NOT NULL`).Scan(&unpublished)
		time.Sleep(50 * time.Millisecond)
	}
	if unpublished != 1 {
		t.Fatal("outbox row not marked published")
	}
}
