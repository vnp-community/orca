package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

// WriteCredentialForAccountInput is aiProvider.writeCredential's usecase
// input — named distinctly from ports.go's existing WriteCredentialInput
// (that one is CredentialBrokerClient's port input; this one is this
// usecase's Execute input) to avoid confusion between the two.
type WriteCredentialForAccountInput struct {
	AccountID     string
	EncryptedBlob []byte
	IV            []byte
}

// WriteCredential writes a new credential onto an EXISTING account —
// distinct from CreateAccount, which writes one at creation time. Reuses
// CreateAccount's exact owner_id derivation and broker-call pattern
// (create_account.go), just against an existing account's Scope/UserID/
// ProjectID instead of create-time input.
type WriteCredential struct {
	repo   ProviderAccountRepository
	broker CredentialBrokerClient
}

func NewWriteCredential(repo ProviderAccountRepository, broker CredentialBrokerClient) *WriteCredential {
	return &WriteCredential{repo: repo, broker: broker}
}

func (uc *WriteCredential) Execute(ctx context.Context, in WriteCredentialForAccountInput) (domain.ProviderAccount, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}
	if in.AccountID == "" {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInvalidArgument, "AIPROVIDER_NO_ACCOUNT_ID", "account_id is required", nil)
	}

	account, err := uc.repo.Get(ctx, tenantID, in.AccountID)
	if err != nil {
		return domain.ProviderAccount{}, err
	}

	// Same owner_id derivation CreateAccount.Execute uses (create_account.go)
	// — identical fallback order, just against an existing account's
	// Scope/UserID/ProjectID instead of the create-time input.
	ownerID := account.UserID
	if ownerID == "" {
		ownerID = account.ProjectID
	}
	if ownerID == "" {
		ownerID = "ai-provider-service"
	}

	// IV is forwarded as part of the opaque encrypted envelope this service
	// never opens — concatenated with EncryptedBlob rather than added as a
	// second CredentialBrokerClient.WriteCredential field, since that port's
	// EncryptedBlob is already documented as "opaque ciphertext bytes...
	// nothing in this package or its callers inspects or decrypts it"
	// (ports.go) and credential-broker-service's WriteCredentialRequest has
	// a single encrypted_envelope field, not a separate IV field.
	envelope := append(append([]byte{}, in.IV...), in.EncryptedBlob...)

	ref, err := uc.broker.WriteCredential(ctx, WriteCredentialInput{TenantID: tenantID, OwnerID: ownerID, EncryptedBlob: envelope})
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_CREDENTIAL_WRITE_FAILED", "failed to write credential via credential-broker-service", err)
	}

	// UpdateStatusInput.CredentialRef already exists for exactly this —
	// ports.go's doc comment: "a rotation never leaves the row in a state
	// where Status says 'rotating' but CredentialRef still points at the
	// old secret" — the same invariant applies here, on first/updated write.
	// pending, not active, until push-confirmed — mirrors CreateAccount's
	// own "never active at creation time" rule (§9, §10).
	return uc.repo.UpdateStatus(ctx, UpdateStatusInput{
		TenantID: tenantID, AccountID: in.AccountID,
		Status: domain.AccountStatusPending, CredentialRef: ref.ID,
	})
}
