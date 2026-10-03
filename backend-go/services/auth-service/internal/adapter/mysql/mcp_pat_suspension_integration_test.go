//go:build integration

package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMcpPatSuspension_SetClearAndTenantIsolation(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	a, b := uuid.NewString(), uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)

	if on, err := repo.IsMcpPatSuspended(ctx, a); err != nil || on {
		t.Fatalf("fresh tenant: on=%v err=%v", on, err)
	}
	if err := repo.SetMcpPatSuspension(ctx, a, true, "incident", now); err != nil {
		t.Fatal(err)
	}
	// Idempotent upsert.
	if err := repo.SetMcpPatSuspension(ctx, a, true, "still incident", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if on, err := repo.IsMcpPatSuspended(ctx, a); err != nil || !on {
		t.Fatalf("suspended tenant: on=%v err=%v", on, err)
	}
	if on, err := repo.IsMcpPatSuspended(ctx, b); err != nil || on {
		t.Fatalf("other tenant must be unaffected: on=%v err=%v", on, err)
	}
	if err := repo.SetMcpPatSuspension(ctx, a, false, "", now); err != nil {
		t.Fatal(err)
	}
	// Clearing twice (retried cleanup) is not an error.
	if err := repo.SetMcpPatSuspension(ctx, a, false, "", now); err != nil {
		t.Fatal(err)
	}
	if on, err := repo.IsMcpPatSuspended(ctx, a); err != nil || on {
		t.Fatalf("cleared tenant: on=%v err=%v", on, err)
	}
}
