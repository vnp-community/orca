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
		out = append(out, acc)
	}
	return out, nil
}

func (f *fakeAccountRepository) UpdateStatus(ctx context.Context, in UpdateStatusInput) (domain.ProviderAccount, error) {
	acc, ok := f.accounts[in.AccountID]
	if !ok || acc.TenantID != in.TenantID {
		return domain.ProviderAccount{}, domain.ErrAccountNotFound
	}
	acc.Status = in.Status
	if in.CredentialRef != "" {
		acc.CredentialRef = in.CredentialRef
	}
	if in.RotationGraceUntil != nil {
		acc.RotationGraceUntil = in.RotationGraceUntil
	}
	f.accounts[in.AccountID] = acc
	return acc, nil
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
	states map[string]domain.QuotaState // keyed by accountID
}

func (f *fakeUsageRepository) GetToday(ctx context.Context, tenantID, accountID string, day time.Time) (domain.QuotaState, error) {
	if state, ok := f.states[accountID]; ok {
		return state, nil
	}
	return domain.QuotaState{AccountID: accountID, Date: day}, nil
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

var errBoom = errors.New("boom")

func withIdentity(ctx context.Context, tenantID, userID string) context.Context {
	ctx = tenant.WithTenantID(ctx, tenantID)
	return tenant.WithUserID(ctx, userID)
}
