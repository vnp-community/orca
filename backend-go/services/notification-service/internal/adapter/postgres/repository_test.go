//go:build integration

// Integration tests run against a real Postgres via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/postgres/...`.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := testutil.StartPostgres(t, "notification")

	migrationsPath, err := filepath.Abs("../../../migrations/postgres")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	cmd := exec.Command("migrate", "-path", migrationsPath, "-database", dsn, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running migrations: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting to postgres: %v", err)
	}
	t.Cleanup(pool.Close)

	return New(pool)
}

// Subscription/tenant/user IDs below must be real UUIDs — push_subscriptions'
// id/tenant_id/user_id are UUID columns (migration 0001). Fixed as part of
// CR-MOBILE-001's MarkExpired work: these 5 tests previously used
// placeholder strings ("sub-1", "tenant-1", ...) that fail
// "invalid input syntax for type uuid" against the real schema — confirmed
// by running them here for the first time under `-tags=integration` — so
// none of them had ever actually passed against Postgres before.
const (
	subTestSubID1      = "d0000000-0000-0000-0000-000000000001"
	subTestSubID1Retry = "d0000000-0000-0000-0000-000000000002"
	subTestSubID2      = "d0000000-0000-0000-0000-000000000003"
	subTestTenant1     = "e0000000-0000-0000-0000-000000000001"
	subTestTenant2     = "e0000000-0000-0000-0000-000000000002"
	subTestUser1       = "f0000000-0000-0000-0000-000000000001"
)

func TestRepository_SaveSubscription_UpsertsOnEndpoint(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	p256dh, auth := "p256dh-1", "auth-1"
	sub, err := domain.NewPushSubscription(subTestSubID1, subTestTenant1, subTestUser1, domain.ChannelWeb,
		"https://push.example/ep-1", &p256dh, &auth, "chrome", time.Now())
	if err != nil {
		t.Fatalf("building subscription: %v", err)
	}
	if err := repo.Save(ctx, sub); err != nil {
		t.Fatalf("first save: %v", err)
	}

	// Re-subscribing to the same endpoint with a new subscription ID must
	// update in place, not create a second row (endpoint UNIQUE index).
	sub.ID = subTestSubID1Retry
	if err := repo.Save(ctx, sub); err != nil {
		t.Fatalf("second save (retry): %v", err)
	}

	subs, err := repo.ListByUser(ctx, subTestTenant1, subTestUser1)
	if err != nil {
		t.Fatalf("list by user: %v", err)
	}
	if len(subs) != 1 {
		t.Errorf("expected exactly 1 subscription after upsert, got %d", len(subs))
	}
}

func TestRepository_ListByUser_FiltersByTenantAndUser(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	p256dh, auth := "p", "a"
	s1, err := domain.NewPushSubscription(subTestSubID1, subTestTenant1, subTestUser1, domain.ChannelWeb, "https://push.example/ep-1", &p256dh, &auth, "", time.Now())
	if err != nil {
		t.Fatalf("building s1: %v", err)
	}
	s2, err := domain.NewPushSubscription(subTestSubID2, subTestTenant2, subTestUser1, domain.ChannelWeb, "https://push.example/ep-2", &p256dh, &auth, "", time.Now())
	if err != nil {
		t.Fatalf("building s2: %v", err)
	}
	if err := repo.Save(ctx, s1); err != nil {
		t.Fatalf("save s1: %v", err)
	}
	if err := repo.Save(ctx, s2); err != nil {
		t.Fatalf("save s2: %v", err)
	}

	subs, err := repo.ListByUser(ctx, subTestTenant1, subTestUser1)
	if err != nil {
		t.Fatalf("list by user: %v", err)
	}
	if len(subs) != 1 || subs[0].TenantID != subTestTenant1 {
		t.Errorf("expected only tenant1's subscription, got %+v", subs)
	}
}

