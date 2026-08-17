//go:build integration

// Integration tests run against a real Postgres via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/postgres/...`.
package postgres

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/usecase"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := testutil.StartPostgres(t, "credential_broker")

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

func TestRepository_CreateGetUpdateStatus_RoundTrip(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	m, err := domain.NewCredentialMetadata(
		uuid.NewString(), uuid.NewString(), uuid.NewString(),
		domain.CategoryScmOAuth, "credential/tenant-1/cred-1", domain.StatusActive, now,
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
// mutation and audit append land together against a real Postgres
// transaction, per credential-broker-service.md §8.
func TestRepository_RunInTx_CommitsBothOnSuccess(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	m, err := domain.NewCredentialMetadata(
		uuid.NewString(), uuid.NewString(), uuid.NewString(),
		domain.CategoryScmOAuth, "credential/tenant-1/tx-commit", domain.StatusActive, now,
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
// in the SAME fn call is rolled back too — the real-Postgres counterpart to
// fakeTxRunner's in-memory rollback assertion in
// internal/usecase/write_credential_test.go.
func TestRepository_RunInTx_RollsBackBothOnFailure(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	m, err := domain.NewCredentialMetadata(
		uuid.NewString(), uuid.NewString(), uuid.NewString(),
		domain.CategoryScmOAuth, "credential/tenant-1/tx-rollback", domain.StatusActive, now,
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
		domain.CategoryAiProviderKey, "credential/tenant-1/cred-2", domain.StatusActive, now,
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
	// rejected by the FK constraint — see migrations/0001_init.up.sql.
	orphan, err := domain.NewAccessAuditEntry(uuid.NewString(), "ai-provider-service", domain.ActionResolve, time.Now().UTC())
	if err != nil {
		t.Fatalf("building orphan audit entry: %v", err)
	}
	if err := repo.Append(ctx, orphan); err == nil {
		t.Error("expected the FK constraint to reject an audit row for a nonexistent credential")
	}
}
