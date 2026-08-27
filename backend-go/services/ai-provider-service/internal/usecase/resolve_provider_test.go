package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

func mustAccount(t *testing.T, id, tenantID string, scope domain.AccountScope, userID, projectID string, status domain.AccountStatus) domain.ProviderAccount {
	t.Helper()
	return mustAccountFull(t, id, tenantID, domain.ProviderTypeAnthropic, scope, userID, projectID, status, "", time.Now())
}

// mustAccountFull is mustAccount plus the provider type, dev server id, and
// createdAt — needed by the cross-provider/dev-server regression tests
// (TASK-AIP-02-05).
func mustAccountFull(t *testing.T, id, tenantID string, provider domain.ProviderType, scope domain.AccountScope, userID, projectID string, status domain.AccountStatus, devServerID string, createdAt time.Time) domain.ProviderAccount {
	t.Helper()
	acc, err := domain.NewProviderAccount(id, tenantID, provider, status, "cred-"+id, scope, userID, projectID, devServerID,
		"", "", "", 0, nil, false, nil, "", nil, createdAt, createdAt)
	if err != nil {
		t.Fatalf("building account %s: %v", id, err)
	}
	return acc
}

// TestResolveProvider_UserScopeWinsOverProjectScope is the load-bearing test
// flagged by ai-provider-service.md §4: prior TS documentation stated this
// cascade backwards. When both a user-scope and a project-scope account
// exist and are active, Resolve MUST return the user-scope one.
func TestResolveProvider_UserScopeWinsOverProjectScope(t *testing.T) {
	repo := newFakeAccountRepository()
	userAccount := mustAccount(t, "acc-user", "tenant-1", domain.ScopeUser, "user-1", "", domain.AccountStatusActive)
	projectAccount := mustAccount(t, "acc-project", "tenant-1", domain.ScopeProject, "", "project-1", domain.AccountStatusActive)
	_ = repo.Create(context.Background(), userAccount)
	_ = repo.Create(context.Background(), projectAccount)

	uc := NewResolveProvider(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Resolve(ctx, ResolveProviderInput{UserID: "user-1", ProjectID: "project-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "acc-user" {
		t.Fatalf("expected user-scope account to win the cascade, got %q (scope=%s)", got.ID, got.Scope)
	}
}

func TestResolveProvider_FallsBackToProjectScopeWhenNoUserAccount(t *testing.T) {
	repo := newFakeAccountRepository()
	projectAccount := mustAccount(t, "acc-project", "tenant-1", domain.ScopeProject, "", "project-1", domain.AccountStatusActive)
	_ = repo.Create(context.Background(), projectAccount)

	uc := NewResolveProvider(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Resolve(ctx, ResolveProviderInput{UserID: "user-1", ProjectID: "project-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "acc-project" {
		t.Fatalf("expected project-scope fallback, got %q", got.ID)
	}
}

func TestResolveProvider_FallsBackToServerScopeWhenNoUserOrProjectAccount(t *testing.T) {
	repo := newFakeAccountRepository()
	serverAccount := mustAccount(t, "acc-server", "tenant-1", domain.ScopeServer, "", "", domain.AccountStatusActive)
	_ = repo.Create(context.Background(), serverAccount)

	uc := NewResolveProvider(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Resolve(ctx, ResolveProviderInput{UserID: "user-1", ProjectID: "project-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "acc-server" {
		t.Fatalf("expected server-scope fallback, got %q", got.ID)
	}
}

func TestResolveProvider_SkipsUserAccountThatIsNotActive(t *testing.T) {
	repo := newFakeAccountRepository()
	pendingUserAccount := mustAccount(t, "acc-user-pending", "tenant-1", domain.ScopeUser, "user-1", "", domain.AccountStatusPending)
	projectAccount := mustAccount(t, "acc-project", "tenant-1", domain.ScopeProject, "", "project-1", domain.AccountStatusActive)
	_ = repo.Create(context.Background(), pendingUserAccount)
	_ = repo.Create(context.Background(), projectAccount)

	uc := NewResolveProvider(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Resolve(ctx, ResolveProviderInput{UserID: "user-1", ProjectID: "project-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// A "pending" account (ciphertext push unconfirmed) must never be
	// returned — see domain.ProviderAccount.Resolvable and §10's
	// cutover-ordering note. Must fall through to project scope.
	if got.ID != "acc-project" {
		t.Fatalf("expected non-active user account to be skipped in favor of project scope, got %q", got.ID)
	}
}

func TestResolveProvider_NoScopeMatchReturnsErrNoProviderAvailable(t *testing.T) {
	repo := newFakeAccountRepository()
	uc := NewResolveProvider(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Resolve(ctx, ResolveProviderInput{UserID: "user-1", ProjectID: "project-1"})
	var notAvailable *domain.ErrNoProviderAvailable
	if !errors.As(err, &notAvailable) {
		t.Fatalf("expected *domain.ErrNoProviderAvailable, got %v", err)
	}
	if notAvailable.Reason != domain.ReasonNoScopeMatch {
		t.Errorf("expected reason %q, got %q", domain.ReasonNoScopeMatch, notAvailable.Reason)
	}
}

func TestResolveProvider_QuotaOrInactiveReasonWhenCandidateExistsButNotActive(t *testing.T) {
	repo := newFakeAccountRepository()
	revoked := mustAccount(t, "acc-user-revoked", "tenant-1", domain.ScopeUser, "user-1", "", domain.AccountStatusRevoked)
	_ = repo.Create(context.Background(), revoked)

	uc := NewResolveProvider(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Resolve(ctx, ResolveProviderInput{UserID: "user-1"})
	var notAvailable *domain.ErrNoProviderAvailable
	if !errors.As(err, &notAvailable) {
		t.Fatalf("expected *domain.ErrNoProviderAvailable, got %v", err)
	}
	if notAvailable.Reason != domain.ReasonQuotaOrInactive {
		t.Errorf("expected reason %q, got %q", domain.ReasonQuotaOrInactive, notAvailable.Reason)
	}
}

func TestResolveProvider_RequiresTenantContext(t *testing.T) {
	uc := NewResolveProvider(newFakeAccountRepository())
	_, err := uc.Resolve(context.Background(), ResolveProviderInput{UserID: "user-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestResolveProvider_DoesNotLeakOtherTenantsAccounts(t *testing.T) {
	repo := newFakeAccountRepository()
	otherTenant := mustAccount(t, "acc-other-tenant", "tenant-2", domain.ScopeUser, "user-1", "", domain.AccountStatusActive)
	_ = repo.Create(context.Background(), otherTenant)

	uc := NewResolveProvider(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Resolve(ctx, ResolveProviderInput{UserID: "user-1"})
	var notAvailable *domain.ErrNoProviderAvailable
	if !errors.As(err, &notAvailable) {
		t.Fatalf("expected *domain.ErrNoProviderAvailable (cross-tenant account must not resolve), got %v", err)
	}
}

// ── TASK-AIP-02-05: cross-provider regression guards ───────────────────────
// The exact symptom BUG-AIP-02 reports: a tenant with both an Anthropic and
// an OpenAI account at the same scope could get either one back depending
// on created_at ordering. The OpenAI account is deliberately given an
// EARLIER created_at than the Anthropic one in every case below, to prove
// the fix is the provider filter, not incidental list ordering.

func TestResolveProvider_ModelHintFiltersProvider_UserScope(t *testing.T) {
	repo := newFakeAccountRepository()
	earlier := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	later := earlier.Add(time.Hour)
	openai := mustAccountFull(t, "acc-openai", "tenant-1", domain.ProviderTypeOpenAI, domain.ScopeUser, "user-1", "", domain.AccountStatusActive, "", earlier)
	anthropic := mustAccountFull(t, "acc-anthropic", "tenant-1", domain.ProviderTypeAnthropic, domain.ScopeUser, "user-1", "", domain.AccountStatusActive, "", later)
	_ = repo.Create(context.Background(), openai)
	_ = repo.Create(context.Background(), anthropic)

	uc := NewResolveProvider(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Resolve(ctx, ResolveProviderInput{UserID: "user-1", ModelHint: "claude-3-5-sonnet"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "acc-anthropic" {
		t.Fatalf("expected model_hint to filter to the Anthropic account regardless of created_at order, got %q (provider=%s)", got.ID, got.ProviderType)
	}
}

func TestResolveProvider_ModelHintFiltersProvider_ProjectScope(t *testing.T) {
	repo := newFakeAccountRepository()
	earlier := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	later := earlier.Add(time.Hour)
	openai := mustAccountFull(t, "acc-openai", "tenant-1", domain.ProviderTypeOpenAI, domain.ScopeProject, "", "project-1", domain.AccountStatusActive, "", earlier)
	anthropic := mustAccountFull(t, "acc-anthropic", "tenant-1", domain.ProviderTypeAnthropic, domain.ScopeProject, "", "project-1", domain.AccountStatusActive, "", later)
	_ = repo.Create(context.Background(), openai)
	_ = repo.Create(context.Background(), anthropic)

	uc := NewResolveProvider(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Resolve(ctx, ResolveProviderInput{ProjectID: "project-1", ModelHint: "claude-3-5-sonnet"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "acc-anthropic" {
		t.Fatalf("expected model_hint to filter to the Anthropic account regardless of created_at order, got %q (provider=%s)", got.ID, got.ProviderType)
	}
}

func TestResolveProvider_ModelHintFiltersProvider_ServerScope(t *testing.T) {
	repo := newFakeAccountRepository()
	earlier := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	later := earlier.Add(time.Hour)
	openai := mustAccountFull(t, "acc-openai", "tenant-1", domain.ProviderTypeOpenAI, domain.ScopeServer, "", "", domain.AccountStatusActive, "", earlier)
	anthropic := mustAccountFull(t, "acc-anthropic", "tenant-1", domain.ProviderTypeAnthropic, domain.ScopeServer, "", "", domain.AccountStatusActive, "", later)
	_ = repo.Create(context.Background(), openai)
	_ = repo.Create(context.Background(), anthropic)

	uc := NewResolveProvider(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Resolve(ctx, ResolveProviderInput{ModelHint: "claude-3-5-sonnet"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "acc-anthropic" {
		t.Fatalf("expected model_hint to filter to the Anthropic account regardless of created_at order, got %q (provider=%s)", got.ID, got.ProviderType)
	}
}

func TestResolveProvider_DevServerScoping(t *testing.T) {
	repo := newFakeAccountRepository()
	now := time.Now()
	devA := mustAccountFull(t, "acc-dev-a", "tenant-1", domain.ProviderTypeAnthropic, domain.ScopeServer, "", "", domain.AccountStatusActive, "dev-a", now)
	devB := mustAccountFull(t, "acc-dev-b", "tenant-1", domain.ProviderTypeAnthropic, domain.ScopeServer, "", "", domain.AccountStatusActive, "dev-b", now)
	_ = repo.Create(context.Background(), devA)
	_ = repo.Create(context.Background(), devB)

	uc := NewResolveProvider(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Resolve(ctx, ResolveProviderInput{DevServerID: "dev-a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "acc-dev-a" {
		t.Fatalf("expected dev_server_id to scope resolution to that server only, got %q", got.ID)
	}
}

// ── TASK-AIP-02-06: explicit accountId (Case 1) / scoped_ref (Case 2) ──────

func TestResolveProvider_ExplicitAccountID(t *testing.T) {
	repo := newFakeAccountRepository()
	active := mustAccount(t, "acc-active", "tenant-1", domain.ScopeServer, "", "", domain.AccountStatusActive)
	inactive := mustAccount(t, "acc-inactive", "tenant-1", domain.ScopeServer, "", "", domain.AccountStatusRevoked)
	_ = repo.Create(context.Background(), active)
	_ = repo.Create(context.Background(), inactive)

	uc := NewResolveProvider(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Resolve(ctx, ResolveProviderInput{AccountID: "acc-active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "acc-active" {
		t.Fatalf("expected explicit account id to bypass the cascade, got %q", got.ID)
	}

	_, err = uc.Resolve(ctx, ResolveProviderInput{AccountID: "acc-inactive"})
	var notAvailable *domain.ErrNoProviderAvailable
	if !errors.As(err, &notAvailable) {
		t.Fatalf("expected an inactive explicitly-requested account to return ErrNoProviderAvailable, got %v", err)
	}
}

func TestResolveScopedRef(t *testing.T) {
	repo := newFakeAccountRepository()
	serverAcc := mustAccountFull(t, "acc-server", "tenant-1", domain.ProviderTypeOpenAI, domain.ScopeServer, "", "", domain.AccountStatusActive, "", time.Now())
	projectAcc := mustAccountFull(t, "acc-project", "tenant-1", domain.ProviderTypeAnthropic, domain.ScopeProject, "", "project-1", domain.AccountStatusActive, "", time.Now())
	userAcc := mustAccountFull(t, "acc-user", "tenant-1", domain.ProviderTypeGoogle, domain.ScopeUser, "user-1", "", domain.AccountStatusActive, "", time.Now())
	_ = repo.Create(context.Background(), serverAcc)
	_ = repo.Create(context.Background(), projectAcc)
	_ = repo.Create(context.Background(), userAcc)

	uc := NewResolveProvider(repo)

	tests := []struct {
		name      string
		scopedRef string
		wantID    string
		wantErr   bool
	}{
		{"server ref", "server:openai", "acc-server", false},
		{"project ref", "project:project-1:anthropic", "acc-project", false},
		{"user ref", "user:google", "acc-user", false},
		{"malformed ref", "bogus-format", "", true},
		{"unknown provider", "server:not-a-provider", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := withIdentity(context.Background(), "tenant-1", "user-1")
			got, err := uc.Resolve(ctx, ResolveProviderInput{ScopedRef: tt.scopedRef})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for scoped_ref %q", tt.scopedRef)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for scoped_ref %q: %v", tt.scopedRef, err)
			}
			if got.ID != tt.wantID {
				t.Fatalf("scoped_ref %q: expected account %q, got %q", tt.scopedRef, tt.wantID, got.ID)
			}
		})
	}
}
