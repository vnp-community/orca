//go:build integration

// Integration tests run against a real Postgres via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/postgres/...`.
package postgres

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/usecase"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := testutil.StartPostgres(t, "ai_provider")

	migrationsPath, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	// Uses the golang-migrate CLI directly rather than importing the
	// library, keeping this test's dependency footprint minimal — swap for
	// the library-based runner once the shared migration-runner helper
	// (referenced in architecture/05-data-architecture.md) exists in common/.
	//
	// Retried a few times: testutil.StartPostgres's wait strategy
	// (wait.ForListeningPort) can observe the container's brief
	// initdb-restart TCP listen before Postgres is actually accepting
	// connections ("the database system is starting up"), independent of
	// anything this test does — same known Postgres-in-Docker race other
	// services' integration suites hit.
	var out []byte
	var runErr error
	for attempt := 0; attempt < 5; attempt++ {
		cmd := exec.Command("migrate", "-path", migrationsPath, "-database", dsn, "up")
		out, runErr = cmd.CombinedOutput()
		if runErr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if runErr != nil {
		t.Fatalf("running migrations: %v\n%s", runErr, out)
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

func newTestAccount(t *testing.T, id, tenantID string, provider domain.ProviderType, status domain.AccountStatus, credRef string, scope domain.AccountScope, userID, projectID, devServerID string, label string, quotaLimitDay int, isDefault bool, now time.Time) domain.ProviderAccount {
	t.Helper()
	account, err := domain.NewProviderAccount(id, tenantID, provider, status, credRef, scope, userID, projectID, devServerID,
		label, "", "", quotaLimitDay, nil, isDefault, nil, "", nil, now, now)
	if err != nil {
		t.Fatalf("building account: %v", err)
	}
	return account
}

func TestRepository_CreateAndGet_RoundTrips(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	account := newTestAccount(t, "11111111-1111-1111-1111-111111111112", "11111111-1111-1111-1111-111111111111",
		domain.ProviderTypeAnthropic, domain.AccountStatusPending, "cred-ref-1", domain.ScopeServer, "", "", "dev-1", "", 0, false, now)
	if err := repo.Create(ctx, account); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.Get(ctx, account.TenantID, account.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.CredentialRef != "cred-ref-1" {
		t.Errorf("expected credential_ref to round-trip, got %q", got.CredentialRef)
	}
	if got.Status != domain.AccountStatusPending {
		t.Errorf("expected status pending, got %q", got.Status)
	}
}

func TestRepository_List_FiltersByTenantAndScope(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	userAccount := newTestAccount(t, "11111111-1111-1111-1111-111111111113", "11111111-1111-1111-1111-111111111111",
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref-1",
		domain.ScopeUser, "22222222-2222-2222-2222-222222222222", "", "dev-1", "", 0, false, now)
	otherTenant := newTestAccount(t, "11111111-1111-1111-1111-111111111114", "33333333-3333-3333-3333-333333333333",
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref-2",
		domain.ScopeUser, "22222222-2222-2222-2222-222222222222", "", "dev-1", "", 0, false, now)
	_ = repo.Create(ctx, userAccount)
	_ = repo.Create(ctx, otherTenant)

	accounts, err := repo.List(ctx, usecase.ListAccountsFilter{
		TenantID: "11111111-1111-1111-1111-111111111111", Scope: domain.ScopeUser,
		ScopeRefID: "22222222-2222-2222-2222-222222222222",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(accounts) != 1 || accounts[0].ID != userAccount.ID {
		t.Errorf("expected only the matching tenant's user-scope account, got %+v", accounts)
	}
}

// TestRepository_List_FiltersByProviderType is TASK-AIP-02-04's own test:
// seed one Anthropic and one OpenAI account at the same scope; List with
// ProviderType: Anthropic must return only the Anthropic account.
func TestRepository_List_FiltersByProviderType(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	tenantID := "11111111-1111-1111-1111-111111111111"

	anthropic := newTestAccount(t, "11111111-1111-1111-1111-111111111115", tenantID,
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref-a", domain.ScopeServer, "", "", "dev-1", "", 0, false, now)
	openai := newTestAccount(t, "11111111-1111-1111-1111-111111111116", tenantID,
		domain.ProviderTypeOpenAI, domain.AccountStatusActive, "cred-ref-o", domain.ScopeServer, "", "", "dev-1", "", 0, false, now)
	_ = repo.Create(ctx, anthropic)
	_ = repo.Create(ctx, openai)

	accounts, err := repo.List(ctx, usecase.ListAccountsFilter{TenantID: tenantID, Scope: domain.ScopeServer, ProviderType: domain.ProviderTypeAnthropic})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(accounts) != 1 || accounts[0].ID != anthropic.ID {
		t.Errorf("expected only the Anthropic account, got %+v", accounts)
	}
}

func TestRepository_UpdateStatus_UpdatesCredentialRefOnRotation(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	account := newTestAccount(t, "11111111-1111-1111-1111-111111111117", "11111111-1111-1111-1111-111111111111",
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref-old", domain.ScopeServer, "", "", "dev-1", "", 0, false, now)
	_ = repo.Create(ctx, account)

	updated, err := repo.UpdateStatus(ctx, usecase.UpdateStatusInput{
		TenantID: account.TenantID, AccountID: account.ID,
		Status: domain.AccountStatusRotating, CredentialRef: "cred-ref-new",
	})
	if err != nil {
		t.Fatalf("update status: %v", err)
	}
	if updated.Status != domain.AccountStatusRotating || updated.CredentialRef != "cred-ref-new" {
		t.Errorf("expected rotating status + new credential_ref, got %+v", updated)
	}
}

// TestRepository_Update_RoundTripsLabelModelHintBaseURL is TASK-AIP-01-07's
// regression guard: Update's SET clause already wrote label/model_hint/
// base_url, but its RETURNING list omitted them, so a write was immediately
// discarded on read-back.
func TestRepository_Update_RoundTripsLabelModelHintBaseURL(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	account := newTestAccount(t, "11111111-1111-1111-1111-111111111118", "11111111-1111-1111-1111-111111111111",
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref-1", domain.ScopeServer, "", "", "dev-1", "", 0, false, now)
	_ = repo.Create(ctx, account)

	updated, err := repo.Update(ctx, usecase.UpdateFields{
		TenantID: account.TenantID, AccountID: account.ID,
		Label: "my label", ModelHint: "claude-3-5-sonnet", BaseURL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Label != "my label" || updated.ModelHint != "claude-3-5-sonnet" || updated.BaseURL != "https://example.com" {
		t.Errorf("expected label/model_hint/base_url to round-trip on the RETURNING row, got %+v", updated)
	}

	// Read back via Get too, to prove it's actually persisted, not just
	// echoed by RETURNING.
	got, err := repo.Get(ctx, account.TenantID, account.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Label != "my label" || got.ModelHint != "claude-3-5-sonnet" || got.BaseURL != "https://example.com" {
		t.Errorf("expected label/model_hint/base_url to persist, got %+v", got)
	}
}

// TestRepository_Create_DefaultDemotion exercises the demote-before-insert
// path: creating a second is_default account for the same
// tenant/dev_server/provider demotes the first rather than violating the
// partial unique index.
func TestRepository_Create_DefaultDemotion(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	tenantID := "11111111-1111-1111-1111-111111111111"

	first := newTestAccount(t, "11111111-1111-1111-1111-111111111119", tenantID,
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref-1", domain.ScopeServer, "", "", "dev-1", "", 0, true, now)
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("create first: %v", err)
	}

	second := newTestAccount(t, "11111111-1111-1111-1111-11111111111a", tenantID,
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref-2", domain.ScopeServer, "", "", "dev-1", "", 0, true, now)
	if err := repo.Create(ctx, second); err != nil {
		t.Fatalf("create second (should demote first, not fail): %v", err)
	}

	gotFirst, err := repo.Get(ctx, tenantID, first.ID)
	if err != nil {
		t.Fatalf("get first: %v", err)
	}
	if gotFirst.IsDefault {
		t.Error("expected the first account to have been demoted when the second was created as default")
	}
	gotSecond, err := repo.Get(ctx, tenantID, second.ID)
	if err != nil {
		t.Fatalf("get second: %v", err)
	}
	if !gotSecond.IsDefault {
		t.Error("expected the second account to be the current default")
	}
}

// TestRepository_UniqueDefaultIndex_RejectsRawDoubleDefaultInsert is the DB
// defense-in-depth half of default demotion: a raw INSERT of a second
// is_default row for the same tenant/dev_server/provider that bypasses
// Create's demote-before-insert step must still be rejected by
// uq_accounts_one_default_per_dev_server_provider.
func TestRepository_UniqueDefaultIndex_RejectsRawDoubleDefaultInsert(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	tenantID := "11111111-1111-1111-1111-111111111111"

	first := newTestAccount(t, "11111111-1111-1111-1111-11111111111d", tenantID,
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref-1", domain.ScopeServer, "", "", "dev-1", "", 0, true, now)
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("create first: %v", err)
	}

	// Bypass Create's demotion step entirely — a raw INSERT straight
	// against the table, exactly like a bug in Create's demotion logic
	// would produce.
	_, err := repo.pool.Exec(ctx, `
		INSERT INTO ai_provider.accounts (
			id, tenant_id, provider_type, status, credential_ref, scope, user_id, project_id,
			dev_server_id, label, model_hint, base_url, quota_limit_day, models, is_default,
			created_by, rotation_grace_until, created_at, updated_at
		) VALUES ($1,$2,'anthropic','active','cred-ref-2','server',NULL,NULL,'dev-1','','','',0,'{}',true,NULL,NULL,$3,$3)
	`, "11111111-1111-1111-1111-11111111111e", tenantID, now)
	if err == nil {
		t.Fatal("expected the partial unique index to reject a raw double-default insert")
	}
}

// TestRepository_Outbox_EnqueueFetchMarkPublished exercises Create's
// same-transaction outbox write, FetchUnpublished, and MarkPublished — the
// transactional-outbox contract this task adds.
func TestRepository_Outbox_EnqueueFetchMarkPublished(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	tenantID := "11111111-1111-1111-1111-111111111111"

	account := newTestAccount(t, "11111111-1111-1111-1111-11111111111b", tenantID,
		domain.ProviderTypeAnthropic, domain.AccountStatusPending, "cred-ref-1", domain.ScopeServer, "", "", "dev-1", "", 0, false, now)
	if err := repo.Create(ctx, account); err != nil {
		t.Fatalf("create: %v", err)
	}

	unpublished, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("fetch unpublished: %v", err)
	}
	var found bool
	for _, rec := range unpublished {
		if rec.Subject == "ai_provider.account.registered" && rec.Event.TenantID == tenantID {
			found = true
			if err := repo.MarkPublished(ctx, []string{rec.ID}); err != nil {
				t.Fatalf("mark published: %v", err)
			}
		}
	}
	if !found {
		t.Fatal("expected an unpublished ai_provider.account.registered event right after Create")
	}

	stillUnpublished, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("fetch unpublished (2nd): %v", err)
	}
	for _, rec := range stillUnpublished {
		if rec.Event.TenantID == tenantID && rec.Subject == "ai_provider.account.registered" {
			t.Errorf("expected the marked-published event to no longer appear, got %+v", rec)
		}
	}
}

// TestRepository_Create_RollsBackAccountInsertIfOutboxInsertFails proves
// Create's account insert and outbox insert share one transaction: a
// trigger-injected failure on the outbox insert must roll back the account
// row too, not leave an account with no corresponding outbox event.
func TestRepository_Create_RollsBackAccountInsertIfOutboxInsertFails(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	tenantID := "11111111-1111-1111-1111-111111111111"

	_, err := repo.pool.Exec(ctx, `
		CREATE OR REPLACE FUNCTION ai_provider.fail_outbox_insert() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'injected outbox failure for test';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER fail_outbox_trigger BEFORE INSERT ON ai_provider.outbox
		FOR EACH ROW EXECUTE FUNCTION ai_provider.fail_outbox_insert();
	`)
	if err != nil {
		t.Fatalf("installing failure trigger: %v", err)
	}

	account := newTestAccount(t, "11111111-1111-1111-1111-11111111111f", tenantID,
		domain.ProviderTypeAnthropic, domain.AccountStatusPending, "cred-ref-1", domain.ScopeServer, "", "", "dev-1", "", 0, false, now)
	if err := repo.Create(ctx, account); err == nil {
		t.Fatal("expected Create to fail when the outbox insert fails")
	}

	if _, err := repo.Get(ctx, tenantID, account.ID); err != domain.ErrAccountNotFound {
		t.Errorf("expected the account insert to have been rolled back too, got err=%v", err)
	}
}

// TestClaimDue_NoDoubleClaimUnderConcurrency is §8's core correctness
// requirement: two goroutines calling ClaimDue against the same due rows
// concurrently must never claim the same row. Uses an explicit barrier
// (rather than just launching two goroutines and hoping the scheduler
// overlaps them) so the two SELECT ... FOR UPDATE SKIP LOCKED transactions
// are provably open at the same time — claim A, hold it uncommitted, THEN
// claim B while A's locks are still held, THEN commit both.
func TestClaimDue_NoDoubleClaimUnderConcurrency(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	tenantID := "11111111-1111-1111-1111-111111111111"

	for i := 0; i < 10; i++ {
		account := newTestAccount(t, fmt.Sprintf("22222222-2222-2222-2222-2222222222%02d", i), tenantID,
			domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref", domain.ScopeServer, "", "", "dev-1", "", 0, false, now.Add(-time.Hour))
		if err := repo.Create(ctx, account); err != nil {
			t.Fatalf("seed account %d: %v", i, err)
		}
	}

	claimedA := make(chan []domain.ProviderAccount, 1)
	releaseA := make(chan struct{})
	var batchA usecase.ClaimedHealthCheckBatch
	go func() {
		var err error
		batchA, err = repo.ClaimDue(ctx, now, 15*time.Minute, 5)
		if err != nil {
			t.Errorf("claim A: %v", err)
			close(claimedA)
			return
		}
		claimedA <- batchA.Accounts()
		<-releaseA // hold the transaction open (locks held) until told to commit
		_ = batchA.Commit(ctx)
	}()

	accountsA := <-claimedA
	batchB, err := repo.ClaimDue(ctx, now, 15*time.Minute, 5)
	if err != nil {
		t.Fatalf("claim B (while A's tx is still open): %v", err)
	}
	accountsB := batchB.Accounts()
	close(releaseA)
	if err := batchB.Commit(ctx); err != nil {
		t.Fatalf("commit B: %v", err)
	}

	claimedIDs := make(map[string]bool)
	for _, acc := range accountsA {
		claimedIDs[acc.ID] = true
	}
	for _, acc := range accountsB {
		if claimedIDs[acc.ID] {
			t.Errorf("account %s claimed by both A and B while A's transaction was still open", acc.ID)
		}
	}
	if len(accountsA) == 0 || len(accountsB) == 0 {
		t.Fatalf("expected both claims to find due accounts (A=%d, B=%d)", len(accountsA), len(accountsB))
	}
}

// TestIncrementUsage_AdditiveAcrossCalls: three calls of 100 tokens each in
// the same day must total 300, not 100.
func TestIncrementUsage_AdditiveAcrossCalls(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	tenantID := "11111111-1111-1111-1111-111111111111"

	account := newTestAccount(t, "11111111-1111-1111-1111-11111111111c", tenantID,
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref", domain.ScopeServer, "", "", "dev-1", "", 0, false, now)
	if err := repo.Create(ctx, account); err != nil {
		t.Fatalf("create: %v", err)
	}

	day := domain.DayKey(now)
	var state domain.QuotaState
	var err error
	for i := 0; i < 3; i++ {
		state, err = repo.IncrementUsage(ctx, tenantID, account.ID, day, 100, 1, 0.01)
		if err != nil {
			t.Fatalf("increment usage call %d: %v", i, err)
		}
	}
	if state.TokensUsed != 300 {
		t.Errorf("expected 300 tokens_used after three additive calls of 100, got %d", state.TokensUsed)
	}
	if state.RequestCount != 3 {
		t.Errorf("expected request_count=3, got %d", state.RequestCount)
	}
}
