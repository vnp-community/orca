package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

// CreateAccountInput mirrors the gRPC request plus the scope/credential
// fields the current CreateAccountRequest proto message doesn't carry yet
// (it only has tenant_id + type — see this service's README "Known gaps").
// Scope/UserID/ProjectID default to server-scope, no-ref when left zero, and
// EncryptedBlob may be nil (WriteCredential isn't wired into the proto
// surface yet either), in which case the broker stub is still called with
// an empty blob so the credential_ref/pending-status plumbing is exercised
// end-to-end even before the real write-credential path exists.
type CreateAccountInput struct {
	TenantID      string
	ProviderType  domain.ProviderType
	Scope         domain.AccountScope
	UserID        string
	ProjectID     string
	EncryptedBlob []byte
}

// CreateAccount creates a new ProviderAccount. It never receives or handles
// plaintext: the caller-supplied EncryptedBlob (if any) is forwarded
// unopened to CredentialBrokerClient, and only the returned opaque
// CredentialRef is ever stored (ai-provider-service.md §3, §9).
type CreateAccount struct {
	repo   ProviderAccountRepository
	broker CredentialBrokerClient
	newID  func() string
	now    func() time.Time
}

func NewCreateAccount(repo ProviderAccountRepository, broker CredentialBrokerClient, newID func() string, now func() time.Time) *CreateAccount {
	if now == nil {
		now = time.Now
	}
	return &CreateAccount{repo: repo, broker: broker, newID: newID, now: now}
}

func (uc *CreateAccount) Execute(ctx context.Context, in CreateAccountInput) (domain.ProviderAccount, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}
	// in.TenantID (from the request body) must agree with the authenticated
	// tenant from context if both are set — never trust the body over
	// context, per architecture/05-data-architecture.md's tenant-isolation rule.
	if in.TenantID != "" && in.TenantID != tenantID {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindPermissionDenied, "AIPROVIDER_TENANT_MISMATCH", "request tenant_id does not match authenticated tenant", nil)
	}

	scope := in.Scope
	if scope == "" {
		scope = domain.ScopeServer
	}

	ref, err := uc.broker.WriteCredential(ctx, WriteCredentialInput{TenantID: tenantID, EncryptedBlob: in.EncryptedBlob})
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_CREDENTIAL_WRITE_FAILED", "failed to write credential via credential-broker-service", err)
	}

	// pending until ciphertext push to the target dev server is confirmed —
	// never "active" at creation time (ai-provider-service.md §9, §10).
	now := uc.now()
	account, err := domain.NewProviderAccount(
		uc.newID(), tenantID, in.ProviderType, domain.AccountStatusPending, ref.ID,
		scope, in.UserID, in.ProjectID, nil, now, now,
	)
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInvalidArgument, "AIPROVIDER_INVALID_ACCOUNT", err.Error(), err)
	}

	if err := uc.repo.Create(ctx, account); err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_CREATE_FAILED", "failed to persist provider account", err)
	}

	return account, nil
}
