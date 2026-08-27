# TASK-AIP-01-06: Wire label uniqueness, test-before-save gate, and default-demotion into `CreateAccount`

**From Solution:** SOL-AIP-01
**Priority:** P1
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/internal/usecase/create_account.go`
**Depends on:** TASK-AIP-01-03, TASK-AIP-01-05
**Status:** `[ ]` TODO

---

## Context

`CreateAccount.Execute` (`create_account.go:47-98`) doesn't require
`DevServerID`, doesn't check label uniqueness, doesn't call the new
test-before-save gate, and doesn't pass any of `TASK-AIP-01-03`'s new
domain fields into `NewProviderAccount` (whose signature that task already
changed — this task is what fixes the resulting compile break).

## Changes to make

Replace `CreateAccountInput` and `Execute` in
`backend-go/services/ai-provider-service/internal/usecase/create_account.go`:

```go
type CreateAccountInput struct {
	TenantID      string
	ProviderType  domain.ProviderType
	Scope         domain.AccountScope
	UserID        string
	ProjectID     string
	DevServerID   string   // NEW — required now
	Label         string   // NEW
	ModelHint     string   // NEW
	BaseURL       string   // NEW
	QuotaLimitDay int      // NEW
	Models        []string // NEW
	IsDefault     bool     // NEW
	EncryptedBlob []byte
}

type CreateAccount struct {
	repo   ProviderAccountRepository
	broker CredentialBrokerClient
	infra  InfraFleetClient // NEW — test-before-save gate
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
```

Add `"fmt"` to the import block. Default-demotion (`IsDefault` clearing a
prior default) and the outbox insert both happen inside
`repo.Create`'s transaction, not here — see `TASK-AIP-01-07`.

Update `cmd/server/main.go`'s `usecase.NewCreateAccount(...)` call site to
pass `infraFleet` (already dialed for `testConnectionUC`) as the new
`infra` argument — see `TASK-AIP-01-07` for the full `main.go` diff.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/ai-provider-service/...
go test ./services/ai-provider-service/internal/usecase/... -run TestCreateAccount
```

Add to `create_account_test.go`:
- `TestCreateAccount_RequiresDevServerID`
- `TestCreateAccount_LabelUniquenessPerDevServerProvider` — same label,
  same dev server, same provider → `AlreadyExists`; different provider →
  succeeds.
- `TestCreateAccount_TestConnectionGate` — fake `InfraFleetClient` returns
  `{success: false}`; assert `Create` fails, fake broker's
  `RevokeCredential` was called with the ref just written, and
  `repo.Create` was never called.
- `TestCreateAccount_TestConnectionGate_RevokeFailureSurfaced` — both the
  test and the revoke fail; error message includes both.
