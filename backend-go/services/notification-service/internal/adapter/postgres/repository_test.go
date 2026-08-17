//go:build integration

// Integration tests run against a real Postgres via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/postgres/...`.
package postgres

import (
	"context"
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

	migrationsPath, err := filepath.Abs("../../../migrations")
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

func TestRepository_SaveSubscription_UpsertsOnEndpoint(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	p256dh, auth := "p256dh-1", "auth-1"
	sub, err := domain.NewPushSubscription("sub-1", "tenant-1", "user-1", domain.ChannelWeb,
		"https://push.example/ep-1", &p256dh, &auth, "chrome", time.Now())
	if err != nil {
		t.Fatalf("building subscription: %v", err)
	}
	if err := repo.Save(ctx, sub); err != nil {
		t.Fatalf("first save: %v", err)
	}

	// Re-subscribing to the same endpoint with a new subscription ID must
	// update in place, not create a second row (endpoint UNIQUE index).
	sub.ID = "sub-1-retry"
	if err := repo.Save(ctx, sub); err != nil {
		t.Fatalf("second save (retry): %v", err)
	}

	subs, err := repo.ListByUser(ctx, "tenant-1", "user-1")
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
	s1, _ := domain.NewPushSubscription("s1", "tenant-1", "user-1", domain.ChannelWeb, "https://push.example/ep-1", &p256dh, &auth, "", time.Now())
	s2, _ := domain.NewPushSubscription("s2", "tenant-2", "user-1", domain.ChannelWeb, "https://push.example/ep-2", &p256dh, &auth, "", time.Now())
	_ = repo.Save(ctx, s1)
	_ = repo.Save(ctx, s2)

	subs, err := repo.ListByUser(ctx, "tenant-1", "user-1")
	if err != nil {
		t.Fatalf("list by user: %v", err)
	}
	if len(subs) != 1 || subs[0].TenantID != "tenant-1" {
		t.Errorf("expected only tenant-1's subscription, got %+v", subs)
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
