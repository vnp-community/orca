package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

// fakeAccountRepository is an in-memory ProviderAccountRepository — the
// "test against fakes, not a real database" pattern from
// specs/backend-go/standards/testing-strategy.md's unit-test section.
type fakeAccountRepository struct {
	accounts  map[string]domain.ProviderAccount
	createErr error
	listErr   error

	// lastListFilter records the last List call's filter — asserted by
	// list_accounts_test.go (TASK-030).
	lastListFilter ListAccountsFilter

	// getReturns, when non-nil, short-circuits Get to return this account
	// instead of looking it up in the accounts map — write_credential_test.go/
	// test_connection_test.go (TASK-030) use this to seed a Get result
	// without needing a full Create round-trip first.
	getReturns *domain.ProviderAccount

	// updateErr/lastUpdateInput back update_account_test.go (TASK-030).
	updateErr       error
	lastUpdateInput UpdateFields

	// deleteErr/lastDeleteTenantID/lastDeleteAccountID back
	// delete_account_test.go (TASK-030).
	deleteErr           error
	lastDeleteTenantID  string
	lastDeleteAccountID string

	// markQuotaWarningSentErr/lastQuotaWarningAccountID/lastQuotaWarningDay
	// back record_token_usage_test.go.
	markQuotaWarningSentErr   error
	lastQuotaWarningAccountID string
	lastQuotaWarningDay       time.Time

	// lastUpdateStatusInput records UpdateStatus's last input — asserted by
	// record_token_usage_test.go / reconcile_provider_health_test.go.
	lastUpdateStatusInput UpdateStatusInput
	updateStatusErr       error
}

func newFakeAccountRepository() *fakeAccountRepository {
	return &fakeAccountRepository{accounts: make(map[string]domain.ProviderAccount)}
}

func (f *fakeAccountRepository) Create(ctx context.Context, account domain.ProviderAccount) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.accounts[account.ID] = account
	return nil
}

func (f *fakeAccountRepository) Get(ctx context.Context, tenantID, id string) (domain.ProviderAccount, error) {
	if f.getReturns != nil {
		return *f.getReturns, nil
	}
	acc, ok := f.accounts[id]
	if !ok || acc.TenantID != tenantID {
		return domain.ProviderAccount{}, domain.ErrAccountNotFound
	}
	return acc, nil
}

func (f *fakeAccountRepository) List(ctx context.Context, filter ListAccountsFilter) ([]domain.ProviderAccount, error) {
	f.lastListFilter = filter
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.ProviderAccount
	for _, acc := range f.accounts {
		if acc.TenantID != filter.TenantID {
			continue
		}
		if filter.Scope != "" && acc.Scope != filter.Scope {
			continue
		}
		if filter.Scope == domain.ScopeUser && acc.UserID != filter.ScopeRefID {
			continue
		}
		if filter.Scope == domain.ScopeProject && acc.ProjectID != filter.ScopeRefID {
			continue
		}
		if filter.DevServerID != "" && acc.DevServerID != filter.DevServerID {
			continue
		}
		if filter.ProviderType != "" && acc.ProviderType != filter.ProviderType {
			continue
		}
		out = append(out, acc)
	}
	return out, nil
}

func (f *fakeAccountRepository) UpdateStatus(ctx context.Context, in UpdateStatusInput) (domain.ProviderAccount, error) {
	f.lastUpdateStatusInput = in
	if f.updateStatusErr != nil {
		return domain.ProviderAccount{}, f.updateStatusErr
	}
	acc, ok := f.accounts[in.AccountID]
	if !ok || acc.TenantID != in.TenantID {
		return domain.ProviderAccount{}, domain.ErrAccountNotFound
	}
	acc.Status = in.Status
	if in.HealthDetail != nil {
		acc.HealthDetail = in.HealthDetail
	}
	if in.CredentialRef != "" {
		acc.CredentialRef = in.CredentialRef
	}
	if in.RotationGraceUntil != nil {
		acc.RotationGraceUntil = in.RotationGraceUntil
	}
	f.accounts[in.AccountID] = acc
	return acc, nil
}

