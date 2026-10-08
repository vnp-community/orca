//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func outboxEvent(tenantID, subject string) domain.OutboxEvent {
	return domain.OutboxEvent{ID: uuid.NewString(), TenantID: tenantID, Subject: subject, OccurredAt: time.Now().UTC(), Version: 1, Payload: []byte(`{"k":"v"}`)}
}

func outboxCount(t *testing.T, f *pgFixture) int {
	t.Helper()
	var n int
	if err := f.admin.QueryRow(context.Background(), `SELECT count(*) FROM request.outbox_events`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestInTx_RollbackLeavesNoOutboxRow(t *testing.T) {
	f := newMigratedPostgres(t)
	repo := New(f.app)
	tid := uuid.NewString()
	ctx := tenant.WithTenantID(context.Background(), tid)
	boom := errors.New("boom")

	err := repo.InTx(ctx, func(txCtx context.Context) error {
		if err := repo.InsertOutboxEvent(txCtx, outboxEvent(tid, "orca.request.request.created")); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) || outboxCount(t, f) != 0 {
		t.Fatalf("rollback must leave no outbox row: err=%v rows=%d", err, outboxCount(t, f))
	}
	err = repo.InTx(ctx, func(txCtx context.Context) error {
		return repo.InsertOutboxEvent(txCtx, outboxEvent(tid, "orca.request.request.created"))
	})
	if err != nil || outboxCount(t, f) != 1 {
		t.Fatalf("commit must keep the row: err=%v rows=%d", err, outboxCount(t, f))
	}
}

func TestInTx_NestedJoinsOuterTransaction(t *testing.T) {
	f := newMigratedPostgres(t)
	repo := New(f.app)
	tid := uuid.NewString()
	ctx := tenant.WithTenantID(context.Background(), tid)
	boom := errors.New("boom")

	err := repo.InTx(ctx, func(outer context.Context) error {
		if err := repo.InTx(outer, func(inner context.Context) error {
			return repo.InsertOutboxEvent(inner, outboxEvent(tid, "orca.request.a"))
		}); err != nil {
			return err
		}
		if err := repo.InsertOutboxEvent(outer, outboxEvent(tid, "orca.request.b")); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) || outboxCount(t, f) != 0 {
		t.Fatalf("outer failure must undo the nested write too: err=%v rows=%d", err, outboxCount(t, f))
	}
}

func TestOutbox_WriteOutsideTxIsRejectedOrScoped(t *testing.T) {
	f := newMigratedPostgres(t)
	repo := New(f.app)
	tid := uuid.NewString()
	if err := repo.InsertOutboxEvent(context.Background(), outboxEvent(tid, "orca.request.x")); err == nil {
		t.Fatal("insert without tenant must fail")
	}
	// Event for another tenant than the ctx tenant must fail rather than be written under the wrong GUC.
	ctx := tenant.WithTenantID(context.Background(), uuid.NewString())
	if err := repo.InsertOutboxEvent(ctx, outboxEvent(tid, "orca.request.x")); err == nil {
		t.Fatal("insert for foreign tenant must fail")
	}
	if outboxCount(t, f) != 0 {
		t.Fatal("no row should have been written")
	}
}

func TestFetchUnpublished_OrderedByCreatedAtThenSeq(t *testing.T) {
	f := newMigratedPostgres(t)
	repo := New(f.app)
	tid := uuid.NewString()
	ctx := tenant.WithTenantID(context.Background(), tid)
	var want []string
	for i := 0; i < 10; i++ {
		ev := outboxEvent(tid, fmt.Sprintf("orca.request.n%d", i))
		want = append(want, ev.Subject)
		if err := repo.InTx(ctx, func(txCtx context.Context) error { return repo.InsertOutboxEvent(txCtx, ev) }); err != nil {
			t.Fatal(err)
		}
	}
	recs, err := repo.FetchUnpublished(context.Background(), 100)
	if err != nil || len(recs) != 10 {
		t.Fatalf("fetch = %d %v", len(recs), err)
	}
	for i, r := range recs {
		if r.Subject != want[i] {
			t.Fatalf("row %d subject %s, want %s", i, r.Subject, want[i])
		}
		if r.Event.TenantID != tid || r.Event.ID != r.ID || r.Event.Version != 1 || r.Event.OccurredAt.IsZero() {
			t.Fatalf("event metadata missing: %+v", r.Event)
		}
	}
	if limited, _ := repo.FetchUnpublished(context.Background(), 3); len(limited) != 3 {
		t.Fatalf("limit not honored: %d", len(limited))
	}
}

func TestOutbox_OrderPreservedWithinOneTransaction(t *testing.T) {
	f := newMigratedPostgres(t)
	repo := New(f.app)
	tid := uuid.NewString()
	ctx := tenant.WithTenantID(context.Background(), tid)
	for round := 0; round < 50; round++ {
		a, b := fmt.Sprintf("orca.request.r%d.first", round), fmt.Sprintf("orca.request.r%d.second", round)
		err := repo.InTx(ctx, func(txCtx context.Context) error {
			if err := repo.InsertOutboxEvent(txCtx, outboxEvent(tid, a)); err != nil {
				return err
			}
			return repo.InsertOutboxEvent(txCtx, outboxEvent(tid, b))
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	recs, err := repo.FetchUnpublished(context.Background(), 1000)
	if err != nil || len(recs) != 100 {
		t.Fatalf("fetch = %d %v", len(recs), err)
	}
	for round := 0; round < 50; round++ {
		if recs[2*round].Subject != fmt.Sprintf("orca.request.r%d.first", round) || recs[2*round+1].Subject != fmt.Sprintf("orca.request.r%d.second", round) {
			t.Fatalf("round %d out of order: %s, %s", round, recs[2*round].Subject, recs[2*round+1].Subject)
		}
	}
}

func TestMarkPublished_ExcludesFromFetch(t *testing.T) {
	f := newMigratedPostgres(t)
	repo := New(f.app)
	tid := uuid.NewString()
	ctx := tenant.WithTenantID(context.Background(), tid)
	for i := 0; i < 3; i++ {
		if err := repo.InTx(ctx, func(txCtx context.Context) error {
			return repo.InsertOutboxEvent(txCtx, outboxEvent(tid, "orca.request.x"))
		}); err != nil {
			t.Fatal(err)
		}
	}
	recs, _ := repo.FetchUnpublished(context.Background(), 10)
	if err := repo.MarkPublished(context.Background(), []string{recs[0].ID, recs[1].ID}); err != nil {
		t.Fatal(err)
	}
	left, err := repo.FetchUnpublished(context.Background(), 10)
	if err != nil || len(left) != 1 || left[0].ID != recs[2].ID {
		t.Fatalf("after mark: %d rows %v", len(left), err)
	}
}

func TestMarkPublished_Empty(t *testing.T) {
	f := newMigratedPostgres(t)
	if err := New(f.app).MarkPublished(context.Background(), nil); err != nil {
		t.Fatalf("empty MarkPublished must not fail: %v", err)
	}
}

func TestRLS_TenantIsolation_Outbox(t *testing.T) {
	f := newMigratedPostgres(t)
	repo := New(f.app)
	tidA, tidB := uuid.NewString(), uuid.NewString()
	for _, tid := range []string{tidA, tidB} {
		ctx := tenant.WithTenantID(context.Background(), tid)
		if err := repo.InTx(ctx, func(txCtx context.Context) error {
			return repo.InsertOutboxEvent(txCtx, outboxEvent(tid, "orca.request.x"))
		}); err != nil {
			t.Fatal(err)
		}
	}
	// Direct SQL as the non-superuser role: tenant A sees only its own row.
	withTenant(t, f.app, tidA, func(tx pgx.Tx) {
		if n := countRows(t, tx, "outbox_events"); n != 1 {
			t.Errorf("tenant A sees %d outbox rows, want 1", n)
		}
	})
}

func TestRelayCanReadAcrossTenants(t *testing.T) {
	f := newMigratedPostgres(t)
	repo := New(f.app)
	for i := 0; i < 2; i++ {
		tid := uuid.NewString()
		ctx := tenant.WithTenantID(context.Background(), tid)
		if err := repo.InTx(ctx, func(txCtx context.Context) error {
			return repo.InsertOutboxEvent(txCtx, outboxEvent(tid, "orca.request.x"))
		}); err != nil {
			t.Fatal(err)
		}
	}
	recs, err := repo.FetchUnpublished(context.Background(), 10)
	if err != nil || len(recs) != 2 {
		t.Fatalf("relay must read both tenants: %d %v", len(recs), err)
	}
}

func TestTwoRelaysConcurrentlyNoLoss(t *testing.T) {
	f := newMigratedPostgres(t)
	repo := New(f.app)
	natsURL := testutil.StartNATS(t)
	pub, consumer, closeBus, err := eventbus.Connect(context.Background(), natsURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = closeBus() })
	_ = consumer
	if err := pub.EnsureStream(context.Background(), "REQUEST", []string{"orca.request.>"}); err != nil {
		t.Fatal(err)
	}

	tid := uuid.NewString()
	ctx := tenant.WithTenantID(context.Background(), tid)
	const total = 40
	for i := 0; i < total; i++ {
		if err := repo.InTx(ctx, func(txCtx context.Context) error {
			return repo.InsertOutboxEvent(txCtx, outboxEvent(tid, "orca.request.request.created"))
		}); err != nil {
			t.Fatal(err)
		}
	}

	cfg := outbox.Config{PollInterval: 50 * time.Millisecond, BatchSize: 7}
	relayCtx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		r := outbox.NewRelay(repo, pub, cfg, nil)
		go func() { defer wg.Done(); r.Run(relayCtx) }()
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		var pending int
		if err := f.admin.QueryRow(context.Background(), `SELECT count(*) FROM request.outbox_events WHERE published_at IS NULL`).Scan(&pending); err != nil {
			t.Fatal(err)
		}
		if pending == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	cancel()
	wg.Wait()
	var pending int
	_ = f.admin.QueryRow(context.Background(), `SELECT count(*) FROM request.outbox_events WHERE published_at IS NULL`).Scan(&pending)
	if pending != 0 {
		t.Fatalf("%d outbox rows never published", pending)
	}
}
