package usecase

import (
	"context"
	"errors"
	"testing"
)

func TestGetUnreadCount_RequiresTenantAndUser(t *testing.T) {
	uc := NewGetUnreadCount(&fakeNotificationRepository{})

	if _, err := uc.Execute(context.Background(), "user-1"); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}

	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, ""); err == nil {
		t.Fatal("expected an error when user_id is empty")
	}
}

func TestGetUnreadCount_ReturnsCountFromRepo(t *testing.T) {
	repo := &fakeNotificationRepository{countUnreadResult: 4}
	uc := NewGetUnreadCount(repo)
	ctx := withTenant(context.Background(), "tenant-1")

	count, err := uc.Execute(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 4 {
		t.Errorf("expected count=4 from repo, got %d", count)
	}
	if repo.lastCountTenantID != "tenant-1" || repo.lastCountUserID != "user-1" {
		t.Errorf("expected repo called with (tenant-1, user-1), got (%s, %s)", repo.lastCountTenantID, repo.lastCountUserID)
	}
}

func TestGetUnreadCount_RepoErrorMapsToInternal(t *testing.T) {
	repo := &fakeNotificationRepository{countUnreadErr: errors.New("boom")}
	uc := NewGetUnreadCount(repo)
	ctx := withTenant(context.Background(), "tenant-1")

	if _, err := uc.Execute(ctx, "user-1"); err == nil {
		t.Fatal("expected an error when repo fails")
	}
}