// MarkQuotaWarningSent implements usecase.ProviderAccountRepository —
// records the call and, when the account is known, sets
// QuotaWarningSentDate on it (record_token_usage_test.go's idempotency
// assertions read this back via Get).
func (f *fakeAccountRepository) MarkQuotaWarningSent(ctx context.Context, tenantID, accountID string, day time.Time) error {
	f.lastQuotaWarningAccountID = accountID
	f.lastQuotaWarningDay = day
	if f.markQuotaWarningSentErr != nil {
		return f.markQuotaWarningSentErr
	}
	if acc, ok := f.accounts[accountID]; ok && acc.TenantID == tenantID {
		d := day
		acc.QuotaWarningSentDate = &d
		f.accounts[accountID] = acc
	}
	return nil
}

// Update implements usecase.ProviderAccountRepository.Update — records the
// input verbatim (lastUpdateInput) and, when the account is known, returns
// it unmodified (Label/ModelHint/BaseURL aren't domain.ProviderAccount
// fields — see ports.go's UpdateFields doc comment — so there is nothing on
// the returned struct for them to change).
func (f *fakeAccountRepository) Update(ctx context.Context, in UpdateFields) (domain.ProviderAccount, error) {
	f.lastUpdateInput = in
	if f.updateErr != nil {
		return domain.ProviderAccount{}, f.updateErr
	}
	acc, ok := f.accounts[in.AccountID]
	if !ok || acc.TenantID != in.TenantID {
		return domain.ProviderAccount{}, domain.ErrAccountNotFound
	}
	return acc, nil
}

// Delete implements usecase.ProviderAccountRepository.Delete.
func (f *fakeAccountRepository) Delete(ctx context.Context, tenantID, accountID string) error {
	f.lastDeleteTenantID = tenantID
	f.lastDeleteAccountID = accountID
	if f.deleteErr != nil {
		return f.deleteErr
	}
	acc, ok := f.accounts[accountID]
	if !ok || acc.TenantID != tenantID {
		return domain.ErrAccountNotFound
	}
	acc.Status = domain.AccountStatusRevoked
	f.accounts[accountID] = acc
	return nil
}

// fakeUsageRepository is an in-memory UsageRepository.
type fakeUsageRepository struct {
	states        map[string]domain.QuotaState // keyed by accountID
	incrementErr  error
	lastIncrement struct {
		tenantID, accountID      string
		day                      time.Time
		tokensUsed, requestCount int64
		costUSD                  float64
	}
}

func (f *fakeUsageRepository) GetToday(ctx context.Context, tenantID, accountID string, day time.Time) (domain.QuotaState, error) {
	if state, ok := f.states[accountID]; ok {
		return state, nil
	}
	return domain.QuotaState{AccountID: accountID, Date: day}, nil
}

// IncrementUsage implements usecase.UsageRepository — additive, in-memory.
func (f *fakeUsageRepository) IncrementUsage(ctx context.Context, tenantID, accountID string, day time.Time, tokensUsed, requestCount int64, costUSD float64) (domain.QuotaState, error) {
	f.lastIncrement.tenantID, f.lastIncrement.accountID = tenantID, accountID
	f.lastIncrement.day = day
	f.lastIncrement.tokensUsed, f.lastIncrement.requestCount, f.lastIncrement.costUSD = tokensUsed, requestCount, costUSD
	if f.incrementErr != nil {
		return domain.QuotaState{}, f.incrementErr
	}
	if f.states == nil {
		f.states = make(map[string]domain.QuotaState)
	}
	state := f.states[accountID]
	state.AccountID = accountID
	state.Date = day
	state.TokensUsed += tokensUsed
	state.RequestCount += requestCount
	state.CostUSD += costUSD
	f.states[accountID] = state
	return state, nil
}

// fakeOutboxEnqueuer is an in-memory OutboxEnqueuer — backs
// reconcile_provider_health_test.go / record_token_usage_test.go.
type fakeOutboxEnqueuer struct {
	enqueueErr error
	enqueued   []struct {
		subject  string
		tenantID string
		payload  map[string]any
	}
}

func (f *fakeOutboxEnqueuer) Enqueue(ctx context.Context, subject string, tenantID string, payload map[string]any) error {
	if f.enqueueErr != nil {
		return f.enqueueErr
	}
	f.enqueued = append(f.enqueued, struct {
		subject  string
		tenantID string
		payload  map[string]any
	}{subject, tenantID, payload})
	return nil
}