func TestRepository_MarkExpired_SetsStatusExpired(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	p256dh, auth := "p256dh-1", "auth-1"
	sub, err := domain.NewPushSubscription(subTestSubID1, subTestTenant1, subTestUser1, domain.ChannelWeb,
		"https://push.example/ep-expiring", &p256dh, &auth, "chrome", time.Now())
	if err != nil {
		t.Fatalf("building subscription: %v", err)
	}
	if err := repo.Save(ctx, sub); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := repo.MarkExpired(ctx, "https://push.example/ep-expiring"); err != nil {
		t.Fatalf("MarkExpired: %v", err)
	}

	var status string
	if err := repo.pool.QueryRow(ctx, `SELECT status FROM notification.push_subscriptions WHERE endpoint = $1`, "https://push.example/ep-expiring").Scan(&status); err != nil {
		t.Fatalf("querying status: %v", err)
	}
	if status != string(domain.SubscriptionExpired) {
		t.Errorf("expected status %q, got %q", domain.SubscriptionExpired, status)
	}

	// ListByUser only returns 'active' subscriptions — an expired one must
	// no longer show up, matching DeliverPush's expectation that a future
	// event won't retry it.
	subs, err := repo.ListByUser(ctx, subTestTenant1, subTestUser1)
	if err != nil {
		t.Fatalf("list by user: %v", err)
	}
	if len(subs) != 0 {
		t.Errorf("expected 0 active subscriptions after MarkExpired, got %d: %+v", len(subs), subs)
	}
}

func TestRepository_MarkExpired_UnknownEndpoint_NoError(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	// Idempotent-by-design, same contract as DeleteByEndpoint — 0 rows
	// affected is not an error.
	if err := repo.MarkExpired(ctx, "https://push.example/never-existed"); err != nil {
		t.Fatalf("expected no error for an unknown endpoint, got: %v", err)
	}
}

func TestRepository_MarkExpired_DoesNotAffectOtherEndpoints(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	p256dh, auth := "p", "a"
	kept, err := domain.NewPushSubscription(subTestSubID1, subTestTenant1, subTestUser1, domain.ChannelWeb, "https://push.example/keep", &p256dh, &auth, "", time.Now())
	if err != nil {
		t.Fatalf("building kept: %v", err)
	}
	expiring, err := domain.NewPushSubscription(subTestSubID2, subTestTenant1, subTestUser1, domain.ChannelWeb, "https://push.example/expire", &p256dh, &auth, "", time.Now())
	if err != nil {
		t.Fatalf("building expiring: %v", err)
	}
	if err := repo.Save(ctx, kept); err != nil {
		t.Fatalf("save kept: %v", err)
	}
	if err := repo.Save(ctx, expiring); err != nil {
		t.Fatalf("save expiring: %v", err)
	}

	if err := repo.MarkExpired(ctx, "https://push.example/expire"); err != nil {
		t.Fatalf("MarkExpired: %v", err)
	}

	subs, err := repo.ListByUser(ctx, subTestTenant1, subTestUser1)
	if err != nil {
		t.Fatalf("list by user: %v", err)
	}
	if len(subs) != 1 || subs[0].Endpoint != "https://push.example/keep" {
		t.Errorf("expected only the non-expired endpoint to remain active, got %+v", subs)
	}
}

func TestRepository_GetPublicKey_NoActiveKeyReturnsDomainError(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	_, err := repo.GetPublicKey(ctx, "tenant-with-no-key")
	if err != domain.ErrNoActiveVapidKey {
		t.Fatalf("expected domain.ErrNoActiveVapidKey, got %v", err)
	}
}

// TestRepository_MarkProcessed_FirstCallReservesEventID verifies the happy
// path: a never-before-seen event ID is reserved and reported as new.
func TestRepository_MarkProcessed_FirstCallReservesEventID(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	eventID := "11111111-1111-1111-1111-111111111111"
	alreadyProcessed, err := repo.MarkProcessed(ctx, eventID, "orca.task.task.completed")
	if err != nil {
		t.Fatalf("mark processed: %v", err)
	}
	if alreadyProcessed {
		t.Errorf("expected alreadyProcessed=false for a never-before-seen event ID")
	}
}

