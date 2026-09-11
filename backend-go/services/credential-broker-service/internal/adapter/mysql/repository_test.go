//go:build integration

// Integration tests run against a real MySQL via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/mysql/...`. Mirrors
// internal/adapter/postgres/repository_test.go's test names/shape 1:1,
// plus 2 tenant-isolation tests (GetByOwner/ListByCategory) mirroring
// TASK-BE-DB-003's pattern for usage-service — CR-DB-002's acceptance
// criterion is that tenant isolation holds on a dialect with NO RLS
// equivalent at all, not just Postgres (migrations/postgres/0001_init.up.sql
// has an RLS policy on credential_metadata; migrations/mysql omits it,
// per BE-DB-SOL-001 §4).
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/usecase"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	// testutil.StartMySQL returns "mysql://root:orca@tcp(host:port)/db" —
	// valid as-is for golang-migrate's mysql driver CLI (used below), but
	// go-sql-driver/mysql's database/sql driver expects its OWN DSN format
	// with no "mysql://" scheme. parseTime=true is required for TIMESTAMP
	// columns to scan into time.Time instead of []byte.
	rawDSN := testutil.StartMySQL(t, "credential")
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

func TestRepository_CreateGetUpdateStatus_RoundTrip(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	m, err := domain.NewCredentialMetadata(
		uuid.NewString(), uuid.NewString(), uuid.NewString(),
		domain.CategoryScmOAuth, "credential/tenant-1/cred-1", domain.StatusActive, "", now,
	)
	if err != nil {
		t.Fatalf("building metadata: %v", err)
	}

	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.VaultPath != m.VaultPath || got.Status != domain.StatusActive {
		t.Errorf("expected round-tripped metadata to match, got %+v", got)
	}

	if err := repo.UpdateStatus(ctx, m.ID, domain.StatusRevoked, time.Now().UTC()); err != nil {
		t.Fatalf("update status: %v", err)
	}
	got, err = repo.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Status != domain.StatusRevoked {
		t.Errorf("expected status revoked, got %s", got.Status)
	}
}

func TestRepository_Get_NotFound(t *testing.T) {
	repo := setupRepository(t)
	_, err := repo.Get(context.Background(), uuid.NewString())
	if !errors.Is(err, domain.ErrCredentialNotFound) {
		t.Fatalf("expected ErrCredentialNotFound, got %v", err)
	}
}

// TestRepository_RunInTx_CommitsBothOnSuccess proves RunInTx's metadata
// mutation and audit append land together against a real MySQL
// transaction, per credential-broker-service.md §8 — mirrors
// internal/adapter/postgres/repository_test.go's test of the same name.
func TestRepository_RunInTx_CommitsBothOnSuccess(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	m, err := domain.NewCredentialMetadata(
		uuid.NewString(), uuid.NewString(), uuid.NewString(),
		domain.CategoryScmOAuth, "credential/tenant-1/tx-commit", domain.StatusActive, "", now,
	)
	if err != nil {
		t.Fatalf("building metadata: %v", err)
	}

	err = repo.RunInTx(ctx, func(ctx context.Context, metadataRepo usecase.CredentialMetadataRepository, auditRepo usecase.AuditRepository) error {
		if err := metadataRepo.Create(ctx, m); err != nil {
			return err
		}
		entry, err := domain.NewAccessAuditEntry(m.ID, "scm-integration-service", domain.ActionWrite, now)
		if err != nil {
			return err
		}
		return auditRepo.Append(ctx, entry)
	})
	if err != nil {
		t.Fatalf("RunInTx: %v", err)
	}

	if _, err := repo.Get(ctx, m.ID); err != nil {
		t.Errorf("expected the metadata row to be committed, got: %v", err)
	}
}

// TestRepository_RunInTx_RollsBackBothOnFailure proves that when the audit
// append inside RunInTx's fn fails, the metadata mutation that ran earlier
// in the SAME fn call is rolled back too — mirrors
// internal/adapter/postgres/repository_test.go's test of the same name.
func TestRepository_RunInTx_RollsBackBothOnFailure(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	m, err := domain.NewCredentialMetadata(
		uuid.NewString(), uuid.NewString(), uuid.NewString(),
		domain.CategoryScmOAuth, "credential/tenant-1/tx-rollback", domain.StatusActive, "", now,
	)
	if err != nil {
		t.Fatalf("building metadata: %v", err)
	}

	sentinel := errors.New("audit append deliberately failed")
	err = repo.RunInTx(ctx, func(ctx context.Context, metadataRepo usecase.CredentialMetadataRepository, auditRepo usecase.AuditRepository) error {
		if err := metadataRepo.Create(ctx, m); err != nil {
			return err
		}
		// Simulate the audit write failing after the metadata Create
		// already ran, inside the same transaction.
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected RunInTx to propagate the sentinel error, got: %v", err)
	}

	// The metadata Create must have been rolled back along with the
	// (never-attempted) audit append — a real transaction, not two
	// independent statements against the same pool.
	if _, err := repo.Get(ctx, m.ID); !errors.Is(err, domain.ErrCredentialNotFound) {
		t.Fatalf("expected the metadata Create to be rolled back, got err=%v", err)
	}
}

func TestRepository_Append_RequiresExistingCredential(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	m, err := domain.NewCredentialMetadata(
		uuid.NewString(), uuid.NewString(), uuid.NewString(),
		domain.CategoryAiProviderKey, "credential/tenant-1/cred-2", domain.StatusActive, "", now,
	)
	if err != nil {
		t.Fatalf("building metadata: %v", err)
	}
	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("create: %v", err)
	}

	entry, err := domain.NewAccessAuditEntry(m.ID, "ai-provider-service", domain.ActionResolve, time.Now().UTC())
	if err != nil {
		t.Fatalf("building audit entry: %v", err)
	}
	if err := repo.Append(ctx, entry); err != nil {
		t.Fatalf("append: %v", err)
	}

	// A credential_id with no matching credential_metadata row must be
	// rejected by the FK constraint — see migrations/mysql/0001_init.up.sql.
	orphan, err := domain.NewAccessAuditEntry(uuid.NewString(), "ai-provider-service", domain.ActionResolve, time.Now().UTC())
	if err != nil {
		t.Fatalf("building orphan audit entry: %v", err)
	}
	if err := repo.Append(ctx, orphan); err == nil {
		t.Error("expected the FK constraint to reject an audit row for a nonexistent credential")
	}
}

