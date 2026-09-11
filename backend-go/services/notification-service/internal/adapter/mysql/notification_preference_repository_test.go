//go:build integration

// Mirrors internal/adapter/postgres/notification_preference_repository_test.go
// 1:1 — gated behind the "integration" build tag, run explicitly with
// `go test -tags=integration ./internal/adapter/mysql/...`.
package mysql

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestNotificationPreferenceStore_IsEnabled_DefaultsToTrueWithNoRow(t *testing.T) {
	repo := setupRepository(t)
	store := NewNotificationPreferenceStore(repo.db)
	ctx := context.Background()

	tenantID, userID := uuid.NewString(), uuid.NewString()
	enabled, err := store.IsEnabled(ctx, tenantID, userID, "agent_completed", "web")
	if err != nil {
		t.Fatalf("is enabled: %v", err)
	}
	if !enabled {
		t.Error("expected default-on (enabled=true) when no preference row exists — BR-MB-08")
	}
}

func TestNotificationPreferenceStore_Set_PersistsExplicitOptOut(t *testing.T) {
	repo := setupRepository(t)
	store := NewNotificationPreferenceStore(repo.db)
	ctx := context.Background()

	tenantID, userID := uuid.NewString(), uuid.NewString()
	if err := store.Set(ctx, tenantID, userID, "agent_completed", "web", false); err != nil {
		t.Fatalf("set: %v", err)
	}

	enabled, err := store.IsEnabled(ctx, tenantID, userID, "agent_completed", "web")
	if err != nil {
		t.Fatalf("is enabled: %v", err)
	}
	if enabled {
		t.Error("expected explicit opt-out row to make IsEnabled return false")
	}

	// A different channel for the same user/event is unaffected.
	otherEnabled, err := store.IsEnabled(ctx, tenantID, userID, "agent_completed", "ios")
	if err != nil {
		t.Fatalf("is enabled (other channel): %v", err)
	}
	if !otherEnabled {
		t.Error("expected an unrelated channel to remain default-on")
	}
}

func TestNotificationPreferenceStore_Set_UpsertsOnConflict(t *testing.T) {
	repo := setupRepository(t)
	store := NewNotificationPreferenceStore(repo.db)
	ctx := context.Background()

	tenantID, userID := uuid.NewString(), uuid.NewString()
	if err := store.Set(ctx, tenantID, userID, "agent_completed", "web", false); err != nil {
		t.Fatalf("first set: %v", err)
	}
	if err := store.Set(ctx, tenantID, userID, "agent_completed", "web", true); err != nil {
		t.Fatalf("second set (flip back on): %v", err)
	}

	enabled, err := store.IsEnabled(ctx, tenantID, userID, "agent_completed", "web")
	if err != nil {
		t.Fatalf("is enabled: %v", err)
	}
	if !enabled {
		t.Error("expected the second Set to overwrite the first, not create a duplicate row")
	}
}
