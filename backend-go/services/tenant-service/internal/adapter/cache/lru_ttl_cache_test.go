package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func TestLRUTTLCache_SetThenGet(t *testing.T) {
	c := NewLRUTTLCache(10)
	ctx := context.Background()
	profile := domain.ResolvedProfile{Settings: domain.Settings{"agent": "sonnet"}}

	c.Set(ctx, "user-1", profile, time.Minute)

	got, ok := c.Get(ctx, "user-1")
	if !ok {
		t.Fatal("expected a cache hit")
	}
	if got.Settings["agent"] != "sonnet" {
		t.Errorf("unexpected cached value: %v", got.Settings)
	}
}

func TestLRUTTLCache_ExpiresAfterTTL(t *testing.T) {
	c := NewLRUTTLCache(10)
	ctx := context.Background()
	c.Set(ctx, "user-1", domain.ResolvedProfile{}, -time.Second) // already expired

	if _, ok := c.Get(ctx, "user-1"); ok {
		t.Error("expected an expired entry to be a cache miss")
	}
}

func TestLRUTTLCache_InvalidateRemovesEntry(t *testing.T) {
	c := NewLRUTTLCache(10)
	ctx := context.Background()
	c.Set(ctx, "user-1", domain.ResolvedProfile{}, time.Minute)

	c.Invalidate(ctx, "user-1")

	if _, ok := c.Get(ctx, "user-1"); ok {
		t.Error("expected the entry to be gone after Invalidate")
	}
}

func TestLRUTTLCache_EvictsLeastRecentlyUsedOverCapacity(t *testing.T) {
	c := NewLRUTTLCache(2)
	ctx := context.Background()

	c.Set(ctx, "user-1", domain.ResolvedProfile{}, time.Minute)
	c.Set(ctx, "user-2", domain.ResolvedProfile{}, time.Minute)
	// Touch user-1 so it's most-recently-used; user-2 becomes the LRU entry.
	c.Get(ctx, "user-1")
	c.Set(ctx, "user-3", domain.ResolvedProfile{}, time.Minute)

	if _, ok := c.Get(ctx, "user-2"); ok {
		t.Error("expected user-2 (least recently used) to have been evicted")
	}
	if _, ok := c.Get(ctx, "user-1"); !ok {
		t.Error("expected user-1 (recently touched) to still be cached")
	}
	if _, ok := c.Get(ctx, "user-3"); !ok {
		t.Error("expected user-3 (just inserted) to be cached")
	}
}

func TestLRUTTLCache_NonPositiveCapacityDefaults(t *testing.T) {
	c := NewLRUTTLCache(0)
	if c.capacity != DefaultCapacity {
		t.Errorf("expected default capacity %d, got %d", DefaultCapacity, c.capacity)
	}
}