// TestRepository_GetByOwner_DoesNotLeakAcrossTenants proves application-layer
// tenant_id scoping alone is sufficient isolation on a dialect with NO RLS
// equivalent — mirrors TASK-BE-DB-003's usage-service pattern, applied here
// because migrations/postgres/0001_init.up.sql DOES declare an RLS policy
// on credential_metadata (unlike usage-service's tables, which never had
// one) — see BE-DB-SOL-006 §4 for why this test exists for this service.
func TestRepository_GetByOwner_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	tenantA, tenantB := uuid.NewString(), uuid.NewString()
	ownerID := "bitbucket" // real caller shape, not UUID-only — see 0003_owner_id_text

	mA, err := domain.NewCredentialMetadata(uuid.NewString(), tenantA, ownerID, domain.CategoryScmOAuth, "credential/tenant-a/scm", domain.StatusActive, "", now)
	if err != nil {
		t.Fatalf("building tenant-a metadata: %v", err)
	}
	mB, err := domain.NewCredentialMetadata(uuid.NewString(), tenantB, ownerID, domain.CategoryScmOAuth, "credential/tenant-b/scm", domain.StatusActive, "", now)
	if err != nil {
		t.Fatalf("building tenant-b metadata: %v", err)
	}
	if err := repo.Create(ctx, mA); err != nil {
		t.Fatalf("creating tenant-a metadata: %v", err)
	}
	if err := repo.Create(ctx, mB); err != nil {
		t.Fatalf("creating tenant-b metadata: %v", err)
	}

	got, err := repo.GetByOwner(ctx, tenantA, domain.CategoryScmOAuth, ownerID)
	if err != nil {
		t.Fatalf("GetByOwner(tenant-a): %v", err)
	}
	if got.TenantID != tenantA || got.ID != mA.ID {
		t.Fatalf("GetByOwner(tenant-a) leaked tenant %q's row (id=%s) — application-layer scoping failed, MySQL has no RLS backstop at all (see BE-DB-SOL-001 §4)", got.TenantID, got.ID)
	}
}

// TestRepository_ListByCategory_DoesNotLeakAcrossTenants is
// GetByOwner's sibling test for ListByCategory — same rationale.
func TestRepository_ListByCategory_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	tenantA, tenantB := uuid.NewString(), uuid.NewString()

	mA, err := domain.NewCredentialMetadata(uuid.NewString(), tenantA, "jira", domain.CategoryIssueTrackerOAuth, "credential/tenant-a/jira", domain.StatusActive, "", now)
	if err != nil {
		t.Fatalf("building tenant-a metadata: %v", err)
	}
	mB, err := domain.NewCredentialMetadata(uuid.NewString(), tenantB, "jira", domain.CategoryIssueTrackerOAuth, "credential/tenant-b/jira", domain.StatusActive, "", now)
	if err != nil {
		t.Fatalf("building tenant-b metadata: %v", err)
	}
	if err := repo.Create(ctx, mA); err != nil {
		t.Fatalf("creating tenant-a metadata: %v", err)
	}
	if err := repo.Create(ctx, mB); err != nil {
		t.Fatalf("creating tenant-b metadata: %v", err)
	}

	rows, err := repo.ListByCategory(ctx, tenantA, domain.CategoryIssueTrackerOAuth)
	if err != nil {
		t.Fatalf("ListByCategory(tenant-a): %v", err)
	}
	for _, row := range rows {
		if row.TenantID != tenantA {
			t.Fatalf("ListByCategory(tenant-a) leaked a row from tenant %q — application-layer scoping failed, MySQL has no RLS backstop at all (see BE-DB-SOL-001 §4)", row.TenantID)
		}
	}
	if len(rows) != 1 || rows[0].ID != mA.ID {
		t.Fatalf("expected exactly tenant-a's row, got %+v", rows)
	}
}
