package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/notification-service/internal/adapter/broadcaster"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

func TestMarkAsRead_RequiresTenantAndFields(t *testing.T) {
	uc := NewMarkAsRead(&fakeNotificationRepository{}, &fakeBroadcaster{})

	if err := uc.Execute(context.Background(), "user-1", "notif-1"); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}

	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, "", "notif-1"); err == nil {
		t.Fatal("expected an error when user_id is empty")
	}
	if err := uc.Execute(ctx, "user-1", ""); err == nil {
		t.Fatal("expected an error when notification_id is empty")
	}
}

func TestMarkAsRead_CallsRepoWithCorrectTenantAndUser(t *testing.T) {
	repo := &fakeNotificationRepository{}
	uc := NewMarkAsRead(repo, &fakeBroadcaster{})
	ctx := withTenant(context.Background(), "tenant-1")

	if err := uc.Execute(ctx, "user-1", "notif-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastMarkAsReadTenantID != "tenant-1" || repo.lastMarkAsReadUserID != "user-1" || repo.lastMarkAsReadID != "notif-1" {
		t.Errorf("expected repo called with (tenant-1, user-1, notif-1), got (%s, %s, %s)",
			repo.lastMarkAsReadTenantID, repo.lastMarkAsReadUserID, repo.lastMarkAsReadID)
	}
}

func TestMarkAsRead_RepoErrorMapsToInternal(t *testing.T) {
	repo := &fakeNotificationRepository{markAsReadErr: errors.New("boom")}
	uc := NewMarkAsRead(repo, &fakeBroadcaster{})
	ctx := withTenant(context.Background(), "tenant-1")

	if err := uc.Execute(ctx, "user-1", "notif-1"); err == nil {
		t.Fatal("expected an error when repo fails")
	}
}

func TestMarkAsRead_BroadcastsReadReceiptAfterPersist(t *testing.T) {
	repo := &fakeNotificationRepository{}
	fb := &fakeBroadcaster{}
	uc := NewMarkAsRead(repo, fb)
	ctx := withTenant(context.Background(), "tenant-1")

	if err := uc.Execute(ctx, "user-1", "notif-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fb.broadcast) != 1 {
		t.Fatalf("expected exactly 1 broadcast, got %d", len(fb.broadcast))
	}
	got := fb.broadcast[0]
	if got.Type != "notification_read" || !got.IsRead || got.ID != "notif-1" {
		t.Errorf("unexpected broadcast event: %+v", got)
	}
	if len(got.RecipientUserIDs) != 1 || got.RecipientUserIDs[0] != "user-1" {
		t.Errorf("expected RecipientUserIDs=[user-1], got %+v", got.RecipientUserIDs)
	}
}

func TestMarkAsRead_PersistFails_DoesNotBroadcast(t *testing.T) {
	repo := &fakeNotificationRepository{markAsReadErr: errors.New("boom")}
	fb := &fakeBroadcaster{}
	uc := NewMarkAsRead(repo, fb)
	ctx := withTenant(context.Background(), "tenant-1")

	if err := uc.Execute(ctx, "user-1", "notif-1"); err == nil {
		t.Fatal("expected an error when repo fails")
	}
	if len(fb.broadcast) != 0 {
		t.Errorf("expected no broadcast when persist fails, got %d", len(fb.broadcast))
	}
}

// TestMarkAsRead_TwoSubscribersSameUser_BothReceiveReadReceipt uses the
// REAL broadcaster.Broadcaster (not a fake) — this is the direct test for
// CR-NOTIF-001's acceptance criterion "MarkAsRead trên tab A phản ánh tới
// tab B", mirroring broadcaster_test.go's own subscribe-then-assert pattern.
func TestMarkAsRead_TwoSubscribersSameUser_BothReceiveReadReceipt(t *testing.T) {
	repo := &fakeNotificationRepository{}
	real := broadcaster.New()
	uc := NewMarkAsRead(repo, real)
	ctx := withTenant(context.Background(), "tenant-1")

	ch1, unsub1 := real.Subscribe(ctx, "tenant-1", "user-1")
	defer unsub1()
	ch2, unsub2 := real.Subscribe(ctx, "tenant-1", "user-1")
	defer unsub2()

	if err := uc.Execute(ctx, "user-1", "notif-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertReceivesReadReceipt := func(t *testing.T, ch <-chan domain.NotificationEvent) {
		t.Helper()
		select {
		case e := <-ch:
			if e.Type != "notification_read" || e.ID != "notif-1" {
				t.Errorf("unexpected event: %+v", e)
			}
		default:
			t.Error("expected a read-receipt event, got none")
		}
	}
	assertReceivesReadReceipt(t, ch1)
	assertReceivesReadReceipt(t, ch2)
}
