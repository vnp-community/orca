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
	acc, ok := f.accounts[id]
	if !ok || acc.TenantID != tenantID {
		return domain.ProviderAccount{}, domain.ErrAccountNotFound
	}
	return acc, nil
}

func (f *fakeAccountRepository) List(ctx context.Context, filter ListAccountsFilter) ([]domain.ProviderAccount, error) {
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
}

func (f *fakeCredentialBroker) WriteCredential(ctx context.Context, in WriteCredentialInput) (CredentialRef, error) {
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
