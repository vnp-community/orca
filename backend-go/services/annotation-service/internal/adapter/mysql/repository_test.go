//go:build integration

// Integration tests run against a real MySQL via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/mysql/...`. Mirrors
// internal/adapter/postgres/repository_test.go's test names/shape 1:1,
// plus TestRepository_ListAnnotations_DoesNotLeakAcrossTenants
// (TASK-BE-DB-003's pattern, mirrored per TASK-BE-DB-010) — annotation
// data is per-tenant and the Postgres migration declares an RLS policy,
// so this proves application-layer tenant_id scoping alone is sufficient
// on a dialect with NO RLS equivalent at all.
package mysql

import (
	"context"
	"database/sql"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/annotation-service/internal/domain"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	// testutil.StartMySQL returns "mysql://root:orca@tcp(host:port)/db" —
	// valid as-is for golang-migrate's mysql driver CLI (used below), but
	// go-sql-driver/mysql's database/sql driver expects its OWN DSN format
	// with no "mysql://" scheme (see usage-service's toMySQLDriverDSN for
	// the general-purpose conversion; this fixed-shape test DSN only needs
	// the prefix stripped). parseTime=true is required for TIMESTAMP
	// columns to scan into time.Time/sql.NullTime instead of []byte.
	rawDSN := testutil.StartMySQL(t, "annotation")
	driverDSN := strings.TrimPrefix(rawDSN, "mysql://") + "?parseTime=true"

	migrationsPath, err := filepath.Abs("../../../migrations/mysql")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	cmd := exec.Command("migrate", "-path", migrationsPath, "-database", rawDSN, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running migrations: %v\n%s", err, out)
	}

	db, err := sql.Open("mysql", driverDSN)
	if err != nil {
		t.Fatalf("connecting to mysql: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("pinging mysql: %v", err)
	}

	return New(db)
}

func TestRepository_CreateAndListAnnotations_FiltersByTenant(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now()

	anchor1 := domain.Anchor{RepoID: "repo-1", FilePath: "main.go", Line: 10, Ref: "abc"}
	a1, err := domain.NewAnnotation("11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333", anchor1, "comment 1", "", false, "req-1", now, now)
	if err != nil {
		t.Fatalf("building annotation: %v", err)
	}
	if _, err := repo.CreateAnnotation(ctx, a1); err != nil {
		t.Fatalf("create annotation: %v", err)
	}

	anchor2 := domain.Anchor{RepoID: "repo-1", FilePath: "main.go", Line: 20, Ref: "abc"}
	a2, err := domain.NewAnnotation("44444444-4444-4444-4444-444444444444", "55555555-5555-5555-5555-555555555555", "33333333-3333-3333-3333-333333333333", anchor2, "comment 2", "", false, "req-2", now, now)
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

// TestRepository_ListAnnotations_DoesNotLeakAcrossTenants is the
// TASK-BE-DB-003-pattern test this task requires: two tenants each with
// their own annotation on the SAME repo_id/file_path/line neighborhood,
// proving ListAnnotations's `WHERE tenant_id = ?` scoping alone — with no
// RLS equivalent available in MySQL at all — is sufficient isolation.
func TestRepository_ListAnnotations_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now()
	tenantA := "aaaaaaaa-0000-4000-8000-000000000001"
	tenantB := "bbbbbbbb-0000-4000-8000-000000000002"
	authorID := "cccccccc-0000-4000-8000-000000000003"

	anchor := domain.Anchor{RepoID: "shared-repo", FilePath: "shared.go", Line: 1, Ref: "abc"}
	aA, err := domain.NewAnnotation("aaaaaaaa-1111-4000-8000-000000000010", tenantA, authorID, anchor, "tenant A comment", "", false, "req-a", now, now)
	if err != nil {
		t.Fatalf("building tenant-a annotation: %v", err)
	}
	if _, err := repo.CreateAnnotation(ctx, aA); err != nil {
		t.Fatalf("saving tenant-a annotation: %v", err)
	}

	aB, err := domain.NewAnnotation("bbbbbbbb-1111-4000-8000-000000000020", tenantB, authorID, anchor, "tenant B comment", "", false, "req-b", now, now)
	if err != nil {
		t.Fatalf("building tenant-b annotation: %v", err)
	}
	if _, err := repo.CreateAnnotation(ctx, aB); err != nil {
		t.Fatalf("saving tenant-b annotation: %v", err)
	}

	annotations, _, err := repo.ListAnnotations(ctx, tenantA, "shared-repo", "", "", 100)
	if err != nil {
		t.Fatalf("listing tenant-a annotations: %v", err)
	}
	for _, a := range annotations {
		if a.TenantID != tenantA {
			t.Fatalf("ListAnnotations(tenant-a) leaked a row from tenant %q — application-layer scoping failed, MySQL has no RLS backstop at all (see BE-DB-SOL-001 §4, TASK-BE-DB-003)", a.TenantID)
		}
	}
	if len(annotations) != 1 || annotations[0].ID != aA.ID {
		t.Fatalf("expected exactly tenant-a's annotation, got %+v", annotations)
	}

	// GetAnnotation must also refuse to cross tenants: fetching tenant-b's
	// id under tenant-a's scope must behave exactly like "not found", not
	// leak tenant-b's row.
	if _, err := repo.GetAnnotation(ctx, tenantA, aB.ID); err != domain.ErrAnnotationNotFound {
		t.Fatalf("GetAnnotation(tenant-a, tenant-b's id) = %v, want ErrAnnotationNotFound — cross-tenant read must not succeed", err)
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

// TestRepository_UpdateAnnotation_NoopRetryStillSucceeds guards the MySQL-
// specific pitfall UpdateAnnotation's doc comment describes: resubmitting
// the SAME content/resolved values MySQL's default RowsAffected() reports
// 0 "changed" rows for a real, matched row — this must still return the
// row, not domain.ErrAnnotationNotFound.
func TestRepository_UpdateAnnotation_NoopRetryStillSucceeds(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now()

	anchor := domain.Anchor{RepoID: "repo-1", FilePath: "main.go", Line: 10, Ref: "abc"}
	a, err := domain.NewAnnotation("11111111-2222-4000-8000-000000000099", "22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333", anchor, "original", "", false, "req-noop", now, now)
	if err != nil {
		t.Fatalf("building annotation: %v", err)
	}
	if _, err := repo.CreateAnnotation(ctx, a); err != nil {
		t.Fatalf("create annotation: %v", err)
	}

	first, err := repo.UpdateAnnotation(ctx, a.TenantID, a.ID, "edited", true)
	if err != nil {
		t.Fatalf("first update: %v", err)
	}

	// Same content/resolved as `first` — MySQL's default driver semantics
	// would report 0 rows "changed" here even though the row exists.
	second, err := repo.UpdateAnnotation(ctx, a.TenantID, a.ID, "edited", true)
	if err != nil {
		t.Fatalf("no-op retry update: %v (want success, not ErrAnnotationNotFound)", err)
	}
	if second.ID != first.ID || second.Content != "edited" || !second.Resolved {
		t.Errorf("expected no-op retry to return the unchanged row, got %+v", second)
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
// 0002_annotation_request_id against a real MySQL — CreateAnnotation's
// idempotency check (internal/usecase.CreateAnnotation) depends on this.
func TestRepository_FindByRequestID_RoundTripsAndScopesToTenant(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now()

	anchor := domain.Anchor{RepoID: "repo-1", FilePath: "main.go", Line: 10, Ref: "abc"}
	a, err := domain.NewAnnotation("66666666-6666-6666-6666-666666666666", "22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333", anchor, "comment", "", false, "req-findme", now, now)
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

// TestRepository_MarkSent_UpdatesExactlyGivenIDsAndSkipsMissing exercises
// MarkSent against a real MySQL: it must update exactly the given ids in
// one statement and silently skip a nonexistent id.
func TestRepository_MarkSent_UpdatesExactlyGivenIDsAndSkipsMissing(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now()
	tenantID := "22222222-2222-2222-2222-222222222222"

	anchor1 := domain.Anchor{RepoID: "repo-1", FilePath: "main.go", Line: 10, Ref: "abc"}
	a1, err := domain.NewAnnotation("77777777-7777-7777-7777-777777777771", tenantID, "33333333-3333-3333-3333-333333333333", anchor1, "comment 1", "", false, "req-mark-1", now, now)
	if err != nil {
		t.Fatalf("building annotation: %v", err)
	}
	if _, err := repo.CreateAnnotation(ctx, a1); err != nil {
		t.Fatalf("create annotation: %v", err)
	}

	anchor2 := domain.Anchor{RepoID: "repo-1", FilePath: "main.go", Line: 20, Ref: "abc"}
	a2, err := domain.NewAnnotation("77777777-7777-7777-7777-777777777772", tenantID, "33333333-3333-3333-3333-333333333333", anchor2, "comment 2", "", false, "req-mark-2", now, now)
	if err != nil {
		t.Fatalf("building annotation: %v", err)
	}
	if _, err := repo.CreateAnnotation(ctx, a2); err != nil {
		t.Fatalf("create annotation: %v", err)
	}

	sentAt := time.Now().UTC()
	// Include one nonexistent id — must be silently skipped, not an error.
	updated, err := repo.MarkSent(ctx, tenantID, []string{a1.ID, "99999999-9999-9999-9999-999999999999"}, sentAt)
	if err != nil {
		t.Fatalf("mark sent: %v", err)
	}
	if len(updated) != 1 || updated[0].ID != a1.ID {
		t.Fatalf("expected exactly a1 to be updated, got %+v", updated)
	}
	if !updated[0].SentToAgent || updated[0].SentAt == nil {
		t.Errorf("expected SentToAgent=true and SentAt set, got %+v", updated[0])
	}

	// a2 must remain untouched.
	a2After, err := repo.GetAnnotation(ctx, tenantID, a2.ID)
	if err != nil {
		t.Fatalf("get a2: %v", err)
	}
	if a2After.SentToAgent {
		t.Error("expected a2 to remain unsent")
	}
}

// TestRepository_MarkSent_EmptyIDsIsNoop mirrors usage-service's
// MarkPublished([]string{})/nil no-op guard — MySQL's `IN ()` is a syntax
// error, unlike Postgres's `= ANY('{}')`, so this adapter must short-
// circuit before building the query.
func TestRepository_MarkSent_EmptyIDsIsNoop(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	if updated, err := repo.MarkSent(ctx, "22222222-2222-2222-2222-222222222222", nil, time.Now()); err != nil || updated != nil {
		t.Fatalf("MarkSent(nil) should be a no-op, got updated=%v err=%v", updated, err)
	}
	if updated, err := repo.MarkSent(ctx, "22222222-2222-2222-2222-222222222222", []string{}, time.Now()); err != nil || updated != nil {
		t.Fatalf("MarkSent([]string{}) should be a no-op, got updated=%v err=%v", updated, err)
	}
}
