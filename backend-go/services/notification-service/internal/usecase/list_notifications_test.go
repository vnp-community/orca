package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

func TestListNotifications_RequiresTenantContext(t *testing.T) {
	uc := NewListNotifications(&fakeNotificationRepository{})
	_, _, err := uc.Execute(context.Background(), ListNotificationsInput{UserID: "user-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestListNotifications_RequiresUserID(t *testing.T) {
	uc := NewListNotifications(&fakeNotificationRepository{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, _, err := uc.Execute(ctx, ListNotificationsInput{})
	if err == nil {
		t.Fatal("expected an error when user_id is empty")
	}
}

func TestListNotifications_DefaultsLimitWhenZeroOrOutOfRange(t *testing.T) {
	for _, limit := range []int32{0, -1, 101, 1000} {
		repo := &fakeNotificationRepository{}
		uc := NewListNotifications(repo)
		ctx := withTenant(context.Background(), "tenant-1")

		if _, _, err := uc.Execute(ctx, ListNotificationsInput{UserID: "user-1", Limit: limit}); err != nil {
			t.Fatalf("limit=%d: unexpected error: %v", limit, err)
		}
		if repo.lastListLimit != 50 {
			t.Errorf("limit=%d: expected repo called with default limit 50, got %d", limit, repo.lastListLimit)
		}
	}

	// A valid in-range limit must pass through unchanged.
	repo := &fakeNotificationRepository{}
	uc := NewListNotifications(repo)
	ctx := withTenant(context.Background(), "tenant-1")
	if _, _, err := uc.Execute(ctx, ListNotificationsInput{UserID: "user-1", Limit: 10}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastListLimit != 10 {
		t.Errorf("expected repo called with limit=10, got %d", repo.lastListLimit)
	}
}

func TestListNotifications_UnreadOnlyPassedThrough(t *testing.T) {
	repo := &fakeNotificationRepository{}
	uc := NewListNotifications(repo)
	ctx := withTenant(context.Background(), "tenant-1")

	if _, _, err := uc.Execute(ctx, ListNotificationsInput{UserID: "user-1", UnreadOnly: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.lastListUnreadOnly {
		t.Error("expected unreadOnly=true passed through to repo")
	}
	if repo.lastListTenantID != "tenant-1" || repo.lastListUserID != "user-1" {
		t.Errorf("expected tenantID=tenant-1 userID=user-1, got tenantID=%s userID=%s", repo.lastListTenantID, repo.lastListUserID)
	}
}

func TestListNotifications_ReturnsEventsAndCursorFromRepo(t *testing.T) {
	want := []domain.NotificationEvent{{ID: "n1", CreatedAt: time.Now()}}
	repo := &fakeNotificationRepository{listEvents: want, listCursor: "next-cursor"}
	uc := NewListNotifications(repo)
	ctx := withTenant(context.Background(), "tenant-1")

	events, next, err := uc.Execute(ctx, ListNotificationsInput{UserID: "user-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 || events[0].ID != "n1" {
		t.Errorf("expected events from repo passed through, got %+v", events)
	}
	if next != "next-cursor" {
		t.Errorf("expected next cursor from repo passed through, got %q", next)
	}
}

func TestListNotifications_RepoErrorMapsToInternal(t *testing.T) {
	repo := &fakeNotificationRepository{listErr: errors.New("boom")}
	uc := NewListNotifications(repo)
	ctx := withTenant(context.Background(), "tenant-1")

	_, _, err := uc.Execute(ctx, ListNotificationsInput{UserID: "user-1"})
	if err == nil {
		t.Fatal("expected an error when repo fails")
	}
}

func TestListNotifications_InvalidCursorMapsToInvalidArgument(t *testing.T) {
	repo := &fakeNotificationRepository{listErr: domain.ErrInvalidCursor}
	uc := NewListNotifications(repo)
	ctx := withTenant(context.Background(), "tenant-1")

	_, _, err := uc.Execute(ctx, ListNotificationsInput{UserID: "user-1", Cursor: "garbage"})
	if err == nil {
		t.Fatal("expected an error for an invalid cursor")
	}
	if !errors.Is(err, domain.ErrInvalidCursor) {
		t.Errorf("expected error to wrap domain.ErrInvalidCursor, got %v", err)
	}
}
