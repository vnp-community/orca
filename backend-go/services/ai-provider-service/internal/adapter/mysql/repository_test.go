//go:build integration

// Integration tests run against a real MySQL via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/mysql/...`. Mirrors
// internal/adapter/postgres/repository_test.go's test names/shape 1:1,
// plus tenant-isolation-without-RLS tests (List/GetToday) mirroring
// TASK-BE-DB-003's pattern — migrations/postgres/0001_init.up.sql has RLS
// policies on both `accounts` and `usage`; migrations/mysql omits both,
// per BE-DB-SOL-001 §4 — and a no-op-retry regression test for the
// matched-vs-changed RowsAffected() pitfall UpdateStatus/Update avoid (see
// repository.go's doc comments and TASK-BE-DB-010's annotation-service
// finding).
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/usecase"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	// testutil.StartMySQL returns "mysql://root:orca@tcp(host:port)/db" —
	// valid as-is for golang-migrate's mysql driver CLI (used below), but
	// go-sql-driver/mysql's database/sql driver expects its OWN DSN format
	// with no "mysql://" scheme. parseTime=true is required for TIMESTAMP
	// columns to scan into time.Time instead of []byte.
	rawDSN := testutil.StartMySQL(t, "ai_provider")
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

// TestRepository_List_DoesNotLeakAcrossTenants is TASK-BE-DB-003's pattern
// applied to accounts: 2 tenants each get an account at the same scope;
// List with tenant A's id must never return tenant B's row. MySQL has no
// RLS at all (see this file's package doc comment) — application-layer
// tenant_id scoping in every query above is the ONLY thing enforcing this,
// proven here rather than assumed.
func TestRepository_List_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	tenantA := newTestAccount(t, "11111111-1111-1111-1111-111111111113", "11111111-1111-1111-1111-111111111111",
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref-1",
		domain.ScopeUser, "22222222-2222-2222-2222-222222222222", "", "dev-1", "", 0, false, now)
	tenantB := newTestAccount(t, "11111111-1111-1111-1111-111111111114", "33333333-3333-3333-3333-333333333333",
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref-2",
		domain.ScopeUser, "22222222-2222-2222-2222-222222222222", "", "dev-1", "", 0, false, now)
	if err := repo.Create(ctx, tenantA); err != nil {
		t.Fatalf("create tenant A account: %v", err)
	}
	if err := repo.Create(ctx, tenantB); err != nil {
		t.Fatalf("create tenant B account: %v", err)
	}

	accounts, err := repo.List(ctx, usecase.ListAccountsFilter{
		TenantID: "11111111-1111-1111-1111-111111111111", Scope: domain.ScopeUser,
		ScopeRefID: "22222222-2222-2222-2222-222222222222",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(accounts) != 1 || accounts[0].ID != tenantA.ID {
		t.Errorf("expected only tenant A's account, got %+v", accounts)
	}
	for _, a := range accounts {
		if a.TenantID == tenantB.TenantID {
			t.Fatalf("tenant B's account leaked into tenant A's List result: %+v", a)
		}
	}
}

func TestRepository_List_FiltersByProviderType(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	tenantID := "11111111-1111-1111-1111-111111111111"

	anthropic := newTestAccount(t, "11111111-1111-1111-1111-111111111115", tenantID,
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref-a", domain.ScopeServer, "", "", "dev-1", "", 0, false, now)
	openai := newTestAccount(t, "11111111-1111-1111-1111-111111111116", tenantID,
		domain.ProviderTypeOpenAI, domain.AccountStatusActive, "cred-ref-o", domain.ScopeServer, "", "", "dev-1", "", 0, false, now)
	if err := repo.Create(ctx, anthropic); err != nil {
		t.Fatalf("create anthropic: %v", err)
	}
	if err := repo.Create(ctx, openai); err != nil {
		t.Fatalf("create openai: %v", err)
	}

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
	if err := repo.Create(ctx, account); err != nil {
		t.Fatalf("create: %v", err)
	}

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

// TestRepository_UpdateStatus_NoopRetryStillSucceeds is the regression
// guard for the matched-vs-changed RowsAffected() pitfall: calling
// UpdateStatus a second time with the EXACT same status/credential_ref it
// already has must still find and return the row, not misreport
// ErrAccountNotFound because MySQL's default driver counted 0 changed
// rows. A naive "RowsAffected() == 0 -> not found" translation would fail
// this test; repository.go's UpdateStatus deliberately never reads
// RowsAffected() for this reason.
func TestRepository_UpdateStatus_NoopRetryStillSucceeds(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	account := newTestAccount(t, "11111111-1111-1111-1111-111111111120", "11111111-1111-1111-1111-111111111111",
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref-same", domain.ScopeServer, "", "", "dev-1", "", 0, false, now)
	if err := repo.Create(ctx, account); err != nil {
		t.Fatalf("create: %v", err)
	}

	in := usecase.UpdateStatusInput{
		TenantID: account.TenantID, AccountID: account.ID,
		Status: domain.AccountStatusActive, CredentialRef: "cred-ref-same",
	}
	if _, err := repo.UpdateStatus(ctx, in); err != nil {
		t.Fatalf("first update status: %v", err)
	}
	// Second call changes nothing — status and credential_ref are already
	// exactly these values.
	updated, err := repo.UpdateStatus(ctx, in)
	if err != nil {
		t.Fatalf("no-op retry update status: %v", err)
	}
	if updated.ID != account.ID || updated.Status != domain.AccountStatusActive || updated.CredentialRef != "cred-ref-same" {
		t.Errorf("expected the no-op retry to still return the existing account, got %+v", updated)
	}
}

func TestRepository_Update_RoundTripsLabelModelHintBaseURL(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	account := newTestAccount(t, "11111111-1111-1111-1111-111111111118", "11111111-1111-1111-1111-111111111111",
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref-1", domain.ScopeServer, "", "", "dev-1", "", 0, false, now)
	if err := repo.Create(ctx, account); err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := repo.Update(ctx, usecase.UpdateFields{
		TenantID: account.TenantID, AccountID: account.ID,
		Label: "my label", ModelHint: "claude-3-5-sonnet", BaseURL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Label != "my label" || updated.ModelHint != "claude-3-5-sonnet" || updated.BaseURL != "https://example.com" {
		t.Errorf("expected label/model_hint/base_url to round-trip, got %+v", updated)
	}

	got, err := repo.Get(ctx, account.TenantID, account.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Label != "my label" || got.ModelHint != "claude-3-5-sonnet" || got.BaseURL != "https://example.com" {
		t.Errorf("expected label/model_hint/base_url to persist, got %+v", got)
	}

	// A second, identical Update call is a realistic idempotent-retry
	// shape — must not report ErrAccountNotFound (matched-vs-changed
	// RowsAffected() pitfall, see Update's doc comment).
	if _, err := repo.Update(ctx, usecase.UpdateFields{
		TenantID: account.TenantID, AccountID: account.ID,
		Label: "my label", ModelHint: "claude-3-5-sonnet", BaseURL: "https://example.com",
	}); err != nil {
		t.Fatalf("no-op retry update: %v", err)
	}
}

// TestRepository_Create_DefaultDemotion exercises the demote-before-insert
// path: creating a second is_default account for the same
// tenant/dev_server/provider demotes the first rather than violating the
// generated-column unique index (migrations/mysql/0003's
// default_slot_key / uq_accounts_one_default_per_dev_server_provider).
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
// Create's demote-before-insert step must still be rejected — this is the
// generated-column unique index (migrations/mysql/0003) doing the same job
// the Postgres partial unique index does, proven directly against MySQL
// rather than assumed equivalent.
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

	_, err := repo.db.ExecContext(ctx, `
		INSERT INTO accounts (
			id, tenant_id, provider_type, status, credential_ref, scope, user_id, project_id,
			dev_server_id, label, model_hint, base_url, quota_limit_day, models, is_default,
			created_by, rotation_grace_until, created_at, updated_at
		) VALUES (?,?,'anthropic','active','cred-ref-2','server',NULL,NULL,'dev-1','','','',0,'[]',true,NULL,NULL,?,?)
	`, "11111111-1111-1111-1111-11111111111e", tenantID, now, now)
	if err == nil {
		t.Fatal("expected the generated-column unique index to reject a raw double-default insert")
	}
}

// TestRepository_Outbox_EnqueueFetchMarkPublished exercises Create's
// same-transaction outbox write, FetchUnpublished, and MarkPublished — the
// transactional-outbox contract.
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
// row too. MySQL triggers use a different syntax from Postgres's plpgsql
// function+trigger pair — SIGNAL raises an error directly inside a
// BEFORE INSERT trigger body.
func TestRepository_Create_RollsBackAccountInsertIfOutboxInsertFails(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	tenantID := "11111111-1111-1111-1111-111111111111"

	_, err := repo.db.ExecContext(ctx, `
		CREATE TRIGGER fail_outbox_trigger BEFORE INSERT ON outbox
		FOR EACH ROW
		BEGIN
			SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'injected outbox failure for test';
		END
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
// requirement, proven directly against MySQL's SELECT...FOR UPDATE SKIP
// LOCKED (supported since MySQL 8.0.1) rather than assumed equivalent to
// Postgres's: two goroutines calling ClaimDue against the same due rows
// concurrently must never claim the same row. Uses an explicit barrier so
// the two transactions are provably open at the same time.
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
// the same day must total 300, not 100 — proves the ON DUPLICATE KEY
// UPDATE translation of Postgres's ON CONFLICT DO UPDATE is additive, not
// a last-write-wins overwrite.
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

// TestRepository_GetToday_DoesNotLeakAcrossTenants is TASK-BE-DB-003's
// pattern applied to usage: 2 tenants record usage for accounts at the
// same account id shape; GetToday scoped to tenant A's (tenant_id,
// account_id) must never return tenant B's rollup. usage's Postgres
// original also has an RLS policy (migrations/postgres/0001_init.up.sql);
// migrations/mysql omits it, same rationale as accounts.
func TestRepository_GetToday_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	tenantA := "11111111-1111-1111-1111-111111111111"
	tenantB := "33333333-3333-3333-3333-333333333333"
	acctA := newTestAccount(t, "44444444-4444-4444-4444-444444444444", tenantA,
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref-a", domain.ScopeServer, "", "", "dev-1", "", 0, false, now)
	acctB := newTestAccount(t, "55555555-5555-5555-5555-555555555555", tenantB,
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref-b", domain.ScopeServer, "", "", "dev-1", "", 0, false, now)
	if err := repo.Create(ctx, acctA); err != nil {
		t.Fatalf("create account A: %v", err)
	}
	if err := repo.Create(ctx, acctB); err != nil {
		t.Fatalf("create account B: %v", err)
	}

	day := domain.DayKey(now)
	if _, err := repo.IncrementUsage(ctx, tenantA, acctA.ID, day, 500, 5, 1.23); err != nil {
		t.Fatalf("increment usage A: %v", err)
	}
	if _, err := repo.IncrementUsage(ctx, tenantB, acctB.ID, day, 999, 9, 9.99); err != nil {
		t.Fatalf("increment usage B: %v", err)
	}

	stateA, err := repo.GetToday(ctx, tenantA, acctA.ID, day)
	if err != nil {
		t.Fatalf("get today A: %v", err)
	}
	if stateA.TokensUsed != 500 || stateA.RequestCount != 5 {
		t.Errorf("expected tenant A's own usage (500 tokens, 5 requests), got %+v", stateA)
	}

	// Tenant A's own account id never collides with tenant B's — this
	// confirms GetToday's WHERE tenant_id = ? clause is load-bearing, not
	// redundant with account_id: query tenant B's tenant_id against
	// account A's id and expect zero usage (no row), not account A's data.
	crossState, err := repo.GetToday(ctx, tenantB, acctA.ID, day)
	if err != nil {
		t.Fatalf("get today (cross-tenant lookup): %v", err)
	}
	if crossState.TokensUsed != 0 || crossState.RequestCount != 0 {
		t.Errorf("expected zero usage when tenant_id doesn't match the account's own tenant, got %+v", crossState)
	}
}
