# TASK-AIP-01-05: Extract shared `verifyConnection` + add `RevokeCredential` port

**From Solution:** SOL-AIP-01
**Priority:** P1
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/internal/usecase/verify_connection.go` (new)
**Depends on:** TASK-AIP-SHARED-01
**Status:** `[ ]` TODO

---

## Context

`TestConnection.Execute`'s relay-call logic
(`test_connection.go:40-65`) needs to be reusable by `CreateAccount`'s new
test-before-save gate (`TASK-AIP-01-06`) and by SOL-AIP-03's health-check
job (`TASK-AIP-03-04`), so it must become one shared function instead of
being copy-pasted. The rollback path in `TASK-AIP-01-06` also needs a
`RevokeCredential` port method that doesn't exist yet on
`CredentialBrokerClient`. This is inert until `TASK-AIP-SHARED-01`'s agent
handler exists — land this task anyway (same posture the existing
`TestConnection` usecase already ships in today, per its own doc comment).

## Changes to make

Create `backend-go/services/ai-provider-service/internal/usecase/verify_connection.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

// verifyConnection is shared by CreateAccount (test-before-save gate,
// TASK-AIP-01-06) and TestConnection (on-demand check) — one relay call, one
// place that knows the agent method name and result-parsing shape. See
// TASK-AIP-SHARED-01 for the agent-side handler this targets.
func verifyConnection(ctx context.Context, infra InfraFleetClient, devServerID, credentialRef string, providerType domain.ProviderType) (ConnectionTestResult, error) {
	result, err := infra.Relay(ctx, devServerID, "ai.testProviderConnection", map[string]any{
		"credentialRef": credentialRef,
		"providerType":  string(providerType),
	})
	if err != nil {
		return ConnectionTestResult{}, err
	}
	return parseConnectionTestResult(result), nil
}
```

Shrink `test_connection.go`'s `Execute` to call it instead of duplicating
the relay call, and delete the now-redundant `parseConnectionTestResult`
call site duplication (keep `parseConnectionTestResult` itself — it's
still used by `verifyConnection` above):

```go
func (uc *TestConnection) Execute(ctx context.Context, in TestConnectionInput) (ConnectionTestResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ConnectionTestResult{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}
	account, err := uc.repo.Get(ctx, tenantID, in.AccountID)
	if err != nil {
		return ConnectionTestResult{}, err
	}
	if account.DevServerID == "" {
		return ConnectionTestResult{}, apperrors.New(apperrors.KindFailedPrecondition, "AIPROVIDER_NO_DEV_SERVER", "account has no dev server bound yet — push a credential first", nil)
	}
	result, err := verifyConnection(ctx, uc.infra, account.DevServerID, account.CredentialRef, account.ProviderType)
	if err != nil {
		return ConnectionTestResult{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_TEST_CONNECTION_FAILED", "failed to relay connection test to dev server agent", err)
	}
	return result, nil
}
```

In `backend-go/services/ai-provider-service/internal/usecase/ports.go`, add
to `CredentialBrokerClient`:

```go
type CredentialBrokerClient interface {
	WriteCredential(ctx context.Context, in WriteCredentialInput) (CredentialRef, error)
	RotateCredential(ctx context.Context, credentialRef string) (CredentialRef, error)
	ResolveCredential(ctx context.Context, credentialRef string) (CredentialRef, error)
	// RevokeCredential — NEW, needed for CreateAccount's test-before-save
	// rollback path (TASK-AIP-01-06). Mirrors credential-broker-service.md
	// §3's RevokeCredential RPC (RevokeCredentialRequest -> Empty).
	RevokeCredential(ctx context.Context, credentialRef string) error
}
```

Add the stub implementation in
`backend-go/services/ai-provider-service/internal/adapter/grpcclient/`
(same file the other `CredentialBrokerClient` stub methods live in),
following the existing stub methods' shape — mark the ref revoked without
ever touching a secret value.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/ai-provider-service/...
go test ./services/ai-provider-service/internal/usecase/... -run TestTestConnection
```

Expected: existing `test_connection_test.go` still passes unmodified
(behavior is unchanged, only the implementation moved); any fake
`CredentialBrokerClient` test double used across the package now needs a
`RevokeCredential` method to satisfy the interface — update
`worktree_fakes_test.go`-style fakes in this package accordingly.