// TestRepository_MarkProcessed_RedeliveryIsDetected verifies the
// atomic-dedup contract ProcessedEventRepository documents: a second
// MarkProcessed call for the same event ID (simulating JetStream
// redelivery, or a concurrent replica racing the same message via its own
// independent SubscribeEphemeral consumer) reports alreadyProcessed=true
// via the INSERT ... ON CONFLICT DO NOTHING path, not an error.
func TestRepository_MarkProcessed_RedeliveryIsDetected(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	eventID := "22222222-2222-2222-2222-222222222222"
	if _, err := repo.MarkProcessed(ctx, eventID, "orca.task.task.completed"); err != nil {
		t.Fatalf("first mark processed: %v", err)
	}

	alreadyProcessed, err := repo.MarkProcessed(ctx, eventID, "orca.task.task.completed")
	if err != nil {
		t.Fatalf("second mark processed: %v", err)
	}
	if !alreadyProcessed {
		t.Errorf("expected alreadyProcessed=true for a redelivered event ID")
	}
}

// Test tenant/user IDs must be real UUIDs — notification_events.tenant_id
// and .recipient_user_id are UUID columns (migration 0003), unlike some
// pre-existing tests elsewhere in this file that predate this task and use
// non-UUID placeholder strings against other tables.
const (
	notifTestTenant      = "10000000-0000-0000-0000-000000000001"
	notifTestTenantWrong = "20000000-0000-0000-0000-000000000002"
	notifTestUserA       = "a0000000-0000-0000-0000-00000000000a"
	notifTestUserB       = "b0000000-0000-0000-0000-00000000000b"
	notifTestUserC       = "c0000000-0000-0000-0000-00000000000c"
)

func newTestNotificationEvent(id, tenantID string, recipients []string, createdAt time.Time) domain.NotificationEvent {
	return domain.NotificationEvent{
		ID:               id,
		TenantID:         tenantID,
		RecipientUserIDs: recipients,
		SourceEventID:    "src-" + id,
		SourceSubject:    "orca.task.task.completed",
		Type:             "task_completed",
		Title:            "Task completed",
		Body:             "Your task has finished.",
		Severity:         domain.SeverityInfo,
		Channels:         []domain.DeliveryChannel{domain.ChannelDeliveryWS},
		CreatedAt:        createdAt,
	}
}

func TestRepository_SaveNotificationEvent_OneRowPerRecipient(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	event := newTestNotificationEvent("11111111-1111-1111-1111-111111111111", notifTestTenant,
		[]string{notifTestUserA, notifTestUserB, notifTestUserC}, time.Now())
	if err := repo.SaveNotificationEvent(ctx, event); err != nil {
		t.Fatalf("save notification event: %v", err)
	}

	for _, user := range []string{notifTestUserA, notifTestUserB, notifTestUserC} {
		count, err := repo.CountUnread(ctx, notifTestTenant, user)
		if err != nil {
			t.Fatalf("count unread for %s: %v", user, err)
		}
		if count != 1 {
			t.Errorf("expected exactly 1 unread row for recipient %s, got %d", user, count)
		}
	}
}

func TestRepository_MarkAsRead_ScopedByTenantAndUser(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	event := newTestNotificationEvent("22222222-2222-2222-2222-222222222222", notifTestTenant,
		[]string{notifTestUserA, notifTestUserB}, time.Now())
	if err := repo.SaveNotificationEvent(ctx, event); err != nil {
		t.Fatalf("save notification event: %v", err)
	}

	if err := repo.MarkAsRead(ctx, notifTestTenant, notifTestUserA, event.ID); err != nil {
		t.Fatalf("mark as read: %v", err)
	}

	countA, err := repo.CountUnread(ctx, notifTestTenant, notifTestUserA)
	if err != nil {
		t.Fatalf("count unread user-a: %v", err)
	}
	if countA != 0 {
		t.Errorf("expected user-a's row to be read (0 unread), got %d", countA)
	}

	countB, err := repo.CountUnread(ctx, notifTestTenant, notifTestUserB)
	if err != nil {
		t.Fatalf("count unread user-b: %v", err)
	}
	if countB != 1 {
		t.Errorf("expected user-b's row to remain unread (same notification ID, different recipient row), got %d", countB)
	}
}

