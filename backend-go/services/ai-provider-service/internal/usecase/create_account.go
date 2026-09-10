package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

// CreateAccountInput mirrors the gRPC request. DevServerID is required —
// registering an account with nowhere to test/push a credential against no
// longer makes sense once the test-before-save gate below exists.
type CreateAccountInput struct {
	TenantID      string
	ProviderType  domain.ProviderType
	Scope         domain.AccountScope
	UserID        string
	ProjectID     string
	DevServerID   string // required
	Label         string
	ModelHint     string
	BaseURL       string
	QuotaLimitDay int
	Models        []string
	IsDefault     bool
	EncryptedBlob []byte
}

// CreateAccount creates a new ProviderAccount. It never receives or handles
// plaintext: the caller-supplied EncryptedBlob (if any) is forwarded
// unopened to CredentialBrokerClient, and only the returned opaque
// CredentialRef is ever stored (ai-provider-service.md §3, §9). Before
// persisting, it live-tests the just-written credential via infra-fleet's
// Relay (verifyConnection, TASK-AIP-01-05) and rolls the credential back on
// failure rather than saving an account nobody can use.
type CreateAccount struct {
	repo   ProviderAccountRepository
	broker CredentialBrokerClient
	infra  InfraFleetClient
	newID  func() string
	now    func() time.Time
}

func NewCreateAccount(repo ProviderAccountRepository, broker CredentialBrokerClient, infra InfraFleetClient, newID func() string, now func() time.Time) *CreateAccount {
	if now == nil {
		now = time.Now
	}
	return &CreateAccount{repo: repo, broker: broker, infra: infra, newID: newID, now: now}
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
	if in.DevServerID == "" {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInvalidArgument, "AIPROVIDER_NO_DEV_SERVER", "dev_server_id is required to register a provider account", nil)
	}

	scope := in.Scope
	if scope == "" {
		scope = domain.ScopeServer
	}

	// Label uniqueness per (dev_server, provider) — app-layer, not a unique
	// index: label isn't guaranteed non-empty for every legacy row, so a
	// straight UNIQUE index would reject two intentionally-unlabeled accounts.
	if in.Label != "" {
		existing, err := uc.repo.List(ctx, ListAccountsFilter{TenantID: tenantID, DevServerID: in.DevServerID})
		if err != nil {
			return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_LABEL_CHECK_FAILED", "failed to check label uniqueness", err)
		}
		for _, acc := range existing {
			if acc.ProviderType == in.ProviderType && acc.Label == in.Label {
				return domain.ProviderAccount{}, apperrors.New(apperrors.KindAlreadyExists, "AIPROVIDER_LABEL_TAKEN", "an account with this name already exists for this provider on this dev server", nil)
			}
		}
	}

	// ownerID: prefer the specific user/project this account is scoped to;
	// server-scoped accounts (the common case today — see this method's
	// doc comment) have neither, so fall back to this service's own name.
	// credential-broker-service's owner_id is documented as "user id or
	// service name" precisely to accommodate this fallback.
	ownerID := in.UserID
	if ownerID == "" {
		ownerID = in.ProjectID
	}
	if ownerID == "" {
		ownerID = "ai-provider-service"
	}

	ref, err := uc.broker.WriteCredential(ctx, WriteCredentialInput{TenantID: tenantID, OwnerID: ownerID, EncryptedBlob: in.EncryptedBlob})
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_CREDENTIAL_WRITE_FAILED", "failed to write credential via credential-broker-service", err)
	}

	// Test-before-save gate — a failed test means the credential is provably
	// bad; roll back the just-written broker credential rather than
	// persisting an account nobody can use. Still "pending" either way — a
	// passed live test is necessary but not sufficient for "active" (§9's
	// push-confirmation invariant is untouched, see SOL-AIP-01's rationale).
	result, testErr := verifyConnection(ctx, uc.infra, in.DevServerID, ref.ID, in.ProviderType)
	if testErr != nil || !result.Success {
		if revokeErr := uc.broker.RevokeCredential(ctx, ref.ID); revokeErr != nil {
			return domain.ProviderAccount{}, apperrors.New(apperrors.KindInvalidArgument, "AIPROVIDER_TEST_CONNECTION_FAILED",
				fmt.Sprintf("connection test failed and credential cleanup also failed: %v / %v", testErr, revokeErr), testErr)
		}
		msg := result.Message
		if testErr != nil {
			msg = testErr.Error()
		}
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInvalidArgument, "AIPROVIDER_TEST_CONNECTION_FAILED", "connection test failed: "+msg, testErr)
	}

	// pending until ciphertext push to the target dev server is confirmed —
	// never "active" at creation time (ai-provider-service.md §9, §10).
	now := uc.now()
	account, err := domain.NewProviderAccount(
		uc.newID(), tenantID, in.ProviderType, domain.AccountStatusPending, ref.ID,
		scope, in.UserID, in.ProjectID, in.DevServerID,
		in.Label, in.ModelHint, in.BaseURL, in.QuotaLimitDay, in.Models, in.IsDefault,
		nil /* LastHealthCheckAt */, ownerID /* CreatedBy */, nil, now, now,
	)
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInvalidArgument, "AIPROVIDER_INVALID_ACCOUNT", err.Error(), err)
	}

	if err := uc.repo.Create(ctx, account); err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_CREATE_FAILED", "failed to persist provider account", err)
	}

	return account, nil
}
