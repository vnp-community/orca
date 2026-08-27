package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// vapidKeyName mirrors notification-service's pre-Epic-B
// internal/adapter/vaultsigner/signer.go convention exactly
// ("vapid-signing-" + tenantID) — moving the call site into this service
// must not also silently rotate every tenant's existing VAPID key by
// changing the name it's addressed under.
func vapidKeyName(tenantID string) string {
	return "vapid-signing-" + tenantID
}

// SignVapidPayloadInput mirrors SignVapidPayloadRequest 1:1.
type SignVapidPayloadInput struct {
	TenantID string
	Payload  []byte
}

// SignVapidPayload signs a Web Push VAPID JWT payload with a per-tenant
// Vault Transit key — see credentialbroker.proto's doc comment on this RPC
// for why it's modeled separately from WriteCredential/ResolveCredential.
// No CredentialMetadataRepository/AuditRepository involved: there is no
// credential_id to audit against (same schema-driven reasoning
// ResolveCredential's not-found path documents), and this key is
// service-provisioned rather than a stored, lifecycle-managed credential.
type SignVapidPayload struct {
	store SecretStore
}

func NewSignVapidPayload(store SecretStore) *SignVapidPayload {
	return &SignVapidPayload{store: store}
}

func (uc *SignVapidPayload) Execute(ctx context.Context, in SignVapidPayloadInput) (string, error) {
	if in.TenantID == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "CREDENTIAL_MISSING_SCOPE", "tenant_id is required", nil)
	}
	if len(in.Payload) == 0 {
		return "", apperrors.New(apperrors.KindInvalidArgument, "CREDENTIAL_EMPTY_PAYLOAD", "payload is required", nil)
	}
	signature, err := uc.store.TransitEncrypt(ctx, vapidKeyName(in.TenantID), in.Payload)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "CREDENTIAL_VAULT_SIGN_FAILED", "failed to sign vapid payload via vault transit", err)
	}
	return signature, nil
}
