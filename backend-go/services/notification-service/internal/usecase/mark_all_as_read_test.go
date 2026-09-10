package usecase

import (
	"context"
	"errors"
	"testing"
)

func TestMarkAllAsRead_RequiresTenantAndUser(t *testing.T) {
	uc := NewMarkAllAsRead(&fakeNotificationRepository{}, &fakeBroadcaster{})

	if _, err := uc.Execute(context.Background(), "user-1"); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}

	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, ""); err == nil {
		t.Fatal("expected an error when user_id is empty")
	}
}

func TestMarkAllAsRead_ReturnsCountFromRepo(t *testing.T) {
	repo := &fakeNotificationRepository{markAllAsReadCount: 7}
	uc := NewMarkAllAsRead(repo, &fakeBroadcaster{})
	ctx := withTenant(context.Background(), "tenant-1")

	count, err := uc.Execute(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 7 {
		t.Errorf("expected count=7 from repo, got %d", count)
	}
	if repo.lastMarkAllTenantID != "tenant-1" || repo.lastMarkAllUserID != "user-1" {
		t.Errorf("expected repo called with (tenant-1, user-1), got (%s, %s)", repo.lastMarkAllTenantID, repo.lastMarkAllUserID)
	}
}

func TestMarkAllAsRead_RepoErrorMapsToInternal(t *testing.T) {
	repo := &fakeNotificationRepository{markAllAsReadErr: errors.New("boom")}
	uc := NewMarkAllAsRead(repo, &fakeBroadcaster{})
	ctx := withTenant(context.Background(), "tenant-1")

	if _, err := uc.Execute(ctx, "user-1"); err == nil {
		t.Fatal("expected an error when repo fails")
	}
}

func TestMarkAllAsRead_BroadcastsAllReadReceipt(t *testing.T) {
	repo := &fakeNotificationRepository{markAllAsReadCount: 3}
	fb := &fakeBroadcaster{}
	uc := NewMarkAllAsRead(repo, fb)
	ctx := withTenant(context.Background(), "tenant-1")

	if _, err := uc.Execute(ctx, "user-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fb.broadcast) != 1 {
		t.Fatalf("expected exactly 1 broadcast, got %d", len(fb.broadcast))
	}
	got := fb.broadcast[0]
	if got.Type != "notification_all_read" || !got.IsRead || got.ID != "" {
		t.Errorf("unexpected broadcast event: %+v", got)
	}
	if len(got.RecipientUserIDs) != 1 || got.RecipientUserIDs[0] != "user-1" {
		t.Errorf("expected RecipientUserIDs=[user-1], got %+v", got.RecipientUserIDs)
	}
}

func TestMarkAllAsRead_PersistFails_DoesNotBroadcast(t *testing.T) {
	repo := &fakeNotificationRepository{markAllAsReadErr: errors.New("boom")}
	fb := &fakeBroadcaster{}
	uc := NewMarkAllAsRead(repo, fb)
	ctx := withTenant(context.Background(), "tenant-1")

	if _, err := uc.Execute(ctx, "user-1"); err == nil {
		t.Fatal("expected an error when repo fails")
	}
	if len(fb.broadcast) != 0 {
		t.Errorf("expected no broadcast when persist fails, got %d", len(fb.broadcast))
	}
}