func TestRepository_MarkAsRead_WrongTenant_NoOp(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	event := newTestNotificationEvent("33333333-3333-3333-3333-333333333333", notifTestTenant,
		[]string{notifTestUserA}, time.Now())
	if err := repo.SaveNotificationEvent(ctx, event); err != nil {
		t.Fatalf("save notification event: %v", err)
	}

	if err := repo.MarkAsRead(ctx, notifTestTenantWrong, notifTestUserA, event.ID); err != nil {
		t.Fatalf("mark as read with wrong tenant should be a no-op, not an error: %v", err)
	}

	count, err := repo.CountUnread(ctx, notifTestTenant, notifTestUserA)
	if err != nil {
		t.Fatalf("count unread: %v", err)
	}
	if count != 1 {
		t.Errorf("expected row to remain unread when tenant doesn't match, got count=%d", count)
	}
}

func TestRepository_MarkAsRead_Idempotent(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	event := newTestNotificationEvent("44444444-4444-4444-4444-444444444444", notifTestTenant,
		[]string{notifTestUserA}, time.Now())
	if err := repo.SaveNotificationEvent(ctx, event); err != nil {
		t.Fatalf("save notification event: %v", err)
	}

	if err := repo.MarkAsRead(ctx, notifTestTenant, notifTestUserA, event.ID); err != nil {
		t.Fatalf("first mark as read: %v", err)
	}
	if err := repo.MarkAsRead(ctx, notifTestTenant, notifTestUserA, event.ID); err != nil {
		t.Fatalf("second mark as read should not error: %v", err)
	}
}

func TestRepository_MarkAllAsRead_ReturnsCorrectCount(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		event := newTestNotificationEvent(
			[]string{"55555555-5555-5555-5555-555555555551", "55555555-5555-5555-5555-555555555552", "55555555-5555-5555-5555-555555555553"}[i],
			notifTestTenant, []string{notifTestUserA}, time.Now())
		if err := repo.SaveNotificationEvent(ctx, event); err != nil {
			t.Fatalf("save notification event %d: %v", i, err)
		}
	}
	// An unrelated user's unread row must not be counted/marked.
	other := newTestNotificationEvent("66666666-6666-6666-6666-666666666666", notifTestTenant, []string{notifTestUserB}, time.Now())
	if err := repo.SaveNotificationEvent(ctx, other); err != nil {
		t.Fatalf("save other user's event: %v", err)
	}

	n, err := repo.MarkAllAsRead(ctx, notifTestTenant, notifTestUserA)
	if err != nil {
		t.Fatalf("mark all as read: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3 rows marked read, got %d", n)
	}

	countB, err := repo.CountUnread(ctx, notifTestTenant, notifTestUserB)
	if err != nil {
		t.Fatalf("count unread user-b: %v", err)
	}
	if countB != 1 {
		t.Errorf("expected user-b's unread row untouched, got count=%d", countB)
	}
}

func TestRepository_CountUnread_MatchesListUnreadOnlyLength(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		id := []string{
			"77777777-7777-7777-7777-777777777771", "77777777-7777-7777-7777-777777777772",
			"77777777-7777-7777-7777-777777777773", "77777777-7777-7777-7777-777777777774",
			"77777777-7777-7777-7777-777777777775",
		}[i]
		event := newTestNotificationEvent(id, notifTestTenant, []string{notifTestUserA}, time.Now().Add(time.Duration(i)*time.Millisecond))
		if err := repo.SaveNotificationEvent(ctx, event); err != nil {
			t.Fatalf("save notification event %d: %v", i, err)
		}
	}
	// Mark one read so unread count != total row count (a stronger check
	// than counting all rows).
	if err := repo.MarkAsRead(ctx, notifTestTenant, notifTestUserA, "77777777-7777-7777-7777-777777777771"); err != nil {
		t.Fatalf("mark as read: %v", err)
	}

	count, err := repo.CountUnread(ctx, notifTestTenant, notifTestUserA)
	if err != nil {
		t.Fatalf("count unread: %v", err)
	}

	var total int
	cursor := ""
	for {
		page, next, err := repo.ListByRecipient(ctx, notifTestTenant, notifTestUserA, cursor, 2, true)
		if err != nil {
			t.Fatalf("list by recipient: %v", err)
		}
		total += len(page)
		if next == "" {
			break
		}
		cursor = next
	}

	if int64(total) != count {
		t.Errorf("CountUnread=%d does not match total unread rows across ListByRecipient pages=%d", count, total)
	}
	if count != 4 {
		t.Errorf("expected 4 unread rows (5 saved, 1 marked read), got %d", count)
	}
}

