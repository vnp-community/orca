# TASK-030: Tests for `aiProvider.*` usecases, `infra-fleet-service`'s new resolver, and `wscompat` channels

**From Solution:** SOL-005 (Test plan section)
**Priority:** P2
**Service:** `ai-provider-service`, `infra-fleet-service`, `api-gateway`
**File:** `services/ai-provider-service/internal/usecase/{list_accounts_test.go,update_account_test.go,delete_account_test.go,write_credential_test.go,test_connection_test.go}` (new), `services/infra-fleet-service/internal/usecase/resolve_connection_test.go` (extend), `services/api-gateway/internal/adapter/wscompat/channels_ai_provider_test.go` (new)
**Depends on:** TASK-025, TASK-026, TASK-027, TASK-028, TASK-029
**Status:** `[partial]` — `services/api-gateway/internal/adapter/wscompat/channels_ai_provider_test.go` added: one success + one error-propagation test per channel (6 channels, 13 tests total), plus a base64-decode-failure case for `writeCredential`, following `channels_accounts_test.go`'s fake-client + `outgoingTenantUser` conventions. `infra-fleet-service`'s `ResolveConnection` dev-server-id/worktree-id cases (`TestResolveConnection_ByDevServerID_MatchesByConnectionID`, `TestResolveConnection_ByWorktreeID_MatchesByConnectionID`) already existed pre-pass. NOT done in this pass: `ai-provider-service/internal/usecase/{update_account_test.go,delete_account_test.go,write_credential_test.go,test_connection_test.go}` — those usecases are implemented but have no dedicated tests yet; out of scope for this pass, which was narrowed to the `wscompat` channel layer only.

---

## Context

Covers SOL-005's full test plan. `ai-provider-service`'s existing
`fakes_test.go` already provides a fake `ProviderAccountRepository`/
`CredentialBrokerClient` for this package (see `create_account_test.go` for
usage) — reuse those fakes rather than redefining them. Add a new
`fakeInfraFleetClient` (usecase-level, distinct from `wscompat`'s
same-named test double — different package, different interface) scoped to
`Relay` for `test_connection_test.go`.

---

## Changes to make

### `ai-provider-service/internal/usecase/list_accounts_test.go` (new)

```go
package usecase_test

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/usecase"
)

func TestListAccounts_FiltersByDevServerID(t *testing.T) {
	repo := newFakeProviderAccountRepository() // existing fakes_test.go helper
	uc := usecase.NewListAccounts(repo)

	ctx := tenant.WithTenantID(context.Background(), "t1")
	if _, err := uc.Execute(ctx, usecase.ListAccountsInput{DevServerID: "ds-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastListFilter.DevServerID != "ds-1" {
		t.Errorf("filter.DevServerID = %q, want ds-1", repo.lastListFilter.DevServerID)
	}
	if repo.lastListFilter.TenantID != "t1" {
		t.Errorf("filter.TenantID = %q, want t1", repo.lastListFilter.TenantID)
	}
}

func TestListAccounts_NoTenant_Errors(t *testing.T) {
	repo := newFakeProviderAccountRepository()
	uc := usecase.NewListAccounts(repo)
	if _, err := uc.Execute(context.Background(), usecase.ListAccountsInput{}); err == nil {
		t.Fatal("expected an error with no tenant in context")
	}
}
```

(`newFakeProviderAccountRepository` and its `lastListFilter` field: extend
the existing fake repository in `fakes_test.go` to record the last `List`
call's filter, if it doesn't already — check that file first.)

### `update_account_test.go` (new)

Asserts `Update` is called with only `Label`/`ModelHint`/`BaseURL` set —
`Status`/`CredentialRef` are untouched fields on `UpdateFields` (the type
has no such fields at all, so this is enforced by the type system; the
test just asserts the fake repo's `Update` receives the exact input the
usecase was given, unmodified) — guards against `UpdateAccount` becoming a
second path to mutate lifecycle state.

### `delete_account_test.go` (new)

Asserts `Delete` is called with `(tenantID, accountID)` from context/input;
empty `accountID` → `AIPROVIDER_NO_ACCOUNT_ID` without calling the repo.

### `write_credential_test.go` (new)

