package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

// RotateKeyInput mirrors the gRPC RotateKeyRequest.
type RotateKeyInput struct {
	AccountID string
}

// RotateKey asks CredentialBrokerClient to rotate the secret behind an
// account's credential ref, then persists the new ref and "rotating"
// status. Never touches the secret itself — only the opaque ref that comes
// back (ai-provider-service.md §9's rotation-grace flow).
type RotateKey struct {
	repo   ProviderAccountRepository
	broker CredentialBrokerClient
}

func NewRotateKey(repo ProviderAccountRepository, broker CredentialBrokerClient) *RotateKey {
	return &RotateKey{repo: repo, broker: broker}
}

func (uc *RotateKey) Execute(ctx context.Context, in RotateKeyInput) (domain.ProviderAccount, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}

	account, err := uc.repo.Get(ctx, tenantID, in.AccountID)
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindNotFound, "AIPROVIDER_ACCOUNT_NOT_FOUND", "provider account not found", err)
	}

	newRef, err := uc.broker.RotateCredential(ctx, account.CredentialRef)
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_ROTATE_FAILED", "failed to rotate credential via credential-broker-service", err)
	}

	// A real implementation sets RotationGraceUntil to however long the
	// previous ciphertext version stays valid on the agent side while the
	// new one propagates (§9). No PushCiphertext port exists yet in this
	// scaffold — see this service's README "Known gaps" — so the grace
	// window is left unset (nil) rather than guessed at.
	updated, err := uc.repo.UpdateStatus(ctx, UpdateStatusInput{
		TenantID:      tenantID,
		AccountID:     in.AccountID,
		Status:        domain.AccountStatusRotating,
		CredentialRef: newRef.ID,
	})
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_UPDATE_STATUS_FAILED", "failed to persist rotation state", err)
	}

	return updated, nil
}