func TestRepository_ListByRecipient_OrderedByCreatedAtDesc(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	base := time.Now()
	ids := []string{
		"88888888-8888-8888-8888-888888888881",
		"88888888-8888-8888-8888-888888888882",
		"88888888-8888-8888-8888-888888888883",
	}
	for i, id := range ids {
		event := newTestNotificationEvent(id, notifTestTenant, []string{notifTestUserA}, base.Add(time.Duration(i)*time.Second))
		if err := repo.SaveNotificationEvent(ctx, event); err != nil {
			t.Fatalf("save notification event %d: %v", i, err)
		}
	}

	page, _, err := repo.ListByRecipient(ctx, notifTestTenant, notifTestUserA, "", 10, false)
	if err != nil {
		t.Fatalf("list by recipient: %v", err)
	}
	if len(page) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(page))
	}
	// Newest (highest offset) first.
	if page[0].ID != ids[2] || page[1].ID != ids[1] || page[2].ID != ids[0] {
		t.Errorf("expected order [%s,%s,%s], got [%s,%s,%s]",
			ids[2], ids[1], ids[0], page[0].ID, page[1].ID, page[2].ID)
	}
}

func TestRepository_ListByRecipient_CursorPaginationNoDuplicateNoGap(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	base := time.Now()
	var ids []string
	for i := 0; i < 7; i++ {
		id := fmt.Sprintf("99999999-9999-9999-9999-9999999999%02d", i)
		ids = append(ids, id)
		event := newTestNotificationEvent(id, notifTestTenant, []string{notifTestUserA}, base.Add(time.Duration(i)*time.Second))
		if err := repo.SaveNotificationEvent(ctx, event); err != nil {
			t.Fatalf("save notification event %d: %v", i, err)
		}
	}

	seen := map[string]bool{}
	var order []string
	cursor := ""
	for pageNum := 0; ; pageNum++ {
		if pageNum > 10 {
			t.Fatalf("too many pages, possible infinite loop")
		}
		page, next, err := repo.ListByRecipient(ctx, notifTestTenant, notifTestUserA, cursor, 3, false)
		if err != nil {
			t.Fatalf("list by recipient page %d: %v", pageNum, err)
		}
		for _, e := range page {
			if seen[e.ID] {
				t.Fatalf("duplicate notification %s across pages", e.ID)
			}
			seen[e.ID] = true
			order = append(order, e.ID)
		}
		if next == "" {
			break
		}
		cursor = next
	}

	if len(order) != len(ids) {
		t.Fatalf("expected %d total notifications across all pages, got %d", len(ids), len(order))
	}
	// order is newest-first; ids was built oldest-first, so reverse-compare.
	for i, id := range order {
		want := ids[len(ids)-1-i]
		if id != want {
			t.Errorf("position %d: want %s, got %s", i, want, id)
		}
	}
}

func TestRepository_ListByRecipient_InvalidCursorReturnsDomainError(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	_, _, err := repo.ListByRecipient(ctx, notifTestTenant, notifTestUserA, "not-a-valid-cursor!!", 10, false)
	if !errors.Is(err, domain.ErrInvalidCursor) {
		t.Fatalf("expected domain.ErrInvalidCursor, got %v", err)
	}
}
