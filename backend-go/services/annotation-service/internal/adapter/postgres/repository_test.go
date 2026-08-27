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
	"github.com/stablyai/orca-go/services/annotation-service/internal/domain"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := testutil.StartPostgres(t, "annotation")

	migrationsPath, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	// Uses the golang-migrate CLI directly rather than importing the
	// library, keeping this test's dependency footprint minimal — swap for
	// the library-based runner once the shared migration-runner helper
	// (referenced in architecture/05-data-architecture.md) exists in common/.
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

func TestRepository_CreateAndListAnnotations_FiltersByTenant(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now()

	anchor1 := domain.Anchor{RepoID: "repo-1", FilePath: "main.go", Line: 10, Ref: "abc"}
	a1, err := domain.NewAnnotation("11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333", anchor1, "comment 1", false, "req-1", now, now)
	if err != nil {
		t.Fatalf("building annotation: %v", err)
	}
	if _, err := repo.CreateAnnotation(ctx, a1); err != nil {
		t.Fatalf("create annotation: %v", err)
	}

	anchor2 := domain.Anchor{RepoID: "repo-1", FilePath: "main.go", Line: 20, Ref: "abc"}
	a2, err := domain.NewAnnotation("44444444-4444-4444-4444-444444444444", "55555555-5555-5555-5555-555555555555", "33333333-3333-3333-3333-333333333333", anchor2, "comment 2", false, "req-2", now, now)
	if err != nil {
		t.Fatalf("building annotation: %v", err)
	}
	if _, err := repo.CreateAnnotation(ctx, a2); err != nil {
		t.Fatalf("create annotation: %v", err)
	}

	annotations, _, err := repo.ListAnnotations(ctx, a1.TenantID, "repo-1", "", "", 50)
	if err != nil {
		t.Fatalf("list annotations: %v", err)
	}
	if len(annotations) != 1 || annotations[0].TenantID != a1.TenantID {
		t.Errorf("expected only %s's annotation, got %+v", a1.TenantID, annotations)
	}
}

func TestRepository_UpdateAnnotation_NotFoundReturnsSentinel(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	_, err := repo.UpdateAnnotation(ctx, "22222222-2222-2222-2222-222222222222", "99999999-9999-9999-9999-999999999999", "edited", true)
	if err != domain.ErrAnnotationNotFound {
		t.Errorf("expected ErrAnnotationNotFound, got %v", err)
	}
}

func TestRepository_DeleteAnnotation_NotFoundReturnsSentinel(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	err := repo.DeleteAnnotation(ctx, "22222222-2222-2222-2222-222222222222", "99999999-9999-9999-9999-999999999999")
	if err != domain.ErrAnnotationNotFound {
		t.Errorf("expected ErrAnnotationNotFound, got %v", err)
	}
}

// TestRepository_FindByRequestID_RoundTripsAndScopesToTenant exercises the
// (tenant_id, request_id) unique constraint from migration
// 0002_annotation_request_id against a real Postgres — CreateAnnotation's
// idempotency check (internal/usecase.CreateAnnotation) depends on this.
func TestRepository_FindByRequestID_RoundTripsAndScopesToTenant(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now()

	anchor := domain.Anchor{RepoID: "repo-1", FilePath: "main.go", Line: 10, Ref: "abc"}
	a, err := domain.NewAnnotation("66666666-6666-6666-6666-666666666666", "22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333", anchor, "comment", false, "req-findme", now, now)
	if err != nil {
		t.Fatalf("building annotation: %v", err)
	}
	if _, err := repo.CreateAnnotation(ctx, a); err != nil {
		t.Fatalf("create annotation: %v", err)
	}

	found, ok, err := repo.FindByRequestID(ctx, a.TenantID, "req-findme")
	if err != nil {
		t.Fatalf("find by request id: %v", err)
	}
	if !ok || found.ID != a.ID {
		t.Errorf("expected to find %s, got found=%v ok=%v", a.ID, found, ok)
	}

	// Scoped to tenant: the same request_id under a different tenant must
	// not match (mirrors the UNIQUE(tenant_id, request_id) constraint).
	_, ok, err = repo.FindByRequestID(ctx, "77777777-7777-7777-7777-777777777777", "req-findme")
	if err != nil {
		t.Fatalf("find by request id (other tenant): %v", err)
	}
	if ok {
		t.Error("expected no match for a different tenant with the same request_id")
	}

	_, ok, err = repo.FindByRequestID(ctx, a.TenantID, "req-does-not-exist")
	if err != nil {
		t.Fatalf("find by request id (missing): %v", err)
	}
	if ok {
		t.Error("expected no match for an unknown request_id")
	}
}