// fakeHealthCheckBatch/fakeHealthCheckClaimer back
// reconcile_provider_health_test.go.
type fakeHealthCheckBatch struct {
	accounts    []domain.ProviderAccount
	recordErr   error
	committed   bool
	rolledBack  bool
	recordCalls []domain.ProviderAccount
}

func (b *fakeHealthCheckBatch) Accounts() []domain.ProviderAccount { return b.accounts }

func (b *fakeHealthCheckBatch) RecordResult(ctx context.Context, accountID string, status domain.AccountStatus, healthDetail *string, latencyMs *int, checkedAt time.Time) error {
	if b.recordErr != nil {
		return b.recordErr
	}
	for i, acc := range b.accounts {
		if acc.ID == accountID {
			b.accounts[i].Status = status
			b.accounts[i].HealthDetail = healthDetail
			b.accounts[i].LatencyMs = latencyMs
			b.recordCalls = append(b.recordCalls, b.accounts[i])
		}
	}
	return nil
}

func (b *fakeHealthCheckBatch) Commit(ctx context.Context) error {
	b.committed = true
	return nil
}

func (b *fakeHealthCheckBatch) Rollback(ctx context.Context) error {
	if !b.committed {
		b.rolledBack = true
	}
	return nil
}

type fakeHealthCheckClaimer struct {
	batch    *fakeHealthCheckBatch
	claimErr error
}

func (f *fakeHealthCheckClaimer) ClaimDue(ctx context.Context, now time.Time, staleness time.Duration, limit int32) (ClaimedHealthCheckBatch, error) {
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	return f.batch, nil
}

// fakeCredentialBroker is an in-memory CredentialBrokerClient — never
// touches real secret material, exactly like the real
// adapter/grpcclient stub it stands in for.
type fakeCredentialBroker struct {
	writeErr  error
	rotateErr error
	nextRefID string

	// lastWriteOwnerID records WriteCredential's last OwnerID — asserted by
	// write_credential_test.go (TASK-030) to verify the owner_id derivation
	// mirrors CreateAccount's.
	lastWriteOwnerID string

	// revokeErr/lastRevokedRef back create_account_test.go's test-before-save
	// gate assertions (TASK-AIP-01-06).
	revokeErr       error
	lastRevokedRef  string
	revokeCallCount int
}

func (f *fakeCredentialBroker) WriteCredential(ctx context.Context, in WriteCredentialInput) (CredentialRef, error) {
	f.lastWriteOwnerID = in.OwnerID
	if f.writeErr != nil {
		return CredentialRef{}, f.writeErr
	}
	id := f.nextRefID
	if id == "" {
		id = "cred-ref-stub"
	}
	return CredentialRef{ID: id, Status: "pending_push"}, nil
}

func (f *fakeCredentialBroker) RotateCredential(ctx context.Context, credentialRef string) (CredentialRef, error) {
	if f.rotateErr != nil {
		return CredentialRef{}, f.rotateErr
	}
	return CredentialRef{ID: credentialRef + "-rotated", Status: "pending_push"}, nil
}

func (f *fakeCredentialBroker) ResolveCredential(ctx context.Context, credentialRef string) (CredentialRef, error) {
	return CredentialRef{ID: credentialRef, Status: "active"}, nil
}

func (f *fakeCredentialBroker) RevokeCredential(ctx context.Context, credentialRef string) error {
	f.lastRevokedRef = credentialRef
	f.revokeCallCount++
	if f.revokeErr != nil {
		return f.revokeErr
	}
	return nil
}

// fakeInfraFleetClient is an in-memory InfraFleetClient — backs
// test_connection_test.go (TASK-030).
type fakeInfraFleetClient struct {
	relayResult map[string]any
	relayErr    error

	lastDevServerID string
	lastMethod      string
	lastParams      map[string]any
}

func (f *fakeInfraFleetClient) Relay(ctx context.Context, devServerID, method string, params map[string]any) (map[string]any, error) {
	f.lastDevServerID = devServerID
	f.lastMethod = method
	f.lastParams = params
	if f.relayErr != nil {
		return nil, f.relayErr
	}
	return f.relayResult, nil
}

var errBoom = errors.New("boom")

func withIdentity(ctx context.Context, tenantID, userID string) context.Context {
	ctx = tenant.WithTenantID(ctx, tenantID)
	return tenant.WithUserID(ctx, userID)
}