```go
package usecase_test

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/usecase"
)

func TestWriteCredential_OwnerIDDerivation_MatchesCreateAccount(t *testing.T) {
	repo := newFakeProviderAccountRepository()
	repo.getReturns = domain.ProviderAccount{ID: "acct-1", TenantID: "t1", UserID: "user-1"}
	broker := newFakeCredentialBrokerClient() // existing fakes_test.go helper
	uc := usecase.NewWriteCredential(repo, broker)

	ctx := tenant.WithTenantID(context.Background(), "t1")
	account, err := uc.Execute(ctx, usecase.WriteCredentialForAccountInput{
		AccountID: "acct-1", EncryptedBlob: []byte("ciphertext"), IV: []byte("iv"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if broker.lastWriteOwnerID != "user-1" {
		t.Errorf("owner_id = %q, want user-1 (same 3-branch fallback as CreateAccount)", broker.lastWriteOwnerID)
	}
	if account.Status != domain.AccountStatusPending {
		t.Errorf("Status = %q, want pending (mirrors §9's not-active-until-push-confirmed rule)", account.Status)
	}
}
```

### `test_connection_test.go` (new)

```go
package usecase_test

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/usecase"
)

// fakeInfraFleetClient is scoped to Relay only.
type fakeInfraFleetClient struct {
	relayFunc func(ctx context.Context, devServerID, method string, params map[string]any) (map[string]any, error)
}

func (f *fakeInfraFleetClient) Relay(ctx context.Context, devServerID, method string, params map[string]any) (map[string]any, error) {
	return f.relayFunc(ctx, devServerID, method, params)
}

func TestTestConnection_NeverConstructsPlaintext(t *testing.T) {
	repo := newFakeProviderAccountRepository()
	repo.getReturns = domain.ProviderAccount{ID: "acct-1", TenantID: "t1", DevServerID: "ds-1", CredentialRef: "cred-ref-1"}

	var gotParams map[string]any
	infra := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, devServerID, method string, params map[string]any) (map[string]any, error) {
			gotParams = params
			return map[string]any{"success": true, "message": "ok"}, nil
		},
	}
	uc := usecase.NewTestConnection(repo, infra)

	ctx := tenant.WithTenantID(context.Background(), "t1")
	result, err := uc.Execute(ctx, usecase.TestConnectionInput{AccountID: "acct-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected Success=true")
	}
	// gotParams must only ever carry credentialRef (an opaque pointer) and
	// providerType — never a raw key. This is the same "no field capable of
	// holding a secret" discipline credential-broker-service.md §9 documents.
	for k := range gotParams {
		if k != "credentialRef" && k != "providerType" {
			t.Errorf("unexpected param key %q — TestConnection must never construct plaintext", k)
		}
	}
}

func TestTestConnection_NoDevServer_FailsFast(t *testing.T) {
	repo := newFakeProviderAccountRepository()
	repo.getReturns = domain.ProviderAccount{ID: "acct-1", TenantID: "t1"} // DevServerID empty
	infra := &fakeInfraFleetClient{relayFunc: func(context.Context, string, string, map[string]any) (map[string]any, error) {
		t.Fatal("Relay must not be called when DevServerID is empty")
		return nil, nil
	}}
	uc := usecase.NewTestConnection(repo, infra)

	ctx := tenant.WithTenantID(context.Background(), "t1")
	if _, err := uc.Execute(ctx, usecase.TestConnectionInput{AccountID: "acct-1"}); err == nil {
		t.Fatal("expected AIPROVIDER_NO_DEV_SERVER error")
	}
}
```

### `infra-fleet-service/internal/usecase/resolve_connection_test.go` — extend

Add a case asserting `ResolveConnectionInput{DevServerID: "ds-1"}` returns
the same `ResolveConnectionOutput` a by-`ConnectionID` resolve of the same
live connection would (create one connection row bound to `ds-1` via the
fake resolver, resolve both ways, compare). Add a second case for
`WorktreeID` the same way. Follow this file's existing table-driven
pattern (read the current file before extending it).

### `services/api-gateway/internal/adapter/wscompat/channels_ai_provider_test.go` (new)

One test per channel following `channels_accounts_test.go`'s pattern
(TASK-022): a `fakeAIProviderClient` embedding
`aiproviderv1.AiProviderServiceClient` with one func field per RPC this
file's handlers call (`CreateAccount`, `ListAccounts`, `UpdateAccount`,
`DeleteAccount`, `WriteCredential`, `TestConnection`), asserting each
channel decodes its args correctly and calls the right RPC with the right
fields. Additionally:

- `aiProvider.writeCredential`: assert `encryptedBlob`/`iv` base64-decode
  correctly and an invalid base64 string returns a decode error without
  calling `WriteCredential`.
- `aiProvider.list`/`update`/`delete`: assert `AttachIdentity` ran (reuse
  `outgoingTenantUser` from `channels_test.go`).

---

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/ai-provider-service/... -v
go test ./services/infra-fleet-service/internal/usecase/... -run ResolveConnection -v
go test ./services/api-gateway/internal/adapter/wscompat/... -run AIProvider -v
```

Expected: all new tests pass; no regressions in existing suites for these
3 packages.
