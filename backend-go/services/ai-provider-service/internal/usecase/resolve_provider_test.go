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
	now := time.Now()
	acc, err := domain.NewProviderAccount(id, tenantID, domain.ProviderTypeAnthropic, status, "cred-"+id, scope, userID, projectID, "", nil, now, now)
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

// TestResolveProvider_ExcludeAccountID_SkipsThatTierFallsThroughToNext
// exercises TASK-AG-04-02: excluding the only resolvable account in a tier
// must skip that tier and fall through to the next, without reordering the
// cascade — SwitchAgentAccount's own regression guard against re-resolving
// straight back to the account it's switching away from.
func TestResolveProvider_ExcludeAccountID_SkipsThatTierFallsThroughToNext(t *testing.T) {
	repo := newFakeAccountRepository()
	userAccount := mustAccount(t, "acc-user", "tenant-1", domain.ScopeUser, "user-1", "", domain.AccountStatusActive)
	projectAccount := mustAccount(t, "acc-project", "tenant-1", domain.ScopeProject, "", "project-1", domain.AccountStatusActive)
	_ = repo.Create(context.Background(), userAccount)
	_ = repo.Create(context.Background(), projectAccount)

	uc := NewResolveProvider(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Resolve(ctx, ResolveProviderInput{UserID: "user-1", ProjectID: "project-1", ExcludeAccountID: "acc-user"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "acc-project" {
		t.Fatalf("expected the excluded user-scope account to be skipped in favor of project scope, got %q", got.ID)
	}
}

// TestResolveProvider_ExcludeAccountID_AllTiersExcluded_ReturnsNoProviderAvailable
// proves excluding every candidate account correctly falls all the way
// through the cascade to ErrNoProviderAvailable, not a false match.
func TestResolveProvider_ExcludeAccountID_AllTiersExcluded_ReturnsNoProviderAvailable(t *testing.T) {
	repo := newFakeAccountRepository()
	serverAccount := mustAccount(t, "acc-server", "tenant-1", domain.ScopeServer, "", "", domain.AccountStatusActive)
	_ = repo.Create(context.Background(), serverAccount)

	uc := NewResolveProvider(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Resolve(ctx, ResolveProviderInput{UserID: "user-1", ExcludeAccountID: "acc-server"})
	var notAvailable *domain.ErrNoProviderAvailable
	if !errors.As(err, &notAvailable) {
		t.Fatalf("expected *domain.ErrNoProviderAvailable when the only candidate is excluded, got %v", err)
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
